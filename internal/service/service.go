package service

import (
	"fmt"
	"time"

	"github.com/TerraKitFramework/TerraKit-CLI/internal/catalog"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/config"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/mesh"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/obj"
	"github.com/TerraKitFramework/TerraKit-CLI/internal/validate"
)

type Engine interface {
	Catalog() ([]catalog.Stage, error)
	Generate(config.Document, []catalog.Stage) (mesh.Data, error)
}
type Result struct {
	Vertices  int
	Triangles int
	Duration  time.Duration
	Path      string
}

func Execute(engine Engine, doc config.Document, stages []catalog.Stage, out string, overwrite bool) (Result, error) {
	if err := validate.Check(doc, stages); err != nil {
		return Result{}, err
	}
	if err := obj.Preflight(out, overwrite); err != nil {
		return Result{}, err
	}
	started := time.Now()
	m, err := engine.Generate(doc, stages)
	if err != nil {
		return Result{}, fmt.Errorf("generation failed: %w", err)
	}
	if err := obj.Save(out, m, overwrite); err != nil {
		return Result{}, fmt.Errorf("OBJ export failed: %w", err)
	}
	return Result{Vertices: len(m.Positions), Triangles: len(m.Indices) / 3, Duration: time.Since(started), Path: out}, nil
}
