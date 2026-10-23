package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const SchemaVersion = 1

type Node struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Parameters map[string]string `json:"parameters,omitempty"`
}
type Link struct {
	FromNode string `json:"from_node"`
	FromPort string `json:"from_port"`
	ToNode   string `json:"to_node"`
	ToPort   string `json:"to_port"`
}
type Output struct {
	Node string `json:"node"`
	Port string `json:"port"`
}
type Region struct {
	Seed     uint64  `json:"seed"`
	X        int64   `json:"x"`
	Y        int64   `json:"y"`
	LOD      uint16  `json:"lod"`
	Width    uint32  `json:"width"`
	Height   uint32  `json:"height"`
	SpacingX float64 `json:"spacing_x"`
	SpacingY float64 `json:"spacing_y"`
}
type Document struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	Region  Region `json:"region"`
	Nodes   []Node `json:"nodes"`
	Links   []Link `json:"links"`
	Output  Output `json:"output"`
}

func New() Document {
	return Document{Version: SchemaVersion, Name: "Untitled terrain", Region: Region{Seed: 12345, Width: 64, Height: 64, SpacingX: 2, SpacingY: 2}, Nodes: []Node{}, Links: []Link{}}
}
func Load(path string) (Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}
	var doc Document
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("parse %s: %w", path, err)
	}
	var extra any
	if err := d.Decode(&extra); err == nil {
		return Document{}, fmt.Errorf("unexpected trailing JSON in %s", path)
	} else if !errors.Is(err, io.EOF) {
		return Document{}, fmt.Errorf("trailing JSON in %s: %w", path, err)
	}
	if doc.Version != SchemaVersion {
		return Document{}, fmt.Errorf("unsupported config version %d (expected %d)", doc.Version, SchemaVersion)
	}
	return doc, nil
}
func Save(path string, doc Document) error {
	if doc.Version != SchemaVersion {
		return fmt.Errorf("unsupported config version %d", doc.Version)
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".terrakit-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
