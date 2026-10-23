package obj

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/TerraKitFramework/TerraKit-CLI/internal/mesh"
)

func validate(m mesh.Data) error {
	n := len(m.Positions)
	if n == 0 || len(m.Indices) == 0 || len(m.Indices)%3 != 0 {
		return fmt.Errorf("mesh requires vertices and triangle indices")
	}
	if len(m.Normals) != 0 && len(m.Normals) != n {
		return fmt.Errorf("normals must match vertex count")
	}
	if len(m.UVs) != 0 && len(m.UVs) != n {
		return fmt.Errorf("UVs must match vertex count")
	}
	for _, p := range m.Positions {
		for _, v := range p {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("non-finite mesh vertex")
			}
		}
	}
	for _, v := range m.Origin {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("non-finite mesh origin")
		}
	}
	for _, index := range m.Indices {
		if uint64(index) >= uint64(n) {
			return fmt.Errorf("mesh index %d is out of bounds", index)
		}
	}
	for _, p := range m.Normals {
		for _, v := range p {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("non-finite normal")
			}
		}
	}
	for _, p := range m.UVs {
		for _, v := range p {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("non-finite UV")
			}
		}
	}
	return nil
}

// Write emits a standard 1-based Wavefront OBJ, retaining native triangle winding.
func Write(w io.Writer, m mesh.Data) error {
	if err := validate(m); err != nil {
		return err
	}
	b := bufio.NewWriter(w)
	fmt.Fprintln(b, "# TerraKit mesh export")
	fmt.Fprintln(b, "o terrain")
	for _, p := range m.Positions {
		fmt.Fprintf(b, "v %.9g %.9g %.9g\n", p[0]+m.Origin[0], p[1]+m.Origin[1], p[2]+m.Origin[2])
	}
	for _, p := range m.UVs {
		fmt.Fprintf(b, "vt %.9g %.9g\n", p[0], p[1])
	}
	for _, p := range m.Normals {
		fmt.Fprintf(b, "vn %.9g %.9g %.9g\n", p[0], p[1], p[2])
	}
	for i := 0; i < len(m.Indices); i += 3 {
		fmt.Fprint(b, "f")
		for j := 0; j < 3; j++ {
			idx := m.Indices[i+j] + 1
			switch {
			case len(m.UVs) > 0 && len(m.Normals) > 0:
				fmt.Fprintf(b, " %d/%d/%d", idx, idx, idx)
			case len(m.UVs) > 0:
				fmt.Fprintf(b, " %d/%d", idx, idx)
			case len(m.Normals) > 0:
				fmt.Fprintf(b, " %d//%d", idx, idx)
			default:
				fmt.Fprintf(b, " %d", idx)
			}
		}
		fmt.Fprintln(b)
	}
	return b.Flush()
}

// Preflight checks the destination before potentially expensive native generation.
func Preflight(path string, overwrite bool) error {
	if !strings.EqualFold(filepath.Ext(path), ".obj") {
		return fmt.Errorf("OBJ output path must end in .obj")
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("output already exists: %s (use --overwrite)", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// Save writes to a temporary file and renames into place only after a complete export.
func Save(path string, m mesh.Data, overwrite bool) error {
	if err := Preflight(path, overwrite); err != nil {
		return err
	}
	if err := validate(m); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".terrakit-*.obj.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := Write(f, m); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// Linking the completed temp file is atomic and refuses to replace an
	// existing destination, including when another writer races us.
	if !overwrite {
		if err := os.Link(f.Name(), path); err != nil {
			return fmt.Errorf("publish OBJ (destination must not exist): %w", err)
		}
		return nil
	}
	// Most OSes support rename-replacement. Some Windows configurations refuse
	// it; in that case removal is allowed only with explicit --overwrite.
	if err := os.Rename(f.Name(), path); err == nil {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(f.Name(), path)
}
