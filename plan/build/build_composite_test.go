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

func TestPrismBuildLayerProducesPerLayerSubDAGs(t *testing.T) {
	root := repoRoot(t)
	s := loadSpec(t, filepath.Join(root, "testdata", "specs", "layer_actual_vs_benchmark.json"))
	c, err := build.BuildComposite(s, build.Options{})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	if c.Kind != plan.CompositeLayer {
		t.Errorf("Kind=%q, want %q", c.Kind, plan.CompositeLayer)
	}
	if got, want := len(c.Children), 2; got != want {
		t.Fatalf("Children=%d, want %d", got, want)
	}
	if c.Rows != 1 || c.Cols != 1 {
		t.Errorf("Layer shape=%dx%d, want 1x1", c.Rows, c.Cols)
	}
	for i, child := range c.Children {
		if child.DAG == nil || child.DAG.Size() == 0 {
			t.Errorf("child %d: empty DAG", i)
		}
		if child.Tip == "" {
			t.Errorf("child %d: empty tip", i)
		}
		if child.Spec == nil {
			t.Errorf("child %d: nil spec", i)
		}
	}
}

func TestPrismBuildLayerInheritsParentDatasets(t *testing.T) {
	// Synthetic: parent declares datasets.foo; layer references it
	// via {"data": {"name": "foo"}} without redeclaring.
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"datasets": {
			"foo": {"values": [{"x": "a", "y": 1}, {"x": "b", "y": 2}]}
		},
		"layer": [
			{
				"$schema": "urn:prism:schema:v1:spec",
				"data": {"name": "foo"},
				"mark": "bar",
				"encoding": {
					"x": {"field": "x", "type": "nominal"},
					"y": {"field": "y", "type": "quantitative"}
				}
			}
		]
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	if len(c.Children) != 1 {
		t.Fatalf("Children=%d, want 1", len(c.Children))
	}
	if c.Children[0].DAG.Size() == 0 {
		t.Error("inherited dataset did not produce a leaf node")
	}
}

func TestPrismBuildLayerPerLayerDataOverride(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"datasets": {
			"alpha": {"values": [{"x": "a", "y": 1}]}
		},
		"layer": [
			{
				"$schema": "urn:prism:schema:v1:spec",
				"data": {"values": [{"x": "z", "y": 99}]},
				"mark": "bar",
				"encoding": {
					"x": {"field": "x", "type": "nominal"},
					"y": {"field": "y", "type": "quantitative"}
				}
			}
		]
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	// Child gets its own inline data; the inherited `alpha` dataset
	// is still registered as a leaf (parent datasets are inherited
	// even when the child overrides data), so we just assert the
	// child has at least one root + the tip resolves.
	if c.Children[0].Tip == "" {
		t.Fatal("empty tip after override")
	}
}

func TestPrismBuildConcatHorizontal(t *testing.T) {
	root := repoRoot(t)
	s := loadSpec(t, filepath.Join(root, "testdata", "specs", "concat_h.json"))
	c, err := build.BuildComposite(s, build.Options{})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	if c.Kind != plan.CompositeHConcat {
		t.Errorf("Kind=%q, want %q", c.Kind, plan.CompositeHConcat)
	}
	if c.Rows != 1 || c.Cols != 2 {
		t.Errorf("shape=%dx%d, want 1x2", c.Rows, c.Cols)
	}
	if len(c.Children) != 2 {
		t.Fatalf("Children=%d, want 2", len(c.Children))
	}
}

func TestPrismBuildConcatVertical(t *testing.T) {
	root := repoRoot(t)
	s := loadSpec(t, filepath.Join(root, "testdata", "specs", "concat_v.json"))
	c, err := build.BuildComposite(s, build.Options{})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	if c.Kind != plan.CompositeVConcat {
		t.Errorf("Kind=%q, want %q", c.Kind, plan.CompositeVConcat)
	}
	if c.Rows != 2 || c.Cols != 1 {
		t.Errorf("shape=%dx%d, want 2x1", c.Rows, c.Cols)
	}
}

func TestPrismBuildNestedCompositionRejected(t *testing.T) {
	// concat[ layer[...] ] — nested composition.
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"hconcat": [
			{
				"$schema": "urn:prism:schema:v1:spec",
				"datasets": {
					"foo": {"values": [{"x": "a", "y": 1}]}
				},
				"layer": [
					{
						"$schema": "urn:prism:schema:v1:spec",
						"data": {"name": "foo"},
						"mark": "bar",
						"encoding": {
							"x": {"field": "x", "type": "nominal"},
							"y": {"field": "y", "type": "quantitative"}
						}
					}
				]
			}
		]
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	_, err = build.BuildComposite(s, build.Options{})
	if err == nil {
		t.Fatal("expected PRISM_PLAN_002 (nested composition), got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_PLAN_002" {
		t.Fatalf("expected PRISM_PLAN_002, got %v", err)
	}
	if got := ae.Context["Kind"]; got != "composition:nested" {
		t.Errorf("Kind=%v, want composition:nested", got)
	}
}

func TestPrismBuildRejectsCompositeViaFlatBuild(t *testing.T) {
	root := repoRoot(t)
	s := loadSpec(t, filepath.Join(root, "testdata", "specs", "concat_h.json"))
	_, _, err := build.Build(s, build.Options{})
	if err == nil {
		t.Fatal("expected PRISM_PLAN_002 from flat Build on composite")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_PLAN_002" {
		t.Fatalf("expected PRISM_PLAN_002, got %v", err)
	}
	if got := ae.Context["Kind"]; got != "composition:flat-build" {
		t.Errorf("Kind=%v, want composition:flat-build", got)
	}
}

func TestPrismIsCompositeDetectsAllFour(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		want     bool
	}{
		{"flat", `{"$schema":"urn:prism:schema:v1:spec","data":{"values":[{"x":1}]},"mark":"bar","encoding":{"x":{"field":"x","type":"quantitative"}}}`, false},
		{"layer", `{"$schema":"urn:prism:schema:v1:spec","layer":[{"$schema":"urn:prism:schema:v1:spec","data":{"values":[{"x":1}]},"mark":"bar","encoding":{"x":{"field":"x","type":"quantitative"}}}]}`, true},
		{"hconcat", `{"$schema":"urn:prism:schema:v1:spec","hconcat":[{"$schema":"urn:prism:schema:v1:spec","data":{"values":[{"x":1}]},"mark":"bar","encoding":{"x":{"field":"x","type":"quantitative"}}}]}`, true},
		{"vconcat", `{"$schema":"urn:prism:schema:v1:spec","vconcat":[{"$schema":"urn:prism:schema:v1:spec","data":{"values":[{"x":1}]},"mark":"bar","encoding":{"x":{"field":"x","type":"quantitative"}}}]}`, true},
		{"concat", `{"$schema":"urn:prism:schema:v1:spec","concat":[{"$schema":"urn:prism:schema:v1:spec","data":{"values":[{"x":1}]},"mark":"bar","encoding":{"x":{"field":"x","type":"quantitative"}}}]}`, true},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			s, err := spec.DecodeBytes([]byte(c.body))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got := build.IsComposite(s); got != c.want {
				t.Errorf("IsComposite=%v, want %v", got, c.want)
			}
		})
	}
}
