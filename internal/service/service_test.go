package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TerraKitFramework/TerraKit-CLI/internal/catalog"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/config"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/mesh"
)

type fakeEngine struct{ called bool }

func (f *fakeEngine) Catalog() ([]catalog.Stage, error) {
	return []catalog.Stage{{TypeID: "source", Outputs: []catalog.Port{{ID: "mesh", Kind: 4}}}}, nil
}
func (f *fakeEngine) Generate(_ config.Document, _ []catalog.Stage) (mesh.Data, error) {
	f.called = true
	return mesh.Data{Positions: []mesh.Vec3{{}, {1, 0, 0}, {0, 1, 0}}, Indices: []uint32{0, 1, 2}}, nil
}
func TestExecute(t *testing.T) {
	e := &fakeEngine{}
	stages, _ := e.Catalog()
	doc := config.New()
	doc.Nodes = []config.Node{{ID: "geometry", Type: "source"}}
	doc.Output = config.Output{Node: "geometry", Port: "mesh"}
	path := filepath.Join(t.TempDir(), "out.obj")
	result, err := Execute(e, doc, stages, path, false)
	if err != nil {
		t.Fatal(err)
	}
	if !e.called || result.Triangles != 1 {
		t.Fatal("engine not executed correctly")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
