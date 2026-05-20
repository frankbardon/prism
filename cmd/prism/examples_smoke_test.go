package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPrismExamplesList exercises `prism examples list` against the
// committed testdata/specs/ tree. Asserts the output contains the
// stable fixture names every later phase still ships with.
func TestPrismExamplesList(t *testing.T) {
	// The examples subcommand uses cwd as base; tests run from the
	// package dir cmd/prism, so use the repo root as cwd.
	root := repoRoot(t)
	t.Chdir(root)
	out, code := runCLI(t, "prism", "examples", "list")
	if code != 0 {
		t.Fatalf("examples list exit = %d\n%s", code, out)
	}
	for _, want := range []string{"bar_basic", "actual_vs_benchmark"} {
		if !strings.Contains(out, want) {
			t.Errorf("examples list missing %q\n%s", want, out)
		}
	}
}

// TestPrismExamplesShow exercises `prism examples show bar_basic`
// against the committed testdata; asserts the output is valid JSON
// carrying the expected $schema.
func TestPrismExamplesShow(t *testing.T) {
	root := repoRoot(t)
	t.Chdir(root)
	out, code := runCLI(t, "prism", "examples", "show", "bar_basic")
	if code != 0 {
		t.Fatalf("examples show exit = %d\n%s", code, out)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("show output not JSON: %v\n%s", err, out)
	}
	if doc["$schema"] != "urn:prism:schema:v1:spec" {
		t.Errorf("$schema = %v; want urn:prism:schema:v1:spec", doc["$schema"])
	}
}

// TestPrismExamplesShowMissing rejects unknown fixture names.
func TestPrismExamplesShowMissing(t *testing.T) {
	root := repoRoot(t)
	t.Chdir(root)
	_, code := runCLI(t, "prism", "examples", "show", "no_such_fixture")
	if code == 0 {
		t.Fatalf("examples show no_such_fixture: exit = 0; want non-zero")
	}
}
