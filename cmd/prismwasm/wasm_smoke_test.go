// wasm_smoke_test.go is the end-to-end smoke test for the WASM entry
// point. Unlike main.go / geo.go (both `//go:build js && wasm`), this
// file carries no build constraint: it never imports syscall/js or
// references any symbol from the constrained sources directly. Instead
// it drives the compiled artifact from the outside — build the TinyGo
// wasm module, load it under Node with TinyGo's paired wasm_exec.js,
// and call the exported globalThis.prism.* functions via the shared
// probe-runner.mjs harness (internal/devtools/cross-impl-runner/) — the
// same mechanism internal/devtools/tinygo_render_test.go uses. Because
// cmd/prismwasm's only non-test sources are js/wasm-gated, this file is
// the package's entire buildable surface under a normal host GOOS/ARCH;
// `go test ./cmd/prismwasm/...` compiles and runs only this file.
//
// This is the E4-S1 regression closer: it proves the new
// html.New() bridge (`prism.renderHTML`) renders a `table`-mark spec
// end-to-end through the TinyGo-built module, not just that the Go
// source compiles.
//
// Needs `tinygo` and `node` on PATH; skips cleanly when either is
// absent so `make test` stays green without the toolchain installed
// (mirrors internal/gates/wasm_tinygo_size_test.go and the
// internal/devtools TinyGo parity harness).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrismWasmRenderHTMLTableSmoke drives validate → plan → execute →
// renderHTML over the docs/src/gallery/table/table_accounts.prism.json
// gallery fixture (a `mark: {type: "table"}` spec) through a
// freshly-built TinyGo wasm module, and asserts the returned HTML
// document carries a semantic <table> — proof the html.New() bridge
// (unreachable from JS before this story) actually renders the table
// mark's markup, not merely that the call returns without error.
func TestPrismWasmRenderHTMLTableSmoke(t *testing.T) {
	tinygo, err := exec.LookPath("tinygo")
	if err != nil {
		t.Skip("tinygo not on PATH; skipping wasm renderHTML smoke test (install via `brew tap tinygo-org/tools && brew install tinygo`)")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH; skipping wasm renderHTML smoke test")
	}

	root := repoRoot(t)
	specPath := filepath.Join(root, "docs", "src", "gallery", "table", "table_accounts.prism.json")
	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("table_accounts fixture missing: %v", err)
	}

	wasmPath, execPath := buildTinyGoWasm(t, root, tinygo)
	runner := filepath.Join(root, "internal", "devtools", "cross-impl-runner", "probe-runner.mjs")

	out := runNode(t, root, runner, "pipeline-html", wasmPath, execPath, specPath)
	html := strings.TrimSpace(string(out))

	if html == "" {
		t.Fatal("renderHTML pipeline produced empty output")
	}
	if strings.HasPrefix(html, `{"ok":false`) {
		t.Fatalf("renderHTML pipeline returned an error envelope: %s", html)
	}
	if !strings.Contains(html, "<!doctype html>") {
		t.Errorf("renderHTML output is not an HTML document; got prefix %q", head(html, 80))
	}
	if !strings.Contains(html, "<table") {
		t.Errorf("renderHTML output for a table-mark spec is missing a <table> tag:\n%s", head(html, 800))
	}
	if !strings.Contains(html, "Acme") || !strings.Contains(html, "Umbrella") {
		t.Errorf("renderHTML output is missing expected table row data:\n%s", head(html, 800))
	}
}

// repoRoot walks upward from the test working directory until it finds
// go.mod. cmd/prismwasm/ is one level below repo root in practice; the
// walker tolerates arbitrary depth.
func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("repoRoot: go.mod not found above %s", cwd)
	return ""
}

// buildTinyGoWasm builds ./cmd/prismwasm (this package) with TinyGo
// into a temp directory and copies over the TinyGo-paired
// wasm_exec.js, mirroring the Makefile's build-wasm-tinygo target
// (-stack-size=8MB, same as TINYGO_STACK_SIZE) and the equivalent
// helper in internal/devtools/tinygo_parity_test.go.
func buildTinyGoWasm(t *testing.T, root, tinygo string) (wasmPath, execPath string) {
	t.Helper()
	dir := t.TempDir()
	wasmPath = filepath.Join(dir, "prism.wasm")

	build := exec.Command(tinygo, "build", "-target=wasm", "-stack-size=8MB", "-o", wasmPath, "./cmd/prismwasm")
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("tinygo build ./cmd/prismwasm: %v\n%s", err, out)
	}

	tinyRootCmd := exec.Command(tinygo, "env", "TINYGOROOT")
	tinyRootCmd.Dir = root
	tinyRootOut, err := tinyRootCmd.Output()
	if err != nil {
		t.Fatalf("tinygo env TINYGOROOT: %v", err)
	}
	src := filepath.Join(strings.TrimSpace(string(tinyRootOut)), "targets", "wasm_exec.js")
	srcBytes, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	execPath = filepath.Join(dir, "wasm_exec.js")
	if err := os.WriteFile(execPath, srcBytes, 0o644); err != nil {
		t.Fatalf("write %s: %v", execPath, err)
	}
	return wasmPath, execPath
}

// runNode invokes the probe-runner.mjs harness and returns its stdout.
func runNode(t *testing.T, root, runner string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("node", append([]string{runner}, args...)...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("node probe-runner %v: %v\nstderr:\n%s", args[0], err, stderr.String())
	}
	return stdout.Bytes()
}

// head returns the first n bytes of s for diagnostics.
func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
