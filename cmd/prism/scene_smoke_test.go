package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// TestPrismCLISceneEmitsValidJSON ensures `prism scene <fixture>`
// emits a JSON document that decodes into a SceneDoc and pins
// Version == CurrentVersion. The output is what the JS port
// (prism.mjs) and the cross-impl harness (D076) consume.
func TestPrismCLISceneEmitsValidJSON(t *testing.T) {
	chdirRepoRoot(t)
	fixture := repoFile(t, "examples", "specs", "bar_basic.json")
	out, exit := runCLI(t, "prism", "scene", fixture)
	if exit != 0 {
		t.Fatalf("scene exited %d: %s", exit, firstChars(out, 400))
	}
	body := stripLeadingWarnings(out)
	if !strings.HasPrefix(strings.TrimSpace(body), "{") {
		t.Fatalf("scene output does not look like JSON: %s", firstChars(body, 200))
	}
	var doc scene.SceneDoc
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("decode scene JSON: %v\n%s", err, firstChars(body, 400))
	}
	if doc.Version != scene.CurrentVersion {
		t.Errorf("Version = %q, want %q", doc.Version, scene.CurrentVersion)
	}
	if len(doc.Grid.Cells) == 0 {
		t.Errorf("Grid.Cells empty; want >= 1")
	}
	if doc.Theme == nil || doc.Theme.CSS == "" {
		t.Errorf("Theme.CSS empty; cross-impl parity requires theme CSS to ship in JSON")
	}
}

// TestPrismCLISceneOutFlag verifies the --out flag writes to a file
// instead of stdout, and the file decodes as a valid SceneDoc.
func TestPrismCLISceneOutFlag(t *testing.T) {
	chdirRepoRoot(t)
	dir := t.TempDir()
	outPath := filepath.Join(dir, "bar.scene.json")
	fixture := repoFile(t, "examples", "specs", "bar_basic.json")
	_, exit := runCLI(t, "prism", "scene", "--out", outPath, fixture)
	if exit != 0 {
		t.Fatalf("scene --out exited %d", exit)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s: %v", outPath, err)
	}
	var doc scene.SceneDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("decode %s: %v", outPath, err)
	}
	if doc.Version != scene.CurrentVersion {
		t.Errorf("Version = %q, want %q", doc.Version, scene.CurrentVersion)
	}
}

// TestPrismCLISceneCompact verifies the --compact flag emits
// single-line JSON (no newlines beyond the trailing one).
func TestPrismCLISceneCompact(t *testing.T) {
	chdirRepoRoot(t)
	fixture := repoFile(t, "examples", "specs", "bar_basic.json")
	out, exit := runCLI(t, "prism", "scene", "--compact", fixture)
	if exit != 0 {
		t.Fatalf("scene --compact exited %d", exit)
	}
	body := stripLeadingWarnings(out)
	body = strings.TrimRight(body, "\n")
	if strings.Contains(body, "\n") {
		t.Errorf("--compact output contains newlines: %s", firstChars(body, 200))
	}
}
