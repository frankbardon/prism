package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// expectedSchemaCount is the count of schema/v1/*.schema.json files
// committed in this repo. Bump in lockstep when schemas are added or
// removed.
const expectedSchemaCount = 15

// TestPrismSchemaList exercises `prism schema list`; asserts the
// known names are present (spec, mark, encoding, ...).
func TestPrismSchemaList(t *testing.T) {
	out, code := runCLI(t, "prism", "schema", "list")
	if code != 0 {
		t.Fatalf("schema list exit = %d\n%s", code, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != expectedSchemaCount {
		t.Fatalf("schema list returned %d lines; want %d\n%s", len(lines), expectedSchemaCount, out)
	}
	for _, want := range []string{"spec", "mark", "encoding", "data", "transform", "selection"} {
		if !strings.Contains(out, want+"\n") && !strings.HasSuffix(out, want) {
			t.Errorf("schema list missing %q\n%s", want, out)
		}
	}
}

// TestPrismSchemaShow asserts `prism schema show spec` returns valid
// JSON whose $id matches the URN convention.
func TestPrismSchemaShow(t *testing.T) {
	out, code := runCLI(t, "prism", "schema", "show", "spec")
	if code != 0 {
		t.Fatalf("schema show exit = %d\n%s", code, out)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("show output not JSON: %v", err)
	}
	if doc["$id"] != "urn:prism:schema:v1:spec" {
		t.Errorf("$id = %v; want urn:prism:schema:v1:spec", doc["$id"])
	}
}

// TestPrismSchemaShowMissing rejects unknown names.
func TestPrismSchemaShowMissing(t *testing.T) {
	_, code := runCLI(t, "prism", "schema", "show", "not_a_schema")
	if code == 0 {
		t.Fatalf("schema show not_a_schema: exit = 0; want non-zero")
	}
}

// TestPrismSchemaExport writes every schema (+ _meta) into a temp
// directory and asserts the file count matches.
func TestPrismSchemaExport(t *testing.T) {
	dir := t.TempDir()
	_, code := runCLI(t, "prism", "schema", "export", dir)
	if code != 0 {
		t.Fatalf("schema export exit = %d", code)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.schema.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != expectedSchemaCount {
		t.Fatalf("export produced %d schema files; want %d", len(matches), expectedSchemaCount)
	}
	metaDir := filepath.Join(dir, "_meta")
	if _, err := os.Stat(metaDir); err != nil {
		t.Errorf("export missing _meta/: %v", err)
	}
}

// TestPrismSchemaBundle writes the D087 bundle and asserts the
// `files` object holds every schema keyed by `<name>.schema.json`.
func TestPrismSchemaBundle(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bundle.json")
	_, code := runCLI(t, "prism", "schema", "bundle", out)
	if code != 0 {
		t.Fatalf("schema bundle exit = %d", code)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	var doc struct {
		Schema  string                     `json:"$schema"`
		ID      string                     `json:"$id"`
		Version string                     `json:"version"`
		Files   map[string]json.RawMessage `json:"files"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse bundle: %v\n%s", err, string(body))
	}
	if doc.ID != "urn:prism:schema:v1:bundle" {
		t.Errorf("$id = %q; want urn:prism:schema:v1:bundle", doc.ID)
	}
	if doc.Version != "v1" {
		t.Errorf("version = %q; want v1", doc.Version)
	}
	if len(doc.Files) != expectedSchemaCount {
		t.Errorf("bundle files = %d; want %d", len(doc.Files), expectedSchemaCount)
	}
	for _, key := range []string{"spec.schema.json", "mark.schema.json", "encoding.schema.json"} {
		if _, ok := doc.Files[key]; !ok {
			t.Errorf("bundle missing %q", key)
		}
	}
}
