// Package gates holds repository-wide invariants enforced by `go test`.
//
// TestPrismNoBeraReferences walks the working tree from the repo root and
// fails if any source file (under 1 MiB, non-binary) contains the substring
// "bera" (case-insensitive). The planning tree is exempt because design docs
// reference upstream/cross-stack context.
package gates

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const maxScanBytes = 1 << 20 // 1 MiB — skip larger files to avoid massive binaries.

func TestPrismNoBeraReferences(t *testing.T) {
	pattern := regexp.MustCompile(`(?i)bera`)
	skip := map[string]bool{
		".git":         true,
		"bin":          true,
		"node_modules": true,
		".planning":    true,
	}
	skipExt := map[string]bool{
		".png":   true,
		".jpg":   true,
		".jpeg":  true,
		".gif":   true,
		".pdf":   true,
		".pulse": true,
		".woff":  true,
		".woff2": true,
		".ttf":   true,
		".ico":   true,
	}

	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Walk up to the repo root (directory containing go.mod).
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("go.mod not found in any parent directory")
		}
		root = parent
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip known binary file types up front.
		if skipExt[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		// Skip this test file itself — it intentionally contains the trigger word.
		if strings.HasSuffix(path, "no_bera_test.go") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		// Skip files larger than 1 MiB to avoid scanning massive blobs.
		if info.Size() > maxScanBytes {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if pattern.Match(data) {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("file %s contains forbidden reference", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
