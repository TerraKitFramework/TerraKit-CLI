//go:build cgo

package native

import (
	"os"
	"testing"

	"github.com/TerraKitFramework/TerraKit-CLI/internal/config"
)

// Run with TERRAKIT_LIBRARY=/path/to/libterrakit.so go test ./internal/native -v.
func TestAgainstTerraKit(t *testing.T) {
	path := os.Getenv("TERRAKIT_LIBRARY")
	if path == "" {
		t.Skip("set TERRAKIT_LIBRARY to test the actual native library")
	}
	lib, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	schemas, err := lib.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]bool{}
	for _, s := range schemas {
		types[s.TypeID] = true
	}
	for _, want := range []string{"terrakit.height.flat", "terrakit.mesh.height_field"} {
		if !types[want] {
			t.Fatalf("missing stage %s", want)
		}
	}
	d := config.New()
	d.Region.Width = 4
	d.Region.Height = 4
	d.Nodes = []config.Node{{ID: "base", Type: "terrakit.height.flat"}, {ID: "mesh", Type: "terrakit.mesh.height_field"}}
	d.Links = []config.Link{{FromNode: "base", FromPort: "height", ToNode: "mesh", ToPort: "height"}}
	d.Output = config.Output{Node: "mesh", Port: "mesh"}
	m, err := lib.Generate(d, schemas)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Positions) != 25 || len(m.Indices) != 96 {
		t.Fatalf("unexpected native geometry %d vertices, %d indices", len(m.Positions), len(m.Indices))
	}
}
