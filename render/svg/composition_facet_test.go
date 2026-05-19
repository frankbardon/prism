package svg_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/render/svg"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// renderCompositeP09 mirrors renderCompositeFromRoot but stays in the
// composition_facet_test.go file so failures isolate cleanly from
// the P08 gates.
func renderCompositeP09(t *testing.T, fixture string) []byte {
	t.Helper()
	root := repoRoot(t)
	path := filepath.Join(root, "testdata", "specs", fixture)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	per := make([]map[plan.NodeID]*table.Table, len(c.Children))
	for i, child := range c.Children {
		res, err := plan.Execute(context.Background(), child.DAG, plan.ExecOpts{})
		if err != nil {
			t.Fatalf("Execute child %d: %v", i, err)
		}
		per[i] = res.Tables
	}
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	out, err := svg.New().Render(doc, render.RenderOpts{Format: "svg", Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return out
}

// TestPrismFacetGrid (PHASE.md gate). 3x3 facet must render exactly
// 9 prism-scene blocks and match the committed golden.
func TestPrismFacetGrid(t *testing.T) {
	out := renderCompositeP09(t, "facet_by_region.json")
	got := strings.Count(string(out), `<g class="prism-scene"`)
	if got != 9 {
		t.Errorf("prism-scene count = %d, want 9", got)
	}
	goldenPath := filepath.Join(repoRoot(t), "testdata", "svgs", "facet_by_region.svg")
	if os.Getenv("UPDATE_GOLDENS") == "1" {
		if err := os.WriteFile(goldenPath, out, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated golden %s (%d bytes)", goldenPath, len(out))
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(out, want) {
		t.Errorf("golden drift on facet_by_region.svg (got %d bytes, want %d)", len(out), len(want))
	}
}

// TestPrismRepeatSubstitution (PHASE.md gate). repeat_metrics
// produces 2 cells; the substituted field names appear in the per-
// cell axis titles.
func TestPrismRepeatSubstitution(t *testing.T) {
	out := renderCompositeP09(t, "repeat_metrics.json")
	s := string(out)
	if got := strings.Count(s, `<g class="prism-scene"`); got != 2 {
		t.Errorf("prism-scene count = %d, want 2", got)
	}
	// The first cell's y-axis title carries "score"; the second's
	// "latency_ms". Both appear in `<text class="prism-axis-title">`
	// elements emitted by the per-cell axes.
	if !strings.Contains(s, "score") {
		t.Errorf("output missing substituted field name 'score'")
	}
	if !strings.Contains(s, "latency_ms") {
		t.Errorf("output missing substituted field name 'latency_ms'")
	}
}

// TestPrismFacetSharedAxes (PHASE.md gate). Default facet resolve
// puts y on shared; assert exactly one prism-axes data-shared="true"
// block + no per-cell y-axis elements (the shared block carries the
// only y axis).
func TestPrismFacetSharedAxes(t *testing.T) {
	out := renderCompositeP09(t, "facet_by_region.json")
	s := string(out)
	if got := strings.Count(s, `data-shared="true"`); got != 1 {
		t.Errorf("data-shared count = %d, want 1", got)
	}
	// The shared y axis is built explicitly; per-cell y axes are
	// stripped by the encoder. Count occurrences of the axis-title
	// "score" — exactly one (the shared y axis).
	if got := strings.Count(s, ">score</text>"); got != 1 {
		t.Errorf("axis-title 'score' count = %d, want 1 (shared y axis only)", got)
	}
}

// TestPrismNestedFacet (PHASE.md gate). 2x1 outer facet x 2x1 inner
// facet produces 4 scenes; the build chain completes without panic.
func TestPrismNestedFacet(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nested facet panicked: %v", r)
		}
	}()
	out := renderCompositeP09(t, "facet_nested.json")
	if got := strings.Count(string(out), `<g class="prism-scene"`); got != 4 {
		t.Errorf("prism-scene count = %d, want 4 (2 outer regions x 2 inner brands)", got)
	}
}
