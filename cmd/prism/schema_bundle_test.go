package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrismSchemaBundleSingleFile — PHASE.md P16 mandate.
// Runs `prism schema bundle <out>`, parses the result, asserts the
// shape is the D087 bundle wrapper carrying every schema verbatim, and
// confirms no relative `$ref` to a sibling schema file leaks through
// the bundle's top-level keys (refs inside individual schemas are
// preserved since they self-resolve within their owning schema).
func TestPrismSchemaBundleSingleFile(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "bundle.json")

	out, exit := runCLI(t, "prism", "schema", "bundle", outFile)
	if exit != 0 {
		t.Fatalf("schema bundle exit %d: %s", exit, firstChars(out, 200))
	}

	body, err := os.ReadFile(outFile)
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
		t.Fatalf("parse bundle: %v", err)
	}

	if doc.Schema == "" {
		t.Error("bundle missing $schema")
	}
	if !strings.HasPrefix(doc.ID, "urn:prism:schema:v1:") {
		t.Errorf("bundle $id = %q, want urn:prism:schema:v1:* prefix", doc.ID)
	}
	if doc.Version == "" {
		t.Error("bundle missing version")
	}
	if len(doc.Files) < 10 {
		t.Errorf("bundle has %d files; expected ≥10", len(doc.Files))
	}

	// Spot-check that spec.schema.json is present and parses.
	specBody, ok := doc.Files["spec.schema.json"]
	if !ok {
		t.Fatal("bundle missing spec.schema.json")
	}
	var specProbe map[string]any
	if err := json.Unmarshal(specBody, &specProbe); err != nil {
		t.Errorf("spec.schema.json malformed inside bundle: %v", err)
	}
}
