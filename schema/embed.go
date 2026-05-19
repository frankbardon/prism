// Package schema embeds the canonical Prism JSON Schema bundle and exposes a
// loader for downstream packages.
//
// Schemas live in schema/v1/. Each file has a URN $id of the form
// urn:prism:schema:v1:<name>. Cross-file $refs are relative paths
// (e.g. "data.schema.json#/$defs/data") that resolve within the embedded
// bundle. Validators register each file under both its URN and its filename
// so relative refs in source resolve identically to URN refs.
package schema

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed v1/*.json v1/_meta/*.md
var FS embed.FS

// Version is the schema bundle version that //go:embed walks.
const Version = "v1"

// URNPrefix is the canonical URN prefix for all v1 schemas.
const URNPrefix = "urn:prism:schema:v1:"

// V1Schemas returns the raw JSON bytes of every embedded schema file,
// keyed by the file's base name without the .schema.json suffix
// (e.g. "spec", "data"). _meta files are excluded — they are documentation,
// not schema resources.
func V1Schemas() (map[string][]byte, error) {
	out := make(map[string][]byte)
	err := fs.WalkDir(FS, Version, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.Contains(p, "/_meta/") {
			return nil
		}
		if !strings.HasSuffix(p, ".schema.json") {
			return nil
		}
		data, readErr := FS.ReadFile(p)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", p, readErr)
		}
		base := strings.TrimSuffix(path.Base(p), ".schema.json")
		out[base] = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("schema: no v1 schemas found in embedded FS")
	}
	return out, nil
}

// V1Filenames returns sorted file names (e.g. "spec.schema.json")
// for every embedded schema file.
func V1Filenames() ([]string, error) {
	names := []string{}
	err := fs.WalkDir(FS, Version, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || strings.Contains(p, "/_meta/") || !strings.HasSuffix(p, ".schema.json") {
			return nil
		}
		names = append(names, path.Base(p))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// URNFor returns the canonical URN for a schema base name (e.g. "spec").
func URNFor(name string) string {
	return URNPrefix + name
}
