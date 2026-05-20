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
	// D3 modules were removed in P17 — the WASM pipeline replaces the
	// JS-side scale / axis / tick / format implementations they
	// previously supported. The bundle is now a minimal four-file
	// payload plus the README.
	wantFiles := []string{
		"prism.mjs",
		"prism-element.mjs",
		"prism-resolver.mjs",
		"prism-selection.mjs",
		"README.md",
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
