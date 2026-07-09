// repo_root_test.go holds shared helpers for the gates test package.
package gates

import (
	"os"
	"path/filepath"
)

// repoRoot walks up from the test's cwd until it finds a directory
// containing go.mod. Shared by the gate invariants that need to resolve
// paths relative to the module root.
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
