// wasm_exec_pinned_test.go asserts that `bin/wasm_exec.js` is
// byte-equal to the version shipped with the active Go toolchain.
// `make build-wasm` copies the file from $(go env GOROOT) and the
// gate guards against accidental hand-edits or toolchain skew.
//
// Skips when bin/wasm_exec.js is absent (non-WASM CI lanes stay
// green) or when GOROOT/lib/wasm/wasm_exec.js cannot be located
// (cross-compilation, unusual toolchain layouts).
package gates

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrismWasmExecJSPinned(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Walk up to repo root.
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			t.Fatal("go.mod not in any parent")
		}
		cwd = parent
	}
	binPath := filepath.Join(cwd, "bin", "wasm_exec.js")
	if _, err := os.Stat(binPath); err != nil {
		t.Skipf("bin/wasm_exec.js not present (run `make build-wasm` first): %v", err)
	}

	gorootOut, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Skipf("go env GOROOT failed: %v", err)
	}
	goroot := strings.TrimSpace(string(gorootOut))

	var canonical string
	for _, rel := range []string{
		filepath.Join("lib", "wasm", "wasm_exec.js"),
		filepath.Join("misc", "wasm", "wasm_exec.js"),
	} {
		candidate := filepath.Join(goroot, rel)
		if _, err := os.Stat(candidate); err == nil {
			canonical = candidate
			break
		}
	}
	if canonical == "" {
		t.Skipf("toolchain wasm_exec.js not located under %s/{lib,misc}/wasm/", goroot)
	}

	want, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("read %s: %v", canonical, err)
	}
	got, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("read %s: %v", binPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("bin/wasm_exec.js drifted from %s (got %d bytes, want %d). Run `make build-wasm` to refresh.",
			canonical, len(got), len(want))
	}
}
