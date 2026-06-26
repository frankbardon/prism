package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/frankbardon/prism/geodata"
)

// TestPrismGeoshapePlot drives the full validate + plot pipeline for
// a geoshape fixture and asserts the SVG contains the geoshape mark
// class. Acts as the standalone smoke test for the geo pipeline so
// regressions in projection, manifest, or render layers surface here
// before the broader gallery sweep.
func TestPrismGeoshapePlot(t *testing.T) {
	spec := filepath.Join(repoFile(t, "examples", "specs"), "geo_world.json")

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

// TestPrismGeoshapePlotGeodataDir exercises the real --geodata-dir flag
// path: it clears any ambient host bundle directory (the package
// TestMain sets one) so the flag itself is what makes the geoshape mark
// resolve, then asserts plot renders the geoshape SVG.
func TestPrismGeoshapePlotGeodataDir(t *testing.T) {
	geoDir := repoFile(t, "geodata")
	specPath := filepath.Join(repoFile(t, "examples", "specs"), "geo_world.json")

	restoreHostBundleDir(t)
	geodata.SetHostBundleDir("")

	out, exit := runCLI(t, "prism", "plot", "--geodata-dir", geoDir, specPath)
	if exit != 0 {
		t.Fatalf("plot --geodata-dir exit %d: %s", exit, firstChars(out, 300))
	}
	body := stripLeadingWarnings(out)
	if !strings.HasPrefix(body, "<svg ") {
		t.Fatalf("output not SVG: %s", firstChars(body, 200))
	}
	if !strings.Contains(body, `class="prism-mark-geoshape"`) {
		t.Fatalf("expected geoshape mark class in output:\n%s", firstChars(body, 400))
	}
}

// TestPrismSceneGeodataDir verifies the same flag wires the host loader
// for the `scene` leaf: a geoshape spec compiles to Scene IR carrying a
// geoshape mark.
func TestPrismSceneGeodataDir(t *testing.T) {
	geoDir := repoFile(t, "geodata")
	specPath := filepath.Join(repoFile(t, "examples", "specs"), "geo_world.json")

	restoreHostBundleDir(t)
	geodata.SetHostBundleDir("")

	out, exit := runCLI(t, "prism", "scene", "--geodata-dir", geoDir, specPath)
	if exit != 0 {
		t.Fatalf("scene --geodata-dir exit %d: %s", exit, firstChars(out, 300))
	}
	if !strings.Contains(out, `"geoshape"`) {
		t.Fatalf("expected geoshape mark in scene JSON:\n%s", firstChars(out, 400))
	}
}

// TestPrismGeoshapeGeodataDirUnset asserts that plotting a geo mark with
// no directory configured (neither --geodata-dir nor PRISM_GEODATA, and
// the ambient TestMain dir cleared) surfaces PRISM_GEODATA_DIR_UNSET.
//
// Uses the admin1-50m tier and resets the process-wide DefaultStore so
// the load is cold: sibling tests (the testdata sweep, the gallery
// fixtures) decode tier geometry into the global cache, so clearing the
// directory alone is not enough — already-decoded geometry would still
// resolve. After the reset + cleared directory the host loader consults
// the (empty) directory and returns PRISM_GEODATA_DIR_UNSET.
func TestPrismGeoshapeGeodataDirUnset(t *testing.T) {
	specPath := filepath.Join(repoFile(t, "examples", "specs"), "geo_admin1.json")

	restoreHostBundleDir(t)
	geodata.SetHostBundleDir("")
	geodata.ResetDefaultStore()
	t.Cleanup(geodata.ResetDefaultStore)

	out, exit := runCLI(t, "prism", "plot", specPath)
	if exit == 0 {
		t.Fatalf("expected non-zero exit with no geodata dir, got 0: %s", firstChars(out, 300))
	}
	if !strings.Contains(out, "PRISM_GEODATA_DIR_UNSET") {
		t.Fatalf("expected PRISM_GEODATA_DIR_UNSET in output:\n%s", firstChars(out, 400))
	}

	// The scene leaf shares the same encode path; confirm parity.
	out, exit = runCLI(t, "prism", "scene", specPath)
	if exit == 0 {
		t.Fatalf("expected non-zero scene exit with no geodata dir, got 0: %s", firstChars(out, 300))
	}
	if !strings.Contains(out, "PRISM_GEODATA_DIR_UNSET") {
		t.Fatalf("expected PRISM_GEODATA_DIR_UNSET in scene output:\n%s", firstChars(out, 400))
	}
}

// restoreHostBundleDir snapshots the current host bundle directory and
// restores it on test cleanup, so a test that clears or repoints the
// global loader directory does not leak into sibling tests (the package
// TestMain sets the repo geodata dir for the gallery / geo fixtures).
func restoreHostBundleDir(t *testing.T) {
	t.Helper()
	prev := geodata.HostBundleDir()
	t.Cleanup(func() { geodata.SetHostBundleDir(prev) })
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
