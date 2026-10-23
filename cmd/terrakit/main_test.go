package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitAndHelpWithoutNative(t *testing.T) {
	var out bytes.Buffer
	path := filepath.Join(t.TempDir(), "terrain.json")
	if err := run([]string{"init", "--config", path}, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"init", "--config", path}, &out); err == nil {
		t.Fatal("overwrote existing config")
	}
	out.Reset()
	if err := run([]string{"--help"}, &out); err != nil || !strings.Contains(out.String(), "interactive") {
		t.Fatalf("help: %s, %v", out.String(), err)
	}
}
func TestUnknownCommand(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"bad"}, &out); err == nil {
		t.Fatal("accepted unknown command")
	}
}
