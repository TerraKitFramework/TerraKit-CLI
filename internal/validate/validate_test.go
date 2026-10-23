package validate

import (
	"strings"
	"testing"

	"github.com/TerraKitFramework/TerraKit-CLI/internal/catalog"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/config"
)

func testSchemas() []catalog.Stage {
	return []catalog.Stage{
		{TypeID: "flat", Outputs: []catalog.Port{{ID: "height", Kind: 1}}, Parameters: []catalog.Parameter{{ID: "elevation", Kind: 5, Default: "0"}}},
		{TypeID: "noise", Inputs: []catalog.Port{{ID: "source", Kind: 1}}, Outputs: []catalog.Port{{ID: "height", Kind: 1}}},
		{TypeID: "mesh", Inputs: []catalog.Port{{ID: "height", Kind: 1}}, Outputs: []catalog.Port{{ID: "mesh", Kind: 4}}},
	}
}
func pipeline() config.Document {
	d := config.New()
	d.Nodes = []config.Node{{ID: "mesh", Type: "mesh"}, {ID: "flat", Type: "flat"}, {ID: "noise", Type: "noise"}}
	d.Links = []config.Link{{FromNode: "flat", FromPort: "height", ToNode: "noise", ToPort: "source"}, {FromNode: "noise", FromPort: "height", ToNode: "mesh", ToPort: "height"}}
	d.Output = config.Output{Node: "mesh", Port: "mesh"}
	return d
}
func TestTopologicalOrder(t *testing.T) {
	ordered, err := Order(pipeline(), testSchemas())
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"flat", "noise", "mesh"} {
		if ordered[i].ID != want {
			t.Errorf("step %d = %s, want %s", i, ordered[i].ID, want)
		}
	}
}
func TestInvalidGraph(t *testing.T) {
	tests := []struct {
		name, err string
		mutate    func(*config.Document)
	}{
		{"missing input", "missing required input", func(d *config.Document) { d.Links = d.Links[:1] }},
		{"unknown type", "unknown stage", func(d *config.Document) { d.Nodes[0].Type = "bogus" }},
		{"duplicated id", "duplicate node", func(d *config.Document) { d.Nodes[1].ID = "mesh" }},
		{"duplicate binding", "more than once", func(d *config.Document) { d.Links = append(d.Links, d.Links[0]) }},
		{"cycle", "cycle", func(d *config.Document) {
			d.Nodes = append(d.Nodes, config.Node{ID: "noise2", Type: "noise"})
			d.Links[0].FromNode = "noise2"
			d.Links = append(d.Links, config.Link{FromNode: "noise", FromPort: "height", ToNode: "noise2", ToPort: "source"})
		}},
		{"no mesh", "not a Mesh", func(d *config.Document) { d.Output = config.Output{Node: "flat", Port: "height"} }},
		{"region too large", "8 million", func(d *config.Document) { d.Region.Width = 8192; d.Region.Height = 8192 }},
		{"bad parameter", "unknown parameter", func(d *config.Document) { d.Nodes[1].Parameters = map[string]string{"xyz": "5"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := pipeline()
			tt.mutate(&d)
			err := Check(d, testSchemas())
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Fatalf("got %v; want %q", err, tt.err)
			}
		})
	}
}
func TestParamValidation(t *testing.T) {
	min, max := "0.1", "10"
	param := catalog.Parameter{Kind: 5, Min: &min, Max: &max}
	for _, tt := range []struct {
		v     string
		valid bool
	}{{"5", true}, {"0.1", true}, {"NaN", false}, {"11", false}, {"0", false}} {
		err := ValidateParameter(param, tt.v)
		if (err == nil) != tt.valid {
			t.Errorf("%q error=%v", tt.v, err)
		}
	}
}
