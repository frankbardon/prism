package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli/v3"
)

func suppressOsExiter(t *testing.T) {
	t.Helper()
	prev := cli.OsExiter
	cli.OsExiter = func(int) {}
	t.Cleanup(func() { cli.OsExiter = prev })
}

// TestPrismInitProducesUsableEditorConfig — PHASE.md P16 mandate.
// Runs `prism init` in a fresh tempdir, asserts the directory tree is
// populated, and parses the VSCode JSON + JetBrains XML configs for
// well-formedness.
func TestPrismInitProducesUsableEditorConfig(t *testing.T) {
	dir := t.TempDir()
	app := newApp()
	if err := app.Run(context.Background(), []string{"prism", "init", dir}); err != nil {
		t.Fatalf("init: %v", err)
	}

	prismDir := filepath.Join(dir, ".prism")
	for _, sub := range []string{"schemas", "examples", "editor", "README.md"} {
		if _, err := os.Stat(filepath.Join(prismDir, sub)); err != nil {
			t.Errorf("missing %s: %v", sub, err)
		}
	}

	// Schema spot-check: spec.schema.json present + non-empty.
	body, err := os.ReadFile(filepath.Join(prismDir, "schemas", "spec.schema.json"))
	if err != nil {
		t.Fatalf("read spec schema: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("spec.schema.json empty")
	}
	var schemaProbe map[string]any
	if err := json.Unmarshal(body, &schemaProbe); err != nil {
		t.Fatalf("spec.schema.json not valid JSON: %v", err)
	}

	// VSCode settings parses as JSON.
	body, err = os.ReadFile(filepath.Join(prismDir, "editor", "vscode-settings.json"))
	if err != nil {
		t.Fatalf("read vscode-settings.json: %v", err)
	}
	var vscodeProbe map[string]any
	if err := json.Unmarshal(body, &vscodeProbe); err != nil {
		t.Errorf("vscode-settings.json malformed: %v", err)
	}

	// JetBrains XML parses.
	body, err = os.ReadFile(filepath.Join(prismDir, "editor", "jetbrains.xml"))
	if err != nil {
		t.Fatalf("read jetbrains.xml: %v", err)
	}
	dec := xml.NewDecoder(nil)
	_ = dec
	var jbProbe struct {
		XMLName xml.Name `xml:"application"`
	}
	if err := xml.Unmarshal(body, &jbProbe); err != nil {
		t.Errorf("jetbrains.xml malformed: %v", err)
	}
}

// TestPrismInitRefusesExistingDir verifies the safety guard.
func TestPrismInitRefusesExistingDir(t *testing.T) {
	suppressOsExiter(t)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".prism"), 0o755); err != nil {
		t.Fatalf("pre-create: %v", err)
	}
	app := newApp()
	err := app.Run(context.Background(), []string{"prism", "init", dir})
	if err == nil {
		t.Fatal("expected init to refuse existing .prism/ without --force")
	}
}

// TestPrismInitForceOverwrites verifies --force.
func TestPrismInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".prism", "stale"), 0o755); err != nil {
		t.Fatalf("pre-create: %v", err)
	}
	app := newApp()
	if err := app.Run(context.Background(), []string{"prism", "init", "--force", dir}); err != nil {
		t.Fatalf("init --force: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".prism", "stale")); err == nil {
		t.Fatal("--force did not clear stale entry")
	}
}

// TestPrismInitBareSkipsExamplesAndEditor verifies --bare.
func TestPrismInitBareSkipsExamplesAndEditor(t *testing.T) {
	dir := t.TempDir()
	app := newApp()
	if err := app.Run(context.Background(), []string{"prism", "init", "--bare", dir}); err != nil {
		t.Fatalf("init --bare: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".prism", "examples")); err == nil {
		t.Error("--bare wrote examples dir")
	}
	if _, err := os.Stat(filepath.Join(dir, ".prism", "editor")); err == nil {
		t.Error("--bare wrote editor dir")
	}
	if _, err := os.Stat(filepath.Join(dir, ".prism", "schemas")); err != nil {
		t.Errorf("--bare missing schemas: %v", err)
	}
}
