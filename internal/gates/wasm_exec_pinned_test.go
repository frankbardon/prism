// wasm_exec_pinned_test.go asserts that `bin/wasm_exec.js` is
// byte-equal to the version shipped with the active Go toolchain.
// `make build-wasm` copies the file from $(go env GOROOT) and the
// gate guards against accidental hand-edits or toolchain skew.
//
// Skips when bin/wasm_exec.js is absent (non-WASM CI lanes stay
// green) or when GOROOT/lib/wasm/wasm_exec.js cannot be located
// (cross-compilation, unusual toolchain layouts).
//
// `make build-wasm-tinygo` also writes bin/wasm_exec.js, but from the
// TinyGo toolchain, whose loader is NOT byte-compatible with Go's.
// When the loader in bin/ is TinyGo's (last build was TinyGo), this
// gate skips rather than false-fail — it only asserts the Go loader for
// the Go build path. Genuine drift (a loader matching neither the Go
// nor the TinyGo toolchain copy) still fails.
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
	if bytes.Equal(got, want) {
		return
	}

	// The loader in bin/ is not Go's. Before failing, check whether it
	// is TinyGo's — `make build-wasm-tinygo` writes its own loader into
	// bin/wasm_exec.js, which is intentionally not byte-compatible with
	// Go's. If bin/ holds the TinyGo loader, the last build was a TinyGo
	// build; this Go-pinned gate has nothing meaningful to assert, so it
	// skips instead of false-failing.
	if tinygoLoader, ok := tinygoWasmExecJS(); ok {
		if want, err := os.ReadFile(tinygoLoader); err == nil && bytes.Equal(got, want) {
			t.Skipf("bin/wasm_exec.js is the TinyGo loader (%s); Go-pinned gate skips. Run `make build-wasm` to assert the Go loader.", tinygoLoader)
		}
	}

	t.Errorf("bin/wasm_exec.js drifted from %s (got %d bytes, want %d). Run `make build-wasm` to refresh.",
		canonical, len(got), len(want))
}

// tinygoWasmExecJS returns the path to the active TinyGo toolchain's
// wasm_exec.js (under $(tinygo env TINYGOROOT)/targets/), and false when
// TinyGo is not on PATH or the loader cannot be located.
func tinygoWasmExecJS() (string, bool) {
	tinygo, err := exec.LookPath("tinygo")
	if err != nil {
		return "", false
	}
	out, err := exec.Command(tinygo, "env", "TINYGOROOT").Output()
	if err != nil {
		return "", false
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", false
	}
	candidate := filepath.Join(root, "targets", "wasm_exec.js")
	if _, err := os.Stat(candidate); err != nil {
		return "", false
	}
	return candidate, true
}
