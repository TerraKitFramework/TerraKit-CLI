package terminal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TerraKitFramework/TerraKit-CLI/internal/catalog"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/config"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/mesh"
)

type fakeEngine struct{}

func (fakeEngine) Catalog() ([]catalog.Stage, error) { return nil, nil }
func (fakeEngine) Generate(config.Document, []catalog.Stage) (mesh.Data, error) {
	return mesh.Data{}, nil
}
func TestInteractiveBuilder(t *testing.T) {
	stages := []catalog.Stage{{TypeID: "flat", Name: "Flat", Outputs: []catalog.Port{{ID: "height", Kind: 1}}}, {TypeID: "mesh", Name: "Mesh", Inputs: []catalog.Port{{ID: "source", Kind: 1}}, Outputs: []catalog.Port{{ID: "mesh", Kind: 4}}}}
	file := filepath.Join(t.TempDir(), "edit.json")
	// Add Flat, add Mesh, connect outputs, select mesh, validate, save, quit.
	input := "3\n1\nbase\n3\n2\ngeo\n5\n1\n1\n6\n1\n9\n10\n\n12\n"
	var out bytes.Buffer
	ui := New(strings.NewReader(input), &out, fakeEngine{}, stages, config.New(), file)
	if err := ui.Run(); err != nil {
		t.Fatal(err)
	}
	if len(ui.Doc.Nodes) != 2 || len(ui.Doc.Links) != 1 || ui.Doc.Output.Node != "geo" {
		t.Fatalf("builder data mismatch: %+v\n%s", ui.Doc, out.String())
	}
	if !strings.Contains(out.String(), "Pipeline valid") {
		t.Fatalf("validation didn't succeed:\n%s", out.String())
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("save failed: %v\n%s", err, out.String())
	}
}
