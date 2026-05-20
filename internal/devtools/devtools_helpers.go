// Package devtools holds Go-side tooling tests that don't fit any
// runtime package. devtools_helpers.go carries shared test helpers
// — currently just the repoRoot walker used by the cross-impl and
// selection harness tests to locate the repository root.
package devtools

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRoot walks upward from the test working directory until it
// finds a `go.mod` file. The test files live in
// internal/devtools/, so two levels up is the repo root in
// practice; the walker handles arbitrary depth for harnesses that
// run from elsewhere.
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
