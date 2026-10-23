"""Offline verification tests for the official release downloader."""
import hashlib
from io import BytesIO
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import fetch_native as native


class FetchNativeTests(unittest.TestCase):
    version = "0.0.3"
    target = "x86_64-pc-windows-msvc"
    library = "terrakit.dll"

    def fixture(self, abi="0.2.0", manifest_version=None, bad_library_digest=False):
        root = f"terrakit-v{self.version}-{self.target}/"
        binary = b"FAKE DLL FOR OFFLINE TESTING ONLY"
        digest = "0" * 64 if bad_library_digest else hashlib.sha256(binary).hexdigest()
        manifest = {"library_version": manifest_version or self.version, "abi_version": abi, "target": self.target,
                    "files": {"lib/terrakit.dll": digest}}
        stream = BytesIO()
        with zipfile.ZipFile(stream, "w") as z:
            z.writestr(root + "manifest.json", json.dumps(manifest))
            z.writestr(root + "lib/terrakit.dll", binary)
        content = stream.getvalue()
        filename = f"terrakit-v{self.version}-{self.target}.zip"
        sidecar = f"{hashlib.sha256(content).hexdigest()} {filename}\n".encode()
        return content, sidecar, binary

    def test_downloads_and_verifies_pinned_dll(self):
        archive, checksum, binary = self.fixture()
        with tempfile.TemporaryDirectory() as tmp, patch.object(native, "download", side_effect=[archive, checksum]) as download:
            path = native.fetch(self.version, self.target, tmp)
            self.assertEqual(Path(path).read_bytes(), binary)
            self.assertEqual(download.call_args_list[0].args[0],
                "https://github.com/TerraKitFramework/TerraKit/releases/download/terrakit-v0.0.3/terrakit-v0.0.3-x86_64-pc-windows-msvc.zip")

    def test_rejects_tampered_archive(self):
        archive, checksum, _ = self.fixture()
        with tempfile.TemporaryDirectory() as tmp, patch.object(native, "download", side_effect=[archive + b"x", checksum]):
            with self.assertRaisesRegex(native.FetchError, "archive SHA-256"):
                native.fetch(self.version, self.target, tmp)
            self.assertFalse((Path(tmp) / self.library).exists())

    def test_rejects_wrong_abi(self):
        archive, checksum, _ = self.fixture(abi="0.3.0")
        with tempfile.TemporaryDirectory() as tmp, patch.object(native, "download", side_effect=[archive, checksum]):
            with self.assertRaisesRegex(native.FetchError, "unsupported TerraKit C ABI"):
                native.fetch(self.version, self.target, tmp)

    def test_rejects_wrong_manifest_file_digest(self):
        archive, checksum, _ = self.fixture(bad_library_digest=True)
        with tempfile.TemporaryDirectory() as tmp, patch.object(native, "download", side_effect=[archive, checksum]):
            with self.assertRaisesRegex(native.FetchError, "library SHA-256"):
                native.fetch(self.version, self.target, tmp)

    def test_rejects_different_manifest_version(self):
        archive, checksum, _ = self.fixture(manifest_version="0.0.4")
        with tempfile.TemporaryDirectory() as tmp, patch.object(native, "download", side_effect=[archive, checksum]):
            with self.assertRaisesRegex(native.FetchError, "version or platform mismatch"):
                native.fetch(self.version, self.target, tmp)

    def test_rejects_latest_unpinned(self):
        with self.assertRaisesRegex(native.FetchError, "explicit semantic version"):
            native.fetch("latest", self.target, "dist/bin")

    def test_expected_checksum_exact_filename(self):
        with self.assertRaises(native.FetchError):
            native.expected_digest(("0"*64+" other.zip\n").encode(), "wanted.zip")

    def test_pinned_version_in_repo(self):
        self.assertEqual(native.pinned_version(), "0.0.3")


if __name__ == "__main__":
    unittest.main()
