package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// TestPrismCLIStaticBundleWasmEmitsGzip ensures `--wasm` writes both
// the raw prism.wasm and a prism.wasm.gz companion that decompresses
// back to the source bytes, and that the standalone loader prefers the
// gzip path. The Go WASM target is ~69 MiB raw / ~12 MiB gzipped; the
// loader fetches the .gz so naive static hosts ship the small payload.
// A fake binary stands in for the real artifact to keep the test fast.
func TestPrismCLIStaticBundleWasmEmitsGzip(t *testing.T) {
	src := []byte("\x00asm\x01\x00\x00\x00 fake wasm payload for gzip round-trip test")
	srcPath := filepath.Join(t.TempDir(), "fake.wasm")
	if err := os.WriteFile(srcPath, src, 0o644); err != nil {
		t.Fatalf("write fake wasm: %v", err)
	}

	dir := t.TempDir()
	_, exit := runCLI(t, "prism", "static-bundle", "--wasm", "--wasm-binary", srcPath, dir)
	if exit != 0 {
		t.Fatalf("static-bundle --wasm exited %d", exit)
	}

	rawOut, err := os.ReadFile(filepath.Join(dir, "prism.wasm"))
	if err != nil {
		t.Fatalf("read emitted prism.wasm: %v", err)
	}
	if !bytes.Equal(rawOut, src) {
		t.Errorf("emitted prism.wasm differs from source")
	}

	gzOut, err := os.ReadFile(filepath.Join(dir, "prism.wasm.gz"))
	if err != nil {
		t.Fatalf("read emitted prism.wasm.gz: %v", err)
	}
	gr, err := gzip.NewReader(bytes.NewReader(gzOut))
	if err != nil {
		t.Fatalf("open gzip reader: %v", err)
	}
	decompressed, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("decompress prism.wasm.gz: %v", err)
	}
	if !bytes.Equal(decompressed, src) {
		t.Errorf("prism.wasm.gz does not decompress to the source bytes")
	}

	html, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(html), "prism.wasm.gz") {
		t.Errorf("standalone loader does not reference prism.wasm.gz")
	}
	if !strings.Contains(string(html), "DecompressionStream") {
		t.Errorf("standalone loader does not use DecompressionStream")
	}
}
