package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/frankbardon/prism/geodata"
)

// TestPrismCLIStaticBundleCopiesAllFiles ensures `prism static-bundle
// <out>` extracts every committed file under static/vendor/prism/.
// Validates the embed.FS round-trip and the path-preservation
// promise (D071 — relative imports keep resolving after extraction).
func TestPrismCLIStaticBundleCopiesAllFiles(t *testing.T) {
	dir := t.TempDir()
	// Source tier geometry from the committed repo geodata/ directory; the
	// host build no longer embeds tiers, so --geodata-dir is the seam that
	// feeds the <outDir>/geodata/<tier>.geo.json copies.
	geoDir := repoFile(t, "geodata")
	_, exit := runCLI(t, "prism", "static-bundle", "--geodata-dir", geoDir, dir)
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
		// Geodata artifacts: the still-embedded manifest plus each tier
		// sourced from --geodata-dir. The WASM runtime fetches these from
		// <bundle>/geodata/, so the layout must stay stable.
		filepath.Join("geodata", "manifest.json"),
	}
	for _, tier := range geodata.AllTiers() {
		wantFiles = append(wantFiles, filepath.Join("geodata", string(tier)+".geo.json"))
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

// TestPrismCLIStaticBundleGeodataDirUnset asserts that running
// static-bundle with no geodata directory configured (neither
// --geodata-dir nor PRISM_GEODATA, and the ambient TestMain dir cleared)
// fails loudly with PRISM_GEODATA_DIR_UNSET instead of an opaque embed
// message. The host build no longer embeds tier geometry, so the loader
// has no fallback source.
func TestPrismCLIStaticBundleGeodataDirUnset(t *testing.T) {
	restoreHostBundleDir(t)
	geodata.SetHostBundleDir("")

	dir := t.TempDir()
	out, exit := runCLI(t, "prism", "static-bundle", dir)
	if exit == 0 {
		t.Fatalf("expected non-zero exit with no geodata dir, got 0: %s", firstChars(out, 300))
	}
	if !strings.Contains(out, "PRISM_GEODATA_DIR_UNSET") {
		t.Fatalf("expected PRISM_GEODATA_DIR_UNSET in output:\n%s", firstChars(out, 400))
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
	// `static-bundle --wasm` sources TinyGo's wasm_exec.js via
	// locateWasmExec (`tinygo env TINYGOROOT`), even when --wasm-binary
	// supplies the module directly — so the command needs tinygo on PATH.
	// Skip cleanly when absent (mirrors the sibling TinyGo smoke + the
	// wasm CI job that installs tinygo and runs this non-skipped).
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("tinygo not on PATH; skipping static-bundle gzip smoke (install via `brew tap tinygo-org/tools && brew install tinygo`)")
	}
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

// TestPrismCLIStaticBundleWasmEmitsTinyGoLoader proves the real
// `--wasm` build path: static-bundle compiles cmd/prismwasm with TinyGo
// (via buildWasmInline) and pairs it with TinyGo's own wasm_exec.js
// (via locateWasmExec). TinyGo is Prism's sole wasm build and its loader
// is NOT byte-compatible with the Go toolchain's, so the emitted bundle
// must carry the TinyGo loader or the module will not instantiate.
//
// Two assertions: (1) the copied wasm_exec.js is TinyGo's (carries its
// provenance banner), and (2) the emitted binary + loader actually
// instantiate under Node and populate globalThis.prism.
//
// Gated by toolchain presence (mirrors internal/gates/wasm_tinygo_size_test.go):
// SKIPS cleanly when `tinygo` or `node` is absent so default CI without
// TinyGo stays green — the hard gate is the dedicated TinyGo job.
func TestPrismCLIStaticBundleWasmEmitsTinyGoLoader(t *testing.T) {
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("tinygo not on PATH; skipping TinyGo static-bundle smoke (install via `brew tap tinygo-org/tools && brew install tinygo`)")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH; skipping TinyGo static-bundle load smoke")
	}

	// buildWasmInline / the geodata copy resolve paths relative to the
	// process cwd (`./cmd/prismwasm`, `geodata/`), so drive the CLI from
	// the repo root. t.Chdir restores the cwd on cleanup.
	root := repoRoot(t)
	t.Chdir(root)

	dir := t.TempDir()
	// Point --wasm-binary at a guaranteed-absent path so the CLI takes its
	// real inline build branch (buildWasmInline → `tinygo build`) rather
	// than copying whatever standard-Go bin/prism.wasm happens to sit in
	// the tree. This mirrors a clean CI checkout (no bin/) and is what
	// makes the TinyGo-binary ↔ TinyGo-loader pairing meaningful.
	absent := filepath.Join(t.TempDir(), "does-not-exist.wasm")
	out, exit := runCLI(t, "prism", "static-bundle", "--wasm", "--wasm-binary", absent, "--geodata-dir", filepath.Join(root, "geodata"), dir)
	if exit != 0 {
		t.Fatalf("static-bundle --wasm exited %d:\n%s", exit, firstChars(out, 600))
	}

	// The emitted binary and loader must both be present.
	for _, name := range []string{"prism.wasm", "wasm_exec.js", "index.html"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("missing emitted %s: %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("emitted %s is zero bytes", name)
		}
	}

	// The loader must be TinyGo's — its wasm_exec.js carries a provenance
	// banner the Go toolchain's copy does not. This is the byte-family
	// check that replaces the retired Go-toolchain byte-identity pin.
	execSrc, err := os.ReadFile(filepath.Join(dir, "wasm_exec.js"))
	if err != nil {
		t.Fatalf("read emitted wasm_exec.js: %v", err)
	}
	if !strings.Contains(string(execSrc), "modified for use by the TinyGo compiler") {
		t.Errorf("emitted wasm_exec.js is not TinyGo's loader (missing TinyGo provenance banner)")
	}

	// End-to-end load smoke: instantiate the emitted binary with the
	// emitted loader under Node and confirm globalThis.prism resolves.
	loaderJS := `
import { readFile } from "node:fs/promises";
const dir = process.argv[2];
const execSrc = await readFile(dir + "/wasm_exec.js", "utf-8");
new Function("globalThis", execSrc)(globalThis);
const wasmBytes = await readFile(dir + "/prism.wasm");
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);
go.run(instance); // parks on a channel; fire-and-forget
const ready = await new Promise((res, rej) => {
  let n = 0;
  const tick = () => {
    if (globalThis.prism && typeof globalThis.prism.version === "function") return res(true);
    if (++n > 200) return rej(new Error("prism.wasm loaded but globalThis.prism.version absent"));
    setTimeout(tick, 0);
  };
  tick();
});
const v = globalThis.prism.version();
if (typeof v !== "string" || v.length === 0) {
  console.error("prism.version returned non-string/empty:", v);
  process.exit(1);
}
process.stdout.write("prism-loaded:" + v);
`
	scriptPath := filepath.Join(dir, "load_smoke.mjs")
	if err := os.WriteFile(scriptPath, []byte(loaderJS), 0o644); err != nil {
		t.Fatalf("write node loader script: %v", err)
	}

	cmd := exec.Command("node", scriptPath, dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("node load smoke failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "prism-loaded:") {
		t.Fatalf("node load smoke did not confirm module load; stdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
}
