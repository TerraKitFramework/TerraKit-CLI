package validate

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/TerraKitFramework/TerraKit-CLI/internal/catalog"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/config"
)

func Check(doc config.Document, stages []catalog.Stage) error {
	_, err := Order(doc, stages)
	return err
}
func Order(doc config.Document, stages []catalog.Stage) ([]config.Node, error) {
	if doc.Version != config.SchemaVersion {
		return nil, fmt.Errorf("unsupported document version %d", doc.Version)
	}
	if doc.Region.Width < 1 || doc.Region.Height < 1 || doc.Region.Width > 8192 || doc.Region.Height > 8192 || uint64(doc.Region.Width)*uint64(doc.Region.Height) > 8_000_000 {
		return nil, fmt.Errorf("region dimensions must be positive with at most 8 million cells")
	}
	if !finitePositive(doc.Region.SpacingX) || !finitePositive(doc.Region.SpacingY) {
		return nil, fmt.Errorf("spacing must be positive finite numbers")
	}
	if len(doc.Nodes) == 0 {
		return nil, fmt.Errorf("pipeline has no nodes")
	}
	schemas := catalog.ByID(stages)
	nodes := map[string]config.Node{}
	incoming := map[string]int{}
	edges := map[string][]string{}
	links := map[string]config.Link{}
	for _, n := range doc.Nodes {
		if n.ID == "" || strings.ContainsAny(n.ID, "/\n") {
			return nil, fmt.Errorf("invalid node ID %q", n.ID)
		}
		if _, exists := nodes[n.ID]; exists {
			return nil, fmt.Errorf("duplicate node ID %q", n.ID)
		}
		s, ok := schemas[n.Type]
		if !ok {
			return nil, fmt.Errorf("unknown stage type %q in node %q", n.Type, n.ID)
		}
		nodes[n.ID] = n
		definitions := map[string]catalog.Parameter{}
		for _, p := range s.Parameters {
			definitions[p.ID] = p
		}
		for id, value := range n.Parameters {
			p, ok := definitions[id]
			if !ok {
				return nil, fmt.Errorf("%s: unknown parameter %s", n.ID, id)
			}
			if err := ValidateParameter(p, value); err != nil {
				return nil, fmt.Errorf("%s.%s: %w", n.ID, id, err)
			}
		}
		for _, p := range s.Parameters {
			if _, present := n.Parameters[p.ID]; !present {
				if err := ValidateParameter(p, p.Default); err != nil {
					return nil, fmt.Errorf("%s.%s default: %w", n.ID, p.ID, err)
				}
			}
		}
	}
	for _, l := range doc.Links {
		source, ok := nodes[l.FromNode]
		if !ok {
			return nil, fmt.Errorf("unknown source node %s", l.FromNode)
		}
		target, ok := nodes[l.ToNode]
		if !ok {
			return nil, fmt.Errorf("unknown target node %s", l.ToNode)
		}
		src, ok := catalog.FindPort(schemas[source.Type].Outputs, l.FromPort)
		if !ok {
			return nil, fmt.Errorf("unknown output %s.%s", l.FromNode, l.FromPort)
		}
		dst, ok := catalog.FindPort(schemas[target.Type].Inputs, l.ToPort)
		if !ok {
			return nil, fmt.Errorf("unknown input %s.%s", l.ToNode, l.ToPort)
		}
		if src.Kind != dst.Kind {
			return nil, fmt.Errorf("resource type mismatch %s.%s (%s) to %s.%s (%s)", l.FromNode, l.FromPort, catalog.KindName(src.Kind), l.ToNode, l.ToPort, catalog.KindName(dst.Kind))
		}
		key := l.ToNode + "/" + l.ToPort
		if _, ok := links[key]; ok {
			return nil, fmt.Errorf("input %s is connected more than once", key)
		}
		links[key] = l
		incoming[l.ToNode]++
		edges[l.FromNode] = append(edges[l.FromNode], l.ToNode)
	}
	for _, n := range doc.Nodes {
		for _, p := range schemas[n.Type].Inputs {
			if !p.Optional {
				if _, ok := links[n.ID+"/"+p.ID]; !ok {
					return nil, fmt.Errorf("missing required input %s.%s", n.ID, p.ID)
				}
			}
		}
	}
	selected, ok := nodes[doc.Output.Node]
	if !ok {
		return nil, fmt.Errorf("mesh output must select an existing node")
	}
	output, ok := catalog.FindPort(schemas[selected.Type].Outputs, doc.Output.Port)
	if !ok || output.Kind != 4 {
		return nil, fmt.Errorf("selected output %s.%s is not a Mesh port", doc.Output.Node, doc.Output.Port)
	}
	// Stable topological order maintains document order between independent nodes.
	sorted := make([]config.Node, 0, len(doc.Nodes))
	used := map[string]bool{}
	for len(sorted) < len(doc.Nodes) {
		found := false
		for _, n := range doc.Nodes {
			if !used[n.ID] && incoming[n.ID] == 0 {
				used[n.ID] = true
				sorted = append(sorted, n)
				for _, dest := range edges[n.ID] {
					incoming[dest]--
				}
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("pipeline contains a dependency cycle")
		}
	}
	return sorted, nil
}
func finitePositive(f float64) bool { return f > 0 && !math.IsInf(f, 0) && !math.IsNaN(f) }
func ValidateParameter(p catalog.Parameter, text string) error {
	var num float64
	switch p.Kind {
	case 1:
		if _, err := strconv.ParseBool(text); err != nil {
			return fmt.Errorf("expected boolean: %w", err)
		}
		return nil
	case 2:
		if _, err := strconv.ParseInt(text, 10, 64); err != nil {
			return fmt.Errorf("expected signed integer: %w", err)
		}
	case 3:
		if _, err := strconv.ParseUint(text, 10, 64); err != nil {
			return fmt.Errorf("expected unsigned integer: %w", err)
		}
	case 4, 5:
		bits := 64
		if p.Kind == 4 {
			bits = 32
		}
		var err error
		num, err = strconv.ParseFloat(text, bits)
		if err != nil || math.IsNaN(num) || math.IsInf(num, 0) {
			return fmt.Errorf("expected finite floating-point value")
		}
	case 6, 7:
		parts := strings.Split(strings.Trim(text, " ()"), ",")
		count := 2
		if p.Kind == 7 {
			count = 3
		}
		if len(parts) != count {
			return fmt.Errorf("expected %d comma-separated values", count)
		}
		for _, part := range parts {
			f, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				return fmt.Errorf("invalid vector component")
			}
		}
		return nil
	case 8:
		return nil
	case 9:
		for _, o := range p.Options {
			if text == o {
				return nil
			}
		}
		return fmt.Errorf("expected one of %s", strings.Join(p.Options, ", "))
	default:
		return fmt.Errorf("unsupported parameter kind %d", p.Kind)
	}
	// Bounds apply to scalar values; compare exact int64 and uint64 where possible.
	if p.Kind == 2 {
		v, _ := strconv.ParseInt(text, 10, 64)
		if p.Min != nil {
			min, e := strconv.ParseInt(*p.Min, 10, 64)
			if e == nil && v < min {
				return fmt.Errorf("below minimum %d", min)
			}
		}
		if p.Max != nil {
			max, e := strconv.ParseInt(*p.Max, 10, 64)
			if e == nil && v > max {
				return fmt.Errorf("above maximum %d", max)
			}
		}
	}
	if p.Kind == 3 {
		v, _ := strconv.ParseUint(text, 10, 64)
		if p.Min != nil {
			min, e := strconv.ParseUint(*p.Min, 10, 64)
			if e == nil && v < min {
				return fmt.Errorf("below minimum %d", min)
			}
		}
		if p.Max != nil {
			max, e := strconv.ParseUint(*p.Max, 10, 64)
			if e == nil && v > max {
				return fmt.Errorf("above maximum %d", max)
			}
		}
	}
	if p.Kind == 4 || p.Kind == 5 {
		if p.Min != nil {
			min, e := strconv.ParseFloat(*p.Min, 64)
			if e == nil && num < min {
				return fmt.Errorf("below minimum %s", *p.Min)
			}
		}
		if p.Max != nil {
			max, e := strconv.ParseFloat(*p.Max, 64)
			if e == nil && num > max {
				return fmt.Errorf("above maximum %s", *p.Max)
			}
		}
	}
	return nil
}
