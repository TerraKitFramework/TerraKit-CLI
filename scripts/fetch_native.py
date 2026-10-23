#!/usr/bin/env python3
"""Fetch the pinned precompiled TerraKit C ABI library from GitHub Releases.

Only stdlib dependencies.  Never compiles TerraKit or trusts an unverified ZIP.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import sys
import tempfile
import urllib.error
import urllib.request
import zipfile
from io import BytesIO

REPO_ROOT = Path(__file__).resolve().parent.parent
RELEASES = "https://github.com/TerraKitFramework/TerraKit/releases/download"
VERSION_FORMAT = re.compile(r"\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?\Z")
SHA256_FORMAT = re.compile(r"[0-9a-fA-F]{64}\Z")
TARGET_LIBS = {
    "x86_64-pc-windows-msvc": "terrakit.dll",
    "x86_64-unknown-linux-gnu": "libterrakit.so",
    "x86_64-apple-darwin": "libterrakit.dylib",
    "aarch64-apple-darwin": "libterrakit.dylib",
}
MAX_ARCHIVE = 512 * 1024 * 1024


class FetchError(Exception):
    """The requested binary could not be securely installed."""


def host_target():
    system = platform.system().lower()
    machine = platform.machine().lower()
    if machine in ("amd64", "x86_64"):
        arch = "x86_64"
    elif machine in ("arm64", "aarch64"):
        arch = "aarch64"
    else:
        raise FetchError(f"unsupported host architecture: {machine}")
    target = {
        ("windows", "x86_64"): "x86_64-pc-windows-msvc",
        ("linux", "x86_64"): "x86_64-unknown-linux-gnu",
        ("darwin", "x86_64"): "x86_64-apple-darwin",
        ("darwin", "aarch64"): "aarch64-apple-darwin",
    }.get((system, arch))
    if not target:
        raise FetchError(f"no upstream release for {system}/{arch}")
    return target


def pinned_version():
    return (REPO_ROOT / "native-version.txt").read_text(encoding="utf-8").strip()


def download(url, max_bytes):
    try:
        request = urllib.request.Request(url, headers={"User-Agent": "TerraKit-CLI-native-fetch/1"})
        with urllib.request.urlopen(request, timeout=120) as response:
            data = response.read(max_bytes + 1)
    except (urllib.error.URLError, OSError) as exc:
        raise FetchError(f"download failed for {url}: {exc}") from exc
    if len(data) > max_bytes:
        raise FetchError(f"download exceeded {max_bytes} bytes: {url}")
    return data


def expected_digest(sidecar, archive_name):
    try:
        data = sidecar.decode("ascii").strip().splitlines()
    except UnicodeDecodeError as exc:
        raise FetchError("checksum file is not ASCII") from exc
    # Upstream writes: '<64-char sha256> <archive-name>.zip'
    if len(data) != 1:
        raise FetchError("unexpected checksum file format")
    fields = data[0].split()
    if len(fields) != 2 or not SHA256_FORMAT.fullmatch(fields[0]) or fields[1].removeprefix("*") != archive_name:
        raise FetchError("checksum file does not match the requested archive")
    return fields[0].lower()


def extract_verified_library(archive, version, target, library):
    root = f"terrakit-v{version}-{target}/"
    binary_name = f"{root}lib/{library}"
    manifest_name = f"{root}manifest.json"
    try:
        with zipfile.ZipFile(BytesIO(archive)) as zipped:
            entries = zipped.namelist()
            # Do not extract paths from the archive; read only exact known files.
            if len(entries) != len(set(entries)):
                raise FetchError("archive has duplicate entries")
            if any(not name.startswith(root) or ".." in name.split("/") or "\\" in name for name in entries):
                raise FetchError("archive contains unexpected or unsafe paths")
            if binary_name not in entries or manifest_name not in entries:
                raise FetchError(f"release archive missing {library} or manifest")
            manifest = json.loads(zipped.read(manifest_name))
            if not isinstance(manifest, dict):
                raise FetchError("invalid release manifest")
            if manifest.get("library_version") != version or manifest.get("target") != target:
                raise FetchError("release manifest version or platform mismatch")
            if not re.fullmatch(r"0\.2\.\d+", str(manifest.get("abi_version", ""))):
                raise FetchError(f"unsupported TerraKit C ABI: {manifest.get('abi_version')}")
            files = manifest.get("files")
            if not isinstance(files, dict):
                raise FetchError("release manifest has no file checksums")
            file_digest = files.get(f"lib/{library}")
            if not isinstance(file_digest, str) or not SHA256_FORMAT.fullmatch(file_digest):
                raise FetchError("release manifest has no valid library checksum")
            if zipped.getinfo(binary_name).file_size > MAX_ARCHIVE:
                raise FetchError("native binary is unexpectedly large")
            binary = zipped.read(binary_name)
    except (zipfile.BadZipFile, KeyError, ValueError, TypeError, RuntimeError) as exc:
        raise FetchError(f"invalid release ZIP or manifest: {exc}") from exc
    if hashlib.sha256(binary).hexdigest() != file_digest.lower():
        raise FetchError("library SHA-256 does not match upstream release manifest")
    return binary


def fetch(version, target, output_dir, base_url=RELEASES):
    if not VERSION_FORMAT.fullmatch(version):
        raise FetchError("expected explicit semantic version like 0.0.3 (not 'latest')")
    if target not in TARGET_LIBS:
        raise FetchError(f"unsupported target: {target}")
    name = f"terrakit-v{version}-{target}.zip"
    base = f"{base_url.rstrip('/')}/terrakit-v{version}/{name}"
    archive = download(base, MAX_ARCHIVE)
    checksum = download(base + ".sha256", 4096)
    if hashlib.sha256(archive).hexdigest() != expected_digest(checksum, name):
        raise FetchError("archive SHA-256 mismatch; refusing to install")
    binary = extract_verified_library(archive, version, target, TARGET_LIBS[target])

    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    destination = output_dir / TARGET_LIBS[target]
    # Do not leave an incomplete binary on failure; replace atomically.
    tmp_name = None
    try:
        with tempfile.NamedTemporaryFile(mode="wb", dir=output_dir, prefix=".terrakit-", delete=False) as tmp:
            tmp_name = tmp.name
            tmp.write(binary)
        os.replace(tmp_name, destination)
    finally:
        if tmp_name and os.path.exists(tmp_name):
            os.unlink(tmp_name)
    return destination


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", default=None, help="upstream release version (defaults to native-version.txt)")
    parser.add_argument("--target", choices=list(TARGET_LIBS), help="Rust target triple (defaults to this machine)")
    parser.add_argument("--output-dir", default="dist/bin", help="folder alongside the Go executable")
    args = parser.parse_args(argv)
    try:
        version = args.version or pinned_version()
        target = args.target or host_target()
        destination = fetch(version, target, args.output_dir)
    except (FetchError, OSError) as exc:
        parser.exit(1, f"error: {exc}\n")
    print(f"TerraKit {version} ({target}) verified and installed: {destination}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
