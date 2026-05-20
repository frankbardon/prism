// wasm_size_test.go enforces the gzipped-size ceiling on
// `bin/prism.wasm` defined in internal/limits.DefaultWasmMaxBytes
// (override via PRISM_WASM_MAX_BYTES). The gate runs only when the
// binary is present so non-WASM CI lanes stay green; the host build
// lane runs `make build-wasm` first to materialise the artifact.
package gates

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/frankbardon/prism/internal/limits"
)

func TestPrismWasmBinaryUnderSizeBudget(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	wasm := filepath.Join(root, "bin", "prism.wasm")
	info, err := os.Stat(wasm)
	if err != nil {
		t.Skipf("bin/prism.wasm not present (run `make build-wasm` first): %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("bin/prism.wasm is empty")
	}

	raw, err := os.ReadFile(wasm)
	if err != nil {
		t.Fatalf("read %s: %v", wasm, err)
	}

	// Gzip at max compression to mirror what a static-host content-
	// encoding pipeline would produce; the binary is shipped gzipped
	// over the wire so the wire size is what matters for the budget.
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		t.Fatalf("gzip writer: %v", err)
	}
	if _, err := io.Copy(gz, bytes.NewReader(raw)); err != nil {
		t.Fatalf("gzip copy: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	gzipped := buf.Len()
	limit := limits.MustWasmMaxBytes()

	t.Logf("bin/prism.wasm: raw=%d bytes (%.1f MiB), gzipped=%d bytes (%.1f MiB), limit=%d bytes (%.1f MiB)",
		len(raw), float64(len(raw))/(1024*1024),
		gzipped, float64(gzipped)/(1024*1024),
		limit, float64(limit)/(1024*1024))

	if gzipped > limit {
		t.Errorf("bin/prism.wasm gzipped size %d exceeds PRISM_WASM_MAX_BYTES=%d (PRISM_WASM_BUDGET_EXCEEDED)", gzipped, limit)
	}
	if gzipped > limits.SoftWarnWasmMaxBytes {
		t.Logf("WARN: bin/prism.wasm gzipped size %d crossed the soft-warn threshold %d", gzipped, limits.SoftWarnWasmMaxBytes)
	}
}

// repoRoot walks up from the test's cwd until it finds a directory
// containing go.mod. Each gates test keeps its own helper so the
// package stays a flat collection of independent invariants.
func repoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", os.ErrNotExist
		}
		cwd = parent
	}
}
