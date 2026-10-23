package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/TerraKitFramework/TerraKit-CLI/internal/catalog"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/config"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/service"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/validate"
)

type UI struct {
	In      *bufio.Scanner
	Out     io.Writer
	Engine  service.Engine
	Schemas []catalog.Stage
	Doc     config.Document
	Path    string
}

func New(r io.Reader, w io.Writer, e service.Engine, schemas []catalog.Stage, doc config.Document, path string) *UI {
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 4096), 1<<20)
	return &UI{In: scan, Out: w, Engine: e, Schemas: schemas, Doc: doc, Path: path}
}
func (ui *UI) ask(label, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(ui.Out, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(ui.Out, "%s: ", label)
	}
	if !ui.In.Scan() {
		if err := ui.In.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	text := strings.TrimSpace(ui.In.Text())
	if text == "" {
		return def, nil
	}
	return text, nil
}
func (ui *UI) choose(title string, choices []string) (int, error) {
	if len(choices) == 0 {
		return -1, fmt.Errorf("no choices available")
	}
	fmt.Fprintf(ui.Out, "\n%s\n", title)
	for i, item := range choices {
		fmt.Fprintf(ui.Out, "  %2d. %s\n", i+1, item)
	}
	for {
		answer, err := ui.ask("Number (or 0 to cancel)", "0")
		if err != nil {
			return -1, err
		}
		n, err := strconv.Atoi(answer)
		if err == nil && n >= 0 && n <= len(choices) {
			return n - 1, nil
		}
		fmt.Fprintln(ui.Out, "Enter a listed number.")
	}
}
func (ui *UI) Run() error {
	for {
		fmt.Fprintf(ui.Out, "\n=== TerraKit CLI | %s ===\n", ui.Doc.Name)
		fmt.Fprintf(ui.Out, "%d stages | %d connections | OBJ output: %s.%s\n", len(ui.Doc.Nodes), len(ui.Doc.Links), ui.Doc.Output.Node, ui.Doc.Output.Port)
		choice, err := ui.choose("Pipeline builder", []string{"List pipeline", "Browse native stages", "Add stage", "Set stage parameter", "Connect stages", "Select mesh output", "Edit region", "Remove stage", "Validate", "Save JSON", "Execute to OBJ", "Quit"})
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var actionErr error
		switch choice {
		case 0:
			ui.list()
		case 1:
			ui.stages()
		case 2:
			actionErr = ui.add()
		case 3:
			actionErr = ui.param()
		case 4:
			actionErr = ui.connect()
		case 5:
			actionErr = ui.output()
		case 6:
			actionErr = ui.region()
		case 7:
			actionErr = ui.remove()
		case 8:
			actionErr = ui.validate()
		case 9:
			actionErr = ui.save()
		case 10:
			actionErr = ui.execute()
		case 11:
			return nil
		default:
			continue
		}
		if errors.Is(actionErr, io.EOF) {
			return nil
		}
		if actionErr != nil {
			fmt.Fprintf(ui.Out, "Error: %v\n", actionErr)
		}
	}
}
func (ui *UI) list() {
	for _, n := range ui.Doc.Nodes {
		fmt.Fprintf(ui.Out, " - %s : %s\n", n.ID, n.Type)
		for k, v := range n.Parameters {
			fmt.Fprintf(ui.Out, "     %s = %s\n", k, v)
		}
	}
	for _, link := range ui.Doc.Links {
		fmt.Fprintf(ui.Out, "   %s.%s -> %s.%s\n", link.FromNode, link.FromPort, link.ToNode, link.ToPort)
	}
	fmt.Fprintf(ui.Out, "Region: %dx%d at (%.3g, %.3g), seed %d, region (%d,%d), lod %d\n", ui.Doc.Region.Width, ui.Doc.Region.Height, ui.Doc.Region.SpacingX, ui.Doc.Region.SpacingY, ui.Doc.Region.Seed, ui.Doc.Region.X, ui.Doc.Region.Y, ui.Doc.Region.LOD)
}
func (ui *UI) stages() {
	for _, s := range ui.Schemas {
		fmt.Fprintf(ui.Out, "\n%s [%s] (%s)\n", s.Name, s.TypeID, s.Category)
		for _, p := range s.Inputs {
			fmt.Fprintf(ui.Out, "  in  %-18s %s (optional=%t)\n", p.ID, catalog.KindName(p.Kind), p.Optional)
		}
		for _, p := range s.Outputs {
			fmt.Fprintf(ui.Out, "  out %-18s %s\n", p.ID, catalog.KindName(p.Kind))
		}
		for _, p := range s.Parameters {
			fmt.Fprintf(ui.Out, "  param %-18s %-8s default: %s\n", p.ID, catalog.ParamName(p.Kind), p.Default)
		}
	}
}
func (ui *UI) add() error {
	if len(ui.Schemas) == 0 {
		return fmt.Errorf("no native stages discovered")
	}
	sorted := append([]catalog.Stage(nil), ui.Schemas...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	choices := make([]string, len(sorted))
	for i, s := range sorted {
		choices[i] = fmt.Sprintf("%s [%s]", s.Name, s.TypeID)
	}
	idx, err := ui.choose("Add native stage", choices)
	if idx < 0 || err != nil {
		return err
	}
	id, err := ui.ask("Unique node ID", fmt.Sprintf("stage%d", len(ui.Doc.Nodes)+1))
	if err != nil {
		return err
	}
	if id == "" || strings.ContainsAny(id, "/\n") {
		return fmt.Errorf("node ID cannot be empty or contain '/' or newline")
	}
	for _, n := range ui.Doc.Nodes {
		if n.ID == id {
			return fmt.Errorf("node ID already exists")
		}
	}
	s := sorted[idx]
	ui.Doc.Nodes = append(ui.Doc.Nodes, config.Node{ID: id, Type: s.TypeID})
	fmt.Fprintf(ui.Out, "Added %s (%s)\n", id, s.Name)
	return nil
}
func (ui *UI) node() (int, error) {
	choices := make([]string, len(ui.Doc.Nodes))
	for i, n := range ui.Doc.Nodes {
		choices[i] = fmt.Sprintf("%s : %s", n.ID, n.Type)
	}
	return ui.choose("Select stage", choices)
}
func (ui *UI) param() error {
	idx, err := ui.node()
	if idx < 0 || err != nil {
		return err
	}
	n := &ui.Doc.Nodes[idx]
	s := catalog.ByID(ui.Schemas)[n.Type]
	choices := make([]string, len(s.Parameters))
	for i, p := range s.Parameters {
		val, ok := n.Parameters[p.ID]
		if !ok {
			val = p.Default
		}
		choices[i] = fmt.Sprintf("%s (%s) = %s", p.ID, catalog.ParamName(p.Kind), val)
	}
	pi, err := ui.choose("Edit parameter", choices)
	if pi < 0 || err != nil {
		return err
	}
	p := s.Parameters[pi]
	def, ok := n.Parameters[p.ID]
	if !ok {
		def = p.Default
	}
	if len(p.Options) > 0 {
		fmt.Fprintf(ui.Out, "Allowed values: %s\n", strings.Join(p.Options, ", "))
	}
	value, err := ui.ask(p.ID, def)
	if err != nil {
		return err
	}
	if err := validate.ValidateParameter(p, value); err != nil {
		return err
	}
	if n.Parameters == nil {
		n.Parameters = map[string]string{}
	}
	n.Parameters[p.ID] = value
	return nil
}
func (ui *UI) connect() error {
	schemas := catalog.ByID(ui.Schemas)
	type selectable struct {
		node, port string
		kind       int
	}
	outputs := []selectable{}
	labels := []string{}
	for _, n := range ui.Doc.Nodes {
		for _, p := range schemas[n.Type].Outputs {
			outputs = append(outputs, selectable{n.ID, p.ID, p.Kind})
			labels = append(labels, fmt.Sprintf("%s.%s [%s]", n.ID, p.ID, catalog.KindName(p.Kind)))
		}
	}
	idx, err := ui.choose("Source output", labels)
	if idx < 0 || err != nil {
		return err
	}
	src := outputs[idx]
	destinations := []selectable{}
	labels = nil
	for _, n := range ui.Doc.Nodes {
		if n.ID == src.node {
			continue
		}
		for _, p := range schemas[n.Type].Inputs {
			if p.Kind == src.kind {
				destinations = append(destinations, selectable{n.ID, p.ID, p.Kind})
				labels = append(labels, n.ID+"."+p.ID)
			}
		}
	}
	idx, err = ui.choose("Target input (replaces any previous connection)", labels)
	if idx < 0 || err != nil {
		return err
	}
	dest := destinations[idx]
	filtered := ui.Doc.Links[:0]
	for _, l := range ui.Doc.Links {
		if l.ToNode != dest.node || l.ToPort != dest.port {
			filtered = append(filtered, l)
		}
	}
	ui.Doc.Links = append(filtered, config.Link{FromNode: src.node, FromPort: src.port, ToNode: dest.node, ToPort: dest.port})
	return nil
}
func (ui *UI) output() error {
	schemas := catalog.ByID(ui.Schemas)
	type output struct{ node, port string }
	outputs := []output{}
	labels := []string{}
	for _, n := range ui.Doc.Nodes {
		for _, p := range schemas[n.Type].Outputs {
			if p.Kind == 4 {
				outputs = append(outputs, output{n.ID, p.ID})
				labels = append(labels, n.ID+"."+p.ID)
			}
		}
	}
	idx, err := ui.choose("Select Mesh output", labels)
	if idx < 0 || err != nil {
		return err
	}
	ui.Doc.Output = config.Output{Node: outputs[idx].node, Port: outputs[idx].port}
	return nil
}
func (ui *UI) region() error {
	r := ui.Doc.Region
	values := []struct {
		label string
		value string
	}{{"Seed", strconv.FormatUint(r.Seed, 10)}, {"Region X", strconv.FormatInt(r.X, 10)}, {"Region Y", strconv.FormatInt(r.Y, 10)}, {"LOD", strconv.FormatUint(uint64(r.LOD), 10)}, {"Cell width", strconv.FormatUint(uint64(r.Width), 10)}, {"Cell height", strconv.FormatUint(uint64(r.Height), 10)}, {"Spacing X", strconv.FormatFloat(r.SpacingX, 'g', -1, 64)}, {"Spacing Y", strconv.FormatFloat(r.SpacingY, 'g', -1, 64)}}
	nums := make([]string, len(values))
	for i, v := range values {
		s, err := ui.ask(v.label, v.value)
		if err != nil {
			return err
		}
		nums[i] = s
	}
	seed, err := strconv.ParseUint(nums[0], 10, 64)
	if err != nil {
		return err
	}
	x, err := strconv.ParseInt(nums[1], 10, 64)
	if err != nil {
		return err
	}
	y, err := strconv.ParseInt(nums[2], 10, 64)
	if err != nil {
		return err
	}
	lod, err := strconv.ParseUint(nums[3], 10, 16)
	if err != nil {
		return err
	}
	w, err := strconv.ParseUint(nums[4], 10, 32)
	if err != nil {
		return err
	}
	h, err := strconv.ParseUint(nums[5], 10, 32)
	if err != nil {
		return err
	}
	sx, err := strconv.ParseFloat(nums[6], 64)
	if err != nil {
		return err
	}
	sy, err := strconv.ParseFloat(nums[7], 64)
	if err != nil {
		return err
	}
	ui.Doc.Region = config.Region{Seed: seed, X: x, Y: y, LOD: uint16(lod), Width: uint32(w), Height: uint32(h), SpacingX: sx, SpacingY: sy}
	return nil
}
func (ui *UI) remove() error {
	idx, err := ui.node()
	if idx < 0 || err != nil {
		return err
	}
	id := ui.Doc.Nodes[idx].ID
	ui.Doc.Nodes = append(ui.Doc.Nodes[:idx], ui.Doc.Nodes[idx+1:]...)
	links := ui.Doc.Links[:0]
	for _, l := range ui.Doc.Links {
		if l.FromNode != id && l.ToNode != id {
			links = append(links, l)
		}
	}
	ui.Doc.Links = links
	if ui.Doc.Output.Node == id {
		ui.Doc.Output = config.Output{}
	}
	return nil
}
func (ui *UI) validate() error {
	if err := validate.Check(ui.Doc, ui.Schemas); err != nil {
		return err
	}
	fmt.Fprintln(ui.Out, "Pipeline valid (native assembly and execution are checked when run).")
	return nil
}
func (ui *UI) save() error {
	path, err := ui.ask("Save JSON configuration", ui.Path)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("no path selected")
	}
	if err := config.Save(path, ui.Doc); err != nil {
		return err
	}
	ui.Path = path
	fmt.Fprintf(ui.Out, "Saved %s\n", path)
	return nil
}
func (ui *UI) execute() error {
	path, err := ui.ask("OBJ export path", "terrain.obj")
	if err != nil {
		return err
	}
	overwrite := false
	if _, err := os.Stat(path); err == nil {
		answer, err := ui.ask("File exists. Overwrite? (yes/no)", "no")
		if err != nil {
			return err
		}
		overwrite = strings.EqualFold(answer, "yes")
		if !overwrite {
			return nil
		}
	}
	res, err := service.Execute(ui.Engine, ui.Doc, ui.Schemas, path, overwrite)
	if err != nil {
		return err
	}
	fmt.Fprintf(ui.Out, "Wrote %s — %d vertices, %d triangles, %s\n", res.Path, res.Vertices, res.Triangles, res.Duration.Round(1_000_000))
	return nil
}
