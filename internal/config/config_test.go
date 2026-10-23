package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "terrain.json")
	want := New()
	want.Nodes = []Node{{ID: "one", Type: "a", Parameters: map[string]string{"foo": "1"}}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip mismatch: %#v != %#v", got, want)
	}
}
func TestRejectTrailingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	for _, suffix := range []string{" {}", "garbage"} {
		if err := os.WriteFile(path, []byte(`{"version":1}`+suffix), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("accepted trailing %q", suffix)
		}
	}
}
func TestRejectVersion(t *testing.T) {
	d := New()
	d.Version = 100
	if err := Save(filepath.Join(t.TempDir(), "bad.json"), d); err == nil {
		t.Fatal("accepted unknown version")
	}
}
