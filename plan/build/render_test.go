package build_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
)

// updateGoldens controls whether failing golden tests rewrite the file
// instead of failing. Set via `go test -run TestPrismPlanDot -update`.
var updateGoldens = flag.Bool("update", false, "rewrite golden files instead of failing")

// goldenPath returns the absolute path of a golden file under
// testdata/golden/, mounted relative to the repo root.
func goldenPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "testdata", "golden", name)
}

// buildBarBasicDAG builds the DAG used by every renderer golden test.
func buildBarBasicDAG(t *testing.T) *plan.DAG {
	t.Helper()
	root := repoRoot(t)
	s := loadSpec(t, filepath.Join(root, "testdata", "specs", "bar_basic.json"))
	d, err := build.Build(s, build.Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return d
}

func TestPrismPlanDot(t *testing.T) {
	d := buildBarBasicDAG(t)
	var buf bytes.Buffer
	if err := plan.RenderDOT(d, &buf); err != nil {
		t.Fatalf("RenderDOT: %v", err)
	}
	got := buf.String()

	// Structural sanity checks first — these catch most malformed-DOT
	// regressions without depending on byte-exact output.
	if !strings.HasPrefix(got, "digraph prism_plan {") {
		t.Errorf("DOT missing digraph header: %q", got[:min(50, len(got))])
	}
	if strings.Count(got, "{") != 1 || strings.Count(got, "}") != 1 {
		t.Errorf("DOT braces unbalanced: %q", got)
	}
	for _, id := range d.Nodes() {
		if !strings.Contains(got, string(id)) {
			t.Errorf("DOT missing node id %q", id)
		}
	}
	// Edge count: every non-source node contributes one edge per input.
	wantEdges := 0
	for _, id := range d.Nodes() {
		n, _ := d.Node(id)
		wantEdges += len(n.Inputs())
	}
	if got, want := strings.Count(got, "->"), wantEdges; got != want {
		t.Errorf("DOT edge count = %d, want %d", got, want)
	}

	// Golden comparison.
	golden := goldenPath(t, "plan_bar_basic.dot")
	if *updateGoldens {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	expected, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (%s): %v. Run `go test -run TestPrismPlanDot -update` to create.", golden, err)
	}
	if string(expected) != got {
		t.Errorf("DOT does not match golden.\n--- golden ---\n%s\n--- got ---\n%s", expected, got)
	}
}

func TestPrismPlanText(t *testing.T) {
	d := buildBarBasicDAG(t)
	var buf bytes.Buffer
	if err := plan.RenderText(d, &buf); err != nil {
		t.Fatalf("RenderText: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "SinkNode") {
		t.Errorf("text missing SinkNode marker: %s", got)
	}
	// Tree style: at least one indented line.
	if !strings.Contains(got, "\n  ") {
		t.Errorf("text missing indentation: %s", got)
	}
}

func TestPrismPlanJSON(t *testing.T) {
	d := buildBarBasicDAG(t)
	var buf bytes.Buffer
	if err := plan.RenderJSON(d, &buf); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var probe map[string]any
	if err := json.Unmarshal(buf.Bytes(), &probe); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, buf.String())
	}
	if _, ok := probe["nodes"]; !ok {
		t.Errorf("JSON missing nodes key: %v", probe)
	}
	nodes, ok := probe["nodes"].([]any)
	if !ok {
		t.Fatalf("nodes is not an array: %T", probe["nodes"])
	}
	if len(nodes) != d.Size() {
		t.Errorf("nodes len=%d, want %d", len(nodes), d.Size())
	}
	if _, ok := probe["roots"]; !ok {
		t.Errorf("JSON missing roots key")
	}
	if _, ok := probe["sinks"]; !ok {
		t.Errorf("JSON missing sinks key")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
