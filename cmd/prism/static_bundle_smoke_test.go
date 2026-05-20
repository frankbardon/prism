package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPrismCLIStaticBundleCopiesAllFiles ensures `prism static-bundle
// <out>` extracts every committed file under static/vendor/prism/.
// Validates the embed.FS round-trip and the path-preservation
// promise (D071 — relative imports keep resolving after extraction).
func TestPrismCLIStaticBundleCopiesAllFiles(t *testing.T) {
	dir := t.TempDir()
	_, exit := runCLI(t, "prism", "static-bundle", dir)
	if exit != 0 {
		t.Fatalf("static-bundle exited %d", exit)
	}
	wantFiles := []string{
		"prism.mjs",
		"prism-element.mjs",
		"prism-resolver.mjs",
		"prism-selection.mjs",
		"README.md",
		"d3/README.md",
		"d3/VERSIONS.json",
		"d3/d3-array.mjs",
		"d3/d3-axis.mjs",
		"d3/d3-brush.mjs",
		"d3/d3-format.mjs",
		"d3/d3-scale.mjs",
		"d3/d3-shape.mjs",
		"d3/d3-time-format.mjs",
		"d3/d3-zoom.mjs",
	}
	for _, rel := range wantFiles {
		path := filepath.Join(dir, rel)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("missing extracted file %s: %v", rel, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("extracted file %s is zero bytes", rel)
		}
	}
}

// TestPrismCLIStaticBundleRejectsMissingArg ensures the subcommand
// errors (exit 2) when no output directory is provided.
func TestPrismCLIStaticBundleRejectsMissingArg(t *testing.T) {
	_, exit := runCLI(t, "prism", "static-bundle")
	if exit != 2 {
		t.Errorf("exit code = %d, want 2 (usage error)", exit)
	}
}
