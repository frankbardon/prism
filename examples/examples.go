// Package examples is the single canonical home for Prism's curated spec
// corpus. The entire tree under specs/ is embedded into the binary so any
// consumer — the CLI examples leaf, the MCP examples tool, golden/plan/render
// tests — can read a spec without touching the filesystem.
//
// On-disk path mapping: a spec named "bar_basic" lives at
// examples/specs/bar_basic.json; specs in subdirectories keep their prefix
// relative to specs/ (e.g. "scales/log" → examples/specs/scales/log.json,
// "invalid/theta_on_bar" → examples/specs/invalid/theta_on_bar.json).
//
// The package imports ONLY embed + the standard library. It intentionally pulls
// in no facade, no MCP SDK, and no afero, so importing it is free of the
// six-stage pipeline's dependency weight and stays safe for every build target.
package examples

import (
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// FS is the embedded spec corpus, rooted at "specs". Use it directly when a
// consumer needs the raw tree (for example, to repoint a golden test at
// examples/specs/invalid/<name>.json) rather than the curated accessors below.
//
//go:embed specs
var FS embed.FS

// specsRoot is the embed root directory.
const specsRoot = "specs"

// invalidDir is the subdirectory holding specs that are designed to FAIL
// validation. They are embedded (so test consumers can read them) but excluded
// from List and Search, which surface only the valid, user-facing corpus.
const invalidDir = "invalid"

// Result is one entry returned by Search. It mirrors the shape produced by the
// MCP prism_examples_search tool: Name is the spec stem, Summary is the spec's
// title field (falling back to the stem when absent), and Spec is the raw JSON.
type Result struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Spec    string `json:"spec"`
}

// List returns the stems of every VALID spec (anything not under invalid/),
// sorted. A stem keeps its subdirectory prefix relative to specs/, e.g.
// "scales/log"; top-level specs are bare, e.g. "bar_basic".
func List() []string {
	names := collect(false)
	sort.Strings(names)
	return names
}

// Invalid returns the stems of every spec under invalid/, sorted. These are the
// designed-to-fail specs; consumers map a stem back to disk via
// examples/specs/<stem>.json.
func Invalid() []string {
	names := collect(true)
	sort.Strings(names)
	return names
}

// All returns the stems of every embedded spec (valid + invalid), sorted.
func All() []string {
	names := append(collect(false), collect(true)...)
	sort.Strings(names)
	return names
}

// Get returns the raw bytes of the spec with the given stem (e.g. "bar_basic"
// or "scales/log"). The boolean reports whether the spec exists. Both valid and
// invalid specs are reachable by stem.
func Get(name string) ([]byte, bool) {
	body, err := FS.ReadFile(path.Join(specsRoot, name+".json"))
	if err != nil {
		return nil, false
	}
	return body, true
}

// Search returns up to limit results whose stem or title contains query,
// case-insensitively. It skips the invalid/ subdirectory, sorts by stem, and —
// matching the MCP tool — falls back to the stem for Summary when the spec
// carries no title. A non-positive limit returns all matches.
func Search(query string, limit int) []Result {
	q := strings.ToLower(query)
	var out []Result
	for _, name := range collect(false) {
		body, ok := Get(name)
		if !ok {
			continue
		}
		title := extractTitle(body)
		hay := strings.ToLower(name + " " + title)
		if !strings.Contains(hay, q) {
			continue
		}
		summary := title
		if summary == "" {
			summary = name
		}
		out = append(out, Result{
			Name:    name,
			Summary: summary,
			Spec:    string(body),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// collect walks the embedded tree and returns the stems of every .json spec.
// When wantInvalid is true it returns only specs under invalid/; otherwise it
// returns only specs outside invalid/.
func collect(wantInvalid bool) []string {
	var names []string
	_ = fs.WalkDir(FS, specsRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".json") {
			return nil
		}
		stem := strings.TrimSuffix(strings.TrimPrefix(p, specsRoot+"/"), ".json")
		isInvalid := stem == invalidDir || strings.HasPrefix(stem, invalidDir+"/")
		if isInvalid != wantInvalid {
			return nil
		}
		names = append(names, stem)
		return nil
	})
	return names
}

// extractTitle picks the spec.title field without forcing a full decode (a spec
// may not pass strict decoding even when it is a valid example). It mirrors the
// MCP tool's extractTitle exactly: only a bare-string title is read; any other
// shape (or a decode failure) yields "".
func extractTitle(body []byte) string {
	var doc struct {
		Title string `json:"title"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return ""
	}
	return doc.Title
}
