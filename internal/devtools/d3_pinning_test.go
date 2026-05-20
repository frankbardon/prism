// Package devtools holds Go-side tooling tests that don't fit any
// runtime package. d3_pinning_test.go enforces D070 vendored-D3
// integrity by recomputing sha256 against the committed
// VERSIONS.json manifest.
package devtools

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type manifestEntry struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	SourceURL string `json:"source_url"`
	Sha256    string `json:"sha256"`
	ByteSize  int    `json:"byte_size"`
}

type manifest struct {
	Generator string          `json:"generator"`
	Note      string          `json:"note"`
	Modules   []manifestEntry `json:"modules"`
}

// repoRoot walks upward from the test working directory until it
// finds a `go.mod` file. The test files live in
// internal/devtools/, so two levels up is the repo root.
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

// TestPrismD3VendoredPinned reads the committed VERSIONS.json under
// static/vendor/prism/d3/ and asserts that every listed module's
// sha256 matches the digest computed from its bytes on disk.
// Empty-sha entries skip cleanly so a partial-sandbox vendoring
// (no network access) can land stub files + an empty-hash manifest
// without breaking the build per the T12.02 fallback path.
func TestPrismD3VendoredPinned(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "static", "vendor", "prism", "d3")
	manifestPath := filepath.Join(dir, "VERSIONS.json")

	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read VERSIONS.json: %v", err)
	}
	var m manifest
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal VERSIONS.json: %v", err)
	}
	if len(m.Modules) == 0 {
		t.Fatalf("VERSIONS.json: zero modules; expected 8 per D070")
	}

	for _, entry := range m.Modules {
		t.Run(entry.Name, func(t *testing.T) {
			path := filepath.Join(dir, entry.Name+".mjs")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if entry.Sha256 == "" {
				t.Skipf("manifest entry has empty sha256 (sandbox stub mode); re-run vendor-d3.sh + vendor-d3-manifest.go to populate")
			}
			if entry.ByteSize > 0 && entry.ByteSize != len(data) {
				t.Fatalf("byte_size mismatch: manifest=%d disk=%d", entry.ByteSize, len(data))
			}
			sum := sha256.Sum256(data)
			got := hex.EncodeToString(sum[:])
			if got != entry.Sha256 {
				t.Fatalf("sha256 mismatch for %s@%s:\n  manifest: %s\n  on disk:  %s\nIf you intentionally updated this file, run:\n  go run ./internal/devtools/vendor-d3-manifest.go > static/vendor/prism/d3/VERSIONS.json",
					entry.Name, entry.Version, entry.Sha256, got)
			}
		})
	}
}
