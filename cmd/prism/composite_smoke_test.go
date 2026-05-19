package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdirRepoRoot chdirs to the prism repo root for the duration of the
// test so spec fixtures with relative paths (e.g. testdata/cohorts/*.pulse
// in actual_vs_benchmark.json) resolve consistently.
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found")
		}
		dir = parent
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
}

// TestPrismPlotCompositeDispatch verifies the CLI routes composite
// specs through BuildComposite + EncodeComposite. The layer fixture
// must produce an SVG with two data-layer-id attributes (one per
// layer); the concat fixture must produce one prism-scene block per
// cell.
func TestPrismPlotCompositeDispatch(t *testing.T) {
	cases := []struct {
		fixture string
		marker  string // substring whose count we check
		min     int
		minSize int
	}{
		{"layer_actual_vs_benchmark.json", `data-layer-id="layer-`, 2, 1500},
		{"concat_h.json", `<g class="prism-scene"`, 2, 1500},
		{"concat_v.json", `<g class="prism-scene"`, 2, 1500},
		// actual_vs_benchmark.json moves to a real two-layer composition
		// in T08.10; until then it remains the P07 JOIN workaround (one
		// data-layer-id only). The two-layer assertion lands in T08.12.
	}
	for _, c := range cases {
		c := c
		t.Run(c.fixture, func(t *testing.T) {
			chdirRepoRoot(t)
			fixturePath := repoFile(t, "testdata", "specs", c.fixture)
			out, exit := runCLI(t, "prism", "plot", fixturePath)
			if exit != 0 {
				t.Fatalf("plot %s exited %d: %s", c.fixture, exit, firstChars(out, 400))
			}
			body := stripLeadingWarnings(out)
			if len(body) < c.minSize {
				t.Errorf("SVG too small (%d bytes): %s", len(body), firstChars(body, 200))
			}
			if got := strings.Count(body, c.marker); got < c.min {
				t.Errorf("marker %q count=%d, want >=%d", c.marker, got, c.min)
			}
		})
	}
}

// TestPrismExecuteCompositeDispatch verifies `prism execute` prints
// per-child sections for a composite spec.
func TestPrismExecuteCompositeDispatch(t *testing.T) {
	fixturePath := repoFile(t, "testdata", "specs", "layer_actual_vs_benchmark.json")
	out, exit := runCLI(t, "prism", "execute", "--format", "table", fixturePath)
	if exit != 0 {
		t.Fatalf("execute exited %d: %s", exit, firstChars(out, 400))
	}
	if !strings.Contains(out, "# layer 0") {
		t.Errorf("missing `# layer 0` section header in execute output: %s", firstChars(out, 400))
	}
	if !strings.Contains(out, "# layer 1") {
		t.Errorf("missing `# layer 1` section header in execute output: %s", firstChars(out, 400))
	}
}
