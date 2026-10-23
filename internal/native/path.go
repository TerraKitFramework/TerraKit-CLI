package native

import (
	"os"
	"path/filepath"
)

// resolveLibraryPath respects explicit paths, then tries the executable's
// directory (as used by release ZIPs), and finally the OS loader search path.
func resolveLibraryPath(explicit, executable, goos string) string {
	if explicit != "" {
		return explicit
	}

	var name string
	switch goos {
	case "windows":
		name = "terrakit.dll"
	case "darwin":
		name = "libterrakit.dylib"
	default:
		name = "libterrakit.so"
	}
	if executable != "" {
		candidate := filepath.Join(filepath.Dir(executable), name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return name
}
