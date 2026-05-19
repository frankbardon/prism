package svg_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/render/svg"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/validate"
	_ "github.com/frankbardon/prism/validate/rules"
)

// TestPrismLayerComposition (PHASE.md gate). Render the
// actual_vs_benchmark fixture end-to-end. Output must (a) contain two
// prism-layer groups and (b) match the committed SVG golden.
func TestPrismLayerComposition(t *testing.T) {
	// The fixture references testdata/cohorts/*.pulse relative paths;
	// chdir to repo root so the resolver finds them regardless of the
	// test cwd.
	chdirRepoRootSVG(t)
	out := renderCompositeFromRoot(t, "actual_vs_benchmark.json")
	if got := strings.Count(string(out), `<g class="prism-layer"`); got != 2 {
		t.Errorf("prism-layer count = %d, want 2", got)
	}
	goldenPath := filepath.Join(repoRoot(t), "testdata", "svgs", "actual_vs_benchmark.svg")
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
		t.Errorf("golden drift on actual_vs_benchmark.svg (got %d bytes, want %d)", len(out), len(want))
	}
}

// TestPrismConcatRowMajor (PHASE.md gate). vconcat_metrics.json
// produces a 3-row SceneGrid; assert exactly three prism-scene
// blocks emit in vconcat order.
func TestPrismConcatRowMajor(t *testing.T) {
	out := renderCompositeFromRoot(t, "vconcat_metrics.json")
	count := strings.Count(string(out), `<g class="prism-scene"`)
	if count != 3 {
		t.Fatalf("prism-scene count = %d, want 3", count)
	}
	// Confirm cells appear in the expected order via their
	// data-scene-id attributes.
	re := regexp.MustCompile(`data-scene-id="(scene-\d+)"`)
	matches := re.FindAllStringSubmatch(string(out), -1)
	if len(matches) != 3 {
		t.Fatalf("data-scene-id matches=%d, want 3", len(matches))
	}
	wantOrder := []string{"scene-0", "scene-1", "scene-2"}
	for i, m := range matches {
		if m[1] != wantOrder[i] {
			t.Errorf("scene order: pos %d = %q, want %q", i, m[1], wantOrder[i])
		}
	}
}

// TestPrismScaleResolveShared (PHASE.md gate). The shared y-scale
// fixture's SceneDoc must carry exactly one shared y-axis whose
// domain spans the union of both layers (~[0, 100]). The per-cell
// scene must NOT carry its own y axis.
func TestPrismScaleResolveShared(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "testdata", "specs", "layer_shared_y_scale.json")
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
	if doc.Grid.Shared.Y == nil {
		t.Fatal("Grid.Shared.Y is nil; expected one shared y-axis")
	}
	for _, ax := range doc.Grid.Cells[0].Scene.Axes {
		if ax.Channel == "y" {
			t.Errorf("per-cell y-axis present alongside Grid.Shared.Y (D051 violation)")
		}
	}
	dom := doc.Grid.Shared.Y.Scale.Domain
	if len(dom) < 2 {
		t.Fatalf("shared y domain=%v, want [min,max]", dom)
	}
	mn := dom[0].(float64)
	mx := dom[1].(float64)
	if mn != 0 {
		t.Errorf("shared y min=%v, want 0", mn)
	}
	if mx < 100 {
		t.Errorf("shared y max=%v, want >=100 (union should cover [0,100])", mx)
	}
	// Render JSON dump for debug-friendly assertions of axis shape.
	if blob, err := json.Marshal(doc.Grid.Shared.Y); err == nil {
		_ = blob // available for inspection on -v
	}
}

// TestPrismScaleResolveIncompatible (PHASE.md gate). The negative
// fixture must trip PRISM_PLAN_005 in BOTH validate and the encoder
// (defense-in-depth).
func TestPrismScaleResolveIncompatible(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "testdata", "specs", "invalid", "layer_shared_y_incompatible.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	// 1) Validator path.
	sem := validate.NewDefaultSemanticValidator()
	semErrs := sem.Validate(s, validate.EmptyLookup{})
	foundValidate := false
	for _, e := range semErrs {
		if e.Code == "PRISM_PLAN_005" {
			foundValidate = true
		}
	}
	if !foundValidate {
		t.Errorf("validate did not emit PRISM_PLAN_005, got: %+v", semErrs)
	}

	// 2) Encoder path (defense-in-depth).
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
	_, err = encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err == nil {
		t.Fatal("encoder did not raise PRISM_PLAN_005")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_PLAN_005" {
		t.Errorf("encoder: expected PRISM_PLAN_005, got %v", err)
	}
}

// chdirRepoRootSVG chdirs into the prism repo root for the lifetime
// of the test so spec fixtures with relative .pulse paths resolve.
func chdirRepoRootSVG(t *testing.T) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoRoot(t)); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// renderCompositeFromRoot loads a fixture name relative to testdata/
// specs/, drives it through the full composite pipeline, and returns
// the SVG bytes.
func renderCompositeFromRoot(t *testing.T, fixture string) []byte {
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
