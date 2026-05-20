//go:build ignore

// vendor-d3-manifest is a one-off generator that walks
// static/vendor/prism/d3/*.mjs and emits VERSIONS.json on stdout.
//
// Run after `vendor-d3.sh` to refresh the sha256 + byte-size manifest.
// The committed VERSIONS.json is the audit log; `TestPrismD3VendoredPinned`
// verifies the digests still match.
//
// Build-ignored so it doesn't ship with `go build ./...`. Invoke with:
//
//	go run ./internal/devtools/vendor-d3-manifest.go > static/vendor/prism/d3/VERSIONS.json
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Version table — keep in sync with internal/devtools/vendor-d3.sh.
// Bump versions there + here together when updating modules.
var modules = map[string]string{
	"d3-array":       "3.2.4",
	"d3-axis":        "3.0.0",
	"d3-brush":       "3.0.0",
	"d3-format":      "3.1.0",
	"d3-scale":       "4.0.2",
	"d3-shape":       "3.2.0",
	"d3-time-format": "4.1.0",
	"d3-zoom":        "3.0.0",
}

type entry struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	SourceURL string `json:"source_url"`
	Sha256    string `json:"sha256"`
	ByteSize  int    `json:"byte_size"`
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail("getwd: %v", err)
	}
	dir := filepath.Join(root, "static", "vendor", "prism", "d3")
	names := make([]string, 0, len(modules))
	for n := range modules {
		names = append(names, n)
	}
	sort.Strings(names)

	out := struct {
		Generator string  `json:"generator"`
		Note      string  `json:"note"`
		Modules   []entry `json:"modules"`
	}{
		Generator: "internal/devtools/vendor-d3-manifest.go",
		Note:      "Regenerate after running internal/devtools/vendor-d3.sh. TestPrismD3VendoredPinned verifies sha256 matches every test run.",
	}
	for _, name := range names {
		path := filepath.Join(dir, name+".mjs")
		data, err := os.ReadFile(path)
		if err != nil {
			fail("read %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		out.Modules = append(out.Modules, entry{
			Name:      name,
			Version:   modules[name],
			SourceURL: fmt.Sprintf("https://cdn.jsdelivr.net/npm/%s@%s/+esm", name, modules[name]),
			Sha256:    hex.EncodeToString(sum[:]),
			ByteSize:  len(data),
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fail("encode: %v", err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vendor-d3-manifest: "+format+"\n", args...)
	os.Exit(1)
}
