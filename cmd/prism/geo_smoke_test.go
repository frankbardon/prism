package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrismGeoshapePlot drives the full validate + plot pipeline for
// a geoshape fixture and asserts the SVG contains the geoshape mark
// class. Acts as the standalone smoke test for the geo pipeline so
// regressions in projection, manifest, or render layers surface here
// before the broader gallery sweep.
func TestPrismGeoshapePlot(t *testing.T) {
	spec := filepath.Join(repoFile(t, "testdata", "specs"), "geo_world.json")

	out, exit := runCLI(t, "prism", "validate", spec)
	if exit != 0 {
		t.Fatalf("validate exit %d: %s", exit, firstChars(out, 200))
	}

	out, exit = runCLI(t, "prism", "plot", spec)
	if exit != 0 {
		t.Fatalf("plot exit %d: %s", exit, firstChars(out, 200))
	}
	body := stripLeadingWarnings(out)
	if !strings.HasPrefix(body, "<svg ") {
		t.Fatalf("output not SVG: %s", firstChars(body, 200))
	}
	if !strings.Contains(body, `class="prism-mark-geoshape"`) {
		t.Fatalf("expected geoshape mark class in output:\n%s", firstChars(body, 400))
	}
	if !strings.Contains(body, `fill-rule="evenodd"`) {
		t.Fatalf("expected even-odd fill rule:\n%s", firstChars(body, 400))
	}
}

// TestPrismGeoshapeMissingFeature asserts the validator fires
// PRISM_SPEC_021 when a geoshape spec lacks the feature channel.
func TestPrismGeoshapeMissingFeature(t *testing.T) {
	tmp := t.TempDir()
	bad := filepath.Join(tmp, "bad.json")
	body := `{"$schema":"urn:prism:schema:v1:spec","mark":"geoshape","projection":{"type":"mercator"},"encoding":{}}`
	if err := os.WriteFile(bad, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out, exit := runCLI(t, "prism", "validate", bad)
	if exit == 0 {
		t.Fatalf("expected non-zero exit, got 0: %s", firstChars(out, 200))
	}
	if !strings.Contains(out, "PRISM_SPEC_021") {
		t.Fatalf("expected PRISM_SPEC_021 in output:\n%s", out)
	}
}
