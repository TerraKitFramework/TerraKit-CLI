package obj

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TerraKitFramework/TerraKit-CLI/internal/mesh"
)

func triangle() mesh.Data {
	return mesh.Data{Origin: mesh.Vec3{10, 20, 30}, Positions: []mesh.Vec3{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}, UVs: []mesh.Vec2{{0, 0}, {1, 0}, {0, 1}}, Normals: []mesh.Vec3{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}}, Indices: []uint32{0, 1, 2}}
}
func TestOBJ(t *testing.T) {
	var b bytes.Buffer
	if err := Write(&b, triangle()); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"v 10 20 30", "v 11 20 30", "vt 1 0", "vn 0 0 1", "f 1/1/1 2/2/2 3/3/3"} {
		if !strings.Contains(b.String(), line) {
			t.Errorf("missing %q in OBJ", line)
		}
	}
}
func TestRejectInvalidMesh(t *testing.T) {
	m := triangle()
	m.Indices[1] = 99
	var b bytes.Buffer
	if err := Write(&b, m); err == nil {
		t.Fatal("accepted out-of-range vertex")
	}
	if b.Len() != 0 {
		t.Fatal("wrote partial OBJ")
	}
}
func TestNoOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "terrain.obj")
	if err := Save(path, triangle(), false); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, triangle(), false); err == nil {
		t.Fatal("clobbered output")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		t.Fatalf("OBJ not saved: %v", err)
	}
}
