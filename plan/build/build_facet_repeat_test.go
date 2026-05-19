package build_test

import (
	"errors"
	"path/filepath"
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/spec"
)

// TestPrismBuildFacetSingleColumn pins facet-by-column build: the
// returned CompositeDAG carries one shared upstream child (D054);
// rows / cols stay zero (encoder fills in after partitioning).
func TestPrismBuildFacetSingleColumn(t *testing.T) {
	root := repoRoot(t)
	s := loadSpec(t, filepath.Join(root, "testdata", "specs", "facet_by_region.json"))
	c, err := build.BuildComposite(s, build.Options{})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	if c.Kind != plan.CompositeFacet {
		t.Errorf("Kind=%q, want %q", c.Kind, plan.CompositeFacet)
	}
	if got, want := len(c.Children), 1; got != want {
		t.Fatalf("Children=%d, want %d (facet shared-upstream convention, D054)", got, want)
	}
	if c.Rows != 0 || c.Cols != 0 {
		t.Errorf("facet Rows/Cols=%dx%d, want 0x0 placeholders (encoder fills in)", c.Rows, c.Cols)
	}
	child := c.Children[0]
	if child.DAG == nil || child.DAG.Size() == 0 {
		t.Error("shared upstream DAG is empty")
	}
	if child.Spec == nil {
		t.Fatal("merged child spec missing")
	}
	if child.Spec.Mark == nil || child.Spec.Mark.TypeName() != "bar" {
		t.Errorf("child spec mark = %v, want bar", child.Spec.Mark)
	}
}

// TestPrismBuildRepeatRowMajor pins repeat build: per-cell sub-DAGs
// land in Children with the substituted field name (D056).
func TestPrismBuildRepeatRowMajor(t *testing.T) {
	root := repoRoot(t)
	s := loadSpec(t, filepath.Join(root, "testdata", "specs", "repeat_metrics.json"))
	c, err := build.BuildComposite(s, build.Options{})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	if c.Kind != plan.CompositeRepeat {
		t.Errorf("Kind=%q, want %q", c.Kind, plan.CompositeRepeat)
	}
	// repeat_metrics.json declares row: ["score", "latency_ms"]
	if got, want := len(c.Children), 2; got != want {
		t.Fatalf("Children=%d, want %d (one per row field)", got, want)
	}
	if c.Rows != 2 || c.Cols != 1 {
		t.Errorf("repeat Rows/Cols=%dx%d, want 2x1", c.Rows, c.Cols)
	}
	wantY := []string{"score", "latency_ms"}
	for i, child := range c.Children {
		if child.Spec == nil || child.Spec.Encoding == nil || child.Spec.Encoding.Y == nil {
			t.Errorf("child %d: missing y encoding", i)
			continue
		}
		got := child.Spec.Encoding.Y.Field
		if got != wantY[i] {
			t.Errorf("child %d y.Field = %q, want %q", i, got, wantY[i])
		}
		if child.Spec.Encoding.Y.FieldRef != nil {
			t.Errorf("child %d: FieldRef should be cleared after substitution, got %+v", i, child.Spec.Encoding.Y.FieldRef)
		}
	}
}

// TestPrismBuildRepeatRejectsUnknownAxis pins PRISM_SPEC_012: a
// substitution that references an axis the parent does not declare
// raises the new error code at build time.
func TestPrismBuildRepeatRejectsUnknownAxis(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [{"day": "2026-01-01", "a": 1.0, "b": 2.0}]},
		"repeat": {"row": ["a"]},
		"spec": {
			"$schema": "urn:prism:schema:v1:spec",
			"mark": "bar",
			"encoding": {
				"x": {"field": "day", "type": "nominal"},
				"y": {"field": {"repeat": "column"}, "type": "quantitative"}
			}
		}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	_, err = build.BuildComposite(s, build.Options{})
	if err == nil {
		t.Fatal("expected PRISM_SPEC_012, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_SPEC_012" {
		t.Fatalf("expected PRISM_SPEC_012, got %v", err)
	}
	if ae.Context["Axis"] != "column" {
		t.Errorf("Context[Axis]=%v, want column", ae.Context["Axis"])
	}
}

// TestPrismBuildFacetNestedChildSpec asserts nested-facet build
// succeeds at the outer build (inner build happens lazily per
// partition at encode time).
func TestPrismBuildFacetNestedChildSpec(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [
			{"r": "A", "c": "x", "v": 1.0},
			{"r": "A", "c": "y", "v": 2.0},
			{"r": "B", "c": "x", "v": 3.0},
			{"r": "B", "c": "y", "v": 4.0}
		]},
		"facet": {"row": {"field": "r", "type": "nominal"}},
		"spec": {
			"$schema": "urn:prism:schema:v1:spec",
			"facet": {"column": {"field": "c", "type": "nominal"}},
			"spec": {
				"$schema": "urn:prism:schema:v1:spec",
				"mark": "bar",
				"encoding": {
					"x": {"field": "c", "type": "nominal"},
					"y": {"field": "v", "type": "quantitative"}
				}
			}
		}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	if c.Kind != plan.CompositeFacet {
		t.Errorf("outer Kind=%q, want %q", c.Kind, plan.CompositeFacet)
	}
	if len(c.Children) != 1 {
		t.Fatalf("outer Children=%d, want 1", len(c.Children))
	}
	// The inner facet spec must survive intact for the encoder to
	// recurse on it.
	innerSpec := c.Children[0].Spec
	if innerSpec == nil || innerSpec.Facet == nil {
		t.Fatal("inner facet spec not preserved on child")
	}
	if innerSpec.ChildSpec == nil {
		t.Fatal("inner facet's child spec missing")
	}
}

// TestPrismBuildRepeatColumnAxis pins repeat with column field list
// (orthogonal to TestPrismBuildRepeatRowMajor).
func TestPrismBuildRepeatColumnAxis(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"values": [{"day": "2026-01-01", "score": 0.4, "lift": 1.2}]},
		"repeat": {"column": ["score", "lift"]},
		"spec": {
			"$schema": "urn:prism:schema:v1:spec",
			"mark": "line",
			"encoding": {
				"x": {"field": "day", "type": "temporal"},
				"y": {"field": {"repeat": "column"}, "type": "quantitative"}
			}
		}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	if c.Rows != 1 || c.Cols != 2 {
		t.Errorf("Rows/Cols=%dx%d, want 1x2", c.Rows, c.Cols)
	}
	wantY := []string{"score", "lift"}
	for i, child := range c.Children {
		got := child.Spec.Encoding.Y.Field
		if got != wantY[i] {
			t.Errorf("child %d y.Field = %q, want %q", i, got, wantY[i])
		}
	}
}
