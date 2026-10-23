package native

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLibraryPath(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "terrakit")
	for _, tc := range []struct{ platform, library string }{
		{"windows", "terrakit.dll"},
		{"linux", "libterrakit.so"},
		{"darwin", "libterrakit.dylib"},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			got := resolveLibraryPath("", exe, tc.platform)
			if got != tc.library {
				t.Fatalf("fallback = %q, want %q", got, tc.library)
			}
			bundled := filepath.Join(dir, tc.library)
			if err := os.WriteFile(bundled, []byte("dummy"), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := resolveLibraryPath("", exe, tc.platform); got != bundled {
				t.Fatalf("bundled = %q, want %q", got, bundled)
			}
			if got := resolveLibraryPath("/explicit/path", exe, tc.platform); got != "/explicit/path" {
				t.Fatalf("explicit = %q", got)
			}
		})
	}
}
