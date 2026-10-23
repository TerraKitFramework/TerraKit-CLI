package catalog

import "fmt"

// Stage schemas and port types are discovered from TerraKit, never hardcoded.
type Port struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     int    `json:"kind"`
	Optional bool   `json:"optional,omitempty"`
}
type Parameter struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Kind        int      `json:"kind"`
	Default     string   `json:"default"`
	Min         *string  `json:"min,omitempty"`
	Max         *string  `json:"max,omitempty"`
	Options     []string `json:"options,omitempty"`
}
type Stage struct {
	TypeID      string      `json:"type_id"`
	Version     uint32      `json:"version"`
	Name        string      `json:"name"`
	Category    string      `json:"category"`
	Description string      `json:"description"`
	Inputs      []Port      `json:"inputs"`
	Outputs     []Port      `json:"outputs"`
	Parameters  []Parameter `json:"parameters"`
}

func ByID(stages []Stage) map[string]Stage {
	out := make(map[string]Stage, len(stages))
	for _, s := range stages {
		out[s.TypeID] = s
	}
	return out
}
func FindPort(ports []Port, id string) (Port, bool) {
	for _, p := range ports {
		if p.ID == id {
			return p, true
		}
	}
	return Port{}, false
}
func KindName(kind int) string {
	switch kind {
	case 1:
		return "HeightField"
	case 2:
		return "DensityField"
	case 3:
		return "VoxelVolume"
	case 4:
		return "Mesh"
	case 5:
		return "ScatterPoints"
	default:
		return fmt.Sprintf("kind(%d)", kind)
	}
}
func ParamName(kind int) string {
	switch kind {
	case 1:
		return "bool"
	case 2:
		return "int64"
	case 3:
		return "uint64"
	case 4:
		return "float32"
	case 5:
		return "float64"
	case 6:
		return "vec2"
	case 7:
		return "vec3"
	case 8:
		return "string"
	case 9:
		return "enum"
	default:
		return fmt.Sprintf("param(%d)", kind)
	}
}
