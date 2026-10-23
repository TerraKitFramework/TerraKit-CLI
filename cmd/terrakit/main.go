package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/TerraKitFramework/TerraKit-CLI/internal/config"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/native"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/service"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/terminal"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/validate"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "terrakit:", err)
		os.Exit(1)
	}
}
func usage(w io.Writer) {
	fmt.Fprint(w, `TerraKit CLI — standalone native terrain pipeline consumer

Commands:
  terrakit init        --config terrain.json
  terrakit stages      [--native /path/to/libterrakit.so]
  terrakit interactive [--config terrain.json] [--native PATH]
  terrakit validate    --config terrain.json [--native PATH]
  terrakit run         --config terrain.json --out terrain.obj [--overwrite] [--native PATH]

Native library path defaults to TERRAKIT_LIBRARY, then a library beside the executable, then the system loader path.
The package consumes TerraKit's C ABI 0.2.x. Build with CGO_ENABLED=1.
`)
}
func run(args []string, w io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		usage(w)
		return nil
	}
	command := args[0]
	switch command {
	case "init", "stages", "interactive", "validate", "run":
	default:
		usage(w)
		return fmt.Errorf("unknown command %q", command)
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(w)
	path := flags.String("config", "terrain.json", "JSON pipeline config")
	binary := flags.String("native", os.Getenv("TERRAKIT_LIBRARY"), "path to TerraKit native shared library")
	out := flags.String("out", "", "output Wavefront OBJ file")
	overwrite := flags.Bool("overwrite", false, "replace an existing OBJ")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if command == "init" {
		if _, err := os.Stat(*path); err == nil {
			return fmt.Errorf("config already exists: %s", *path)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := config.Save(*path, config.New()); err != nil {
			return err
		}
		fmt.Fprintf(w, "Created %s\n", *path)
		return nil
	}
	lib, err := native.Open(*binary)
	if err != nil {
		return err
	}
	defer lib.Close()
	schemas, err := lib.Catalog()
	if err != nil {
		return err
	}
	if command == "stages" {
		fmt.Fprintf(w, "TerraKit native %s (%d stages)\n", lib.Version(), len(schemas))
		for _, s := range schemas {
			fmt.Fprintf(w, "\n%s  %s (%s)\n", s.TypeID, s.Name, s.Category)
			for _, p := range s.Inputs {
				fmt.Fprintf(w, "  input  %-18s kind=%d optional=%t\n", p.ID, p.Kind, p.Optional)
			}
			for _, p := range s.Outputs {
				fmt.Fprintf(w, "  output %-18s kind=%d\n", p.ID, p.Kind)
			}
			for _, p := range s.Parameters {
				fmt.Fprintf(w, "  param  %-18s default=%q\n", p.ID, p.Default)
			}
		}
		return nil
	}
	var doc config.Document
	if command == "interactive" {
		doc, err = config.Load(*path)
		if errors.Is(err, os.ErrNotExist) {
			doc = config.New()
		} else if err != nil {
			return err
		}
		fmt.Fprintf(w, "Connected to TerraKit %s\n", lib.Version())
		return terminal.New(os.Stdin, w, lib, schemas, doc, *path).Run()
	}
	doc, err = config.Load(*path)
	if err != nil {
		return err
	}
	if err := validate.Check(doc, schemas); err != nil {
		return fmt.Errorf("invalid pipeline: %w", err)
	}
	if command == "validate" {
		fmt.Fprintln(w, "Pipeline valid. Native execution not attempted.")
		return nil
	}
	if *out == "" {
		return fmt.Errorf("--out is required for run")
	}
	result, err := service.Execute(lib, doc, schemas, *out, *overwrite)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Wrote %s — %d vertices, %d triangles in %s\n", result.Path, result.Vertices, result.Triangles, result.Duration.Round(1_000_000))
	return nil
}
