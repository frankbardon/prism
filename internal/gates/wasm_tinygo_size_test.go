// wasm_tinygo_size_test.go enforces the gzipped- and raw-size ceilings
// on the TinyGo-built `bin/prism.wasm` module, defined in
// internal/limits.DefaultWasmTinygoMaxBytes /
// DefaultWasmTinygoRawMaxBytes (override via PRISM_WASM_TINYGO_MAX_BYTES
// / PRISM_WASM_TINYGO_RAW_MAX_BYTES).
//
// The gate is opt-in by toolchain presence: it SKIPS cleanly when
// `tinygo` is not on PATH (default CI lanes without TinyGo stay green)
// and RUNS when it is present. Rather than depend on whichever build
// last populated bin/, it produces a fresh TinyGo artifact into a temp
// directory so the measurement is deterministic and cannot be fooled by
// a stale standard-Go binary in bin/. The standard-Go size gate in
// wasm_size_test.go is unaffected and continues to guard bin/prism.wasm.
package gates

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/frankbardon/prism/internal/limits"
)

// tinygoStackSize mirrors the Makefile's TINYGO_STACK_SIZE. TinyGo's
// default per-goroutine stack overflows on the JSON-Schema shape
// validator's recursion; 8 MB clears it. Stack size is a memory
// reservation, not a binary-size lever — the emitted .wasm bytes are
// effectively identical regardless — so the gate's measurement matches
// what `make build-wasm-tinygo` ships.
const tinygoStackSize = "8MB"

func TestPrismWasmTinygoBinaryUnderSizeBudget(t *testing.T) {
	tinygo, err := exec.LookPath("tinygo")
	if err != nil {
		t.Skip("tinygo not on PATH; skipping TinyGo wasm size gate (install via `brew tap tinygo-org/tools && brew install tinygo`)")
	}

	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}

	// Build a fresh TinyGo artifact into a temp dir so the gate measures
	// the current source, independent of whatever sits in bin/.
	out := filepath.Join(t.TempDir(), "prism.tinygo.wasm")
	cmd := exec.Command(tinygo, "build", "-target=wasm", "-stack-size="+tinygoStackSize, "-o", out, "./cmd/prismwasm")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tinygo build failed: %v\n%s", err, combined)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read %s: %v", out, err)
	}
	if len(raw) == 0 {
		t.Fatalf("TinyGo prism.wasm is empty")
	}

	// Gzip at max compression to mirror the wire size a static-host
	// content-encoding pipeline serves.
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

	limit := limits.MustWasmTinygoMaxBytes()
	rawLimit := limits.MustWasmTinygoRawMaxBytes()

	t.Logf("TinyGo prism.wasm: raw=%d bytes (%.1f MiB), gzipped=%d bytes (%.1f MiB), gzip-limit=%d bytes (%.1f MiB), raw-limit=%d bytes (%.1f MiB)",
		len(raw), float64(len(raw))/(1024*1024),
		gzipped, float64(gzipped)/(1024*1024),
		limit, float64(limit)/(1024*1024),
		rawLimit, float64(rawLimit)/(1024*1024))

	if gzipped > limit {
		t.Errorf("TinyGo prism.wasm gzipped size %d exceeds PRISM_WASM_TINYGO_MAX_BYTES=%d (PRISM_WASM_BUDGET_EXCEEDED)", gzipped, limit)
	}
	if gzipped > limits.SoftWarnWasmTinygoMaxBytes {
		t.Logf("WARN: TinyGo prism.wasm gzipped size %d crossed the soft-warn threshold %d", gzipped, limits.SoftWarnWasmTinygoMaxBytes)
	}

	if len(raw) > rawLimit {
		t.Errorf("TinyGo prism.wasm raw size %d exceeds PRISM_WASM_TINYGO_RAW_MAX_BYTES=%d (PRISM_WASM_BUDGET_EXCEEDED)", len(raw), rawLimit)
	}
	if len(raw) > limits.SoftWarnWasmTinygoRawMaxBytes {
		t.Logf("WARN: TinyGo prism.wasm raw size %d crossed the raw soft-warn threshold %d", len(raw), limits.SoftWarnWasmTinygoRawMaxBytes)
	}
}
