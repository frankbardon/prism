package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func markOrientSpec(mark, orient string) *spec.Spec {
	return &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Def: &spec.MarkDef{Type: mark, Orient: orient}},
		Encoding: &spec.Encoding{
			Y: pos("brand", "nominal"),
			X: pos("score", "quantitative"),
		},
	}
}

func TestMarkOrientSupportedAcceptsHorizontalBar(t *testing.T) {
	if errs := (MarkOrientSupported{}).Check(markOrientSpec("bar", "horizontal"), validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

// TestMarkOrientSupportedAcceptsCartesianFamilies pins the E9-S2
// widening: every mark whose encoder now resolves through
// MarkOrientation accepts both directions.
func TestMarkOrientSupportedAcceptsCartesianFamilies(t *testing.T) {
	marks := []string{"bar", "rect", "area", "tick", "boxplot", "violin", "winloss", "sparkbar", "sparkarea"}
	for _, mark := range marks {
		for _, orient := range []string{"vertical", "horizontal"} {
			if errs := (MarkOrientSupported{}).Check(markOrientSpec(mark, orient), validate.EmptyLookup{}); len(errs) != 0 {
				t.Errorf("mark %q orient %q: expected no errors, got: %+v", mark, orient, errs)
			}
		}
		errs := (MarkOrientSupported{}).Check(markOrientSpec(mark, "radial"), validate.EmptyLookup{})
		if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_046" {
			t.Errorf("mark %q orient radial: expected one PRISM_SPEC_046, got: %+v", mark, errs)
		}
	}
}

func TestMarkOrientSupportedIgnoresUnsetOrient(t *testing.T) {
	// Every pre-E9-S1 spec omits `orient`; the rule must stay silent.
	for _, mark := range []string{"bar", "point", "line", "sankey"} {
		if errs := (MarkOrientSupported{}).Check(markOrientSpec(mark, ""), validate.EmptyLookup{}); len(errs) != 0 {
			t.Errorf("mark %q: expected no errors, got: %+v", mark, errs)
		}
	}
}

func TestMarkOrientSupportedRejectsRadial(t *testing.T) {
	// Radial is vocabulary-only. No mark implements it, including the
	// tree family, which reads orient for a different purpose.
	for _, mark := range []string{"bar", "rect", "tree", "dendrogram", "network"} {
		errs := (MarkOrientSupported{}).Check(markOrientSpec(mark, "radial"), validate.EmptyLookup{})
		if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_046" {
			t.Errorf("mark %q: expected one PRISM_SPEC_046, got: %+v", mark, errs)
		}
	}
}

func TestMarkOrientSupportedRejectsUnorientableMark(t *testing.T) {
	// Silently ignoring orient on these marks is the failure this rule
	// exists to end.
	for _, mark := range []string{"point", "line", "pie", "sankey", "heatmap", "bullet"} {
		errs := (MarkOrientSupported{}).Check(markOrientSpec(mark, "horizontal"), validate.EmptyLookup{})
		if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_046" {
			t.Errorf("mark %q: expected one PRISM_SPEC_046, got: %+v", mark, errs)
		}
	}
}

func TestMarkOrientSupportedRejectsUnknownValue(t *testing.T) {
	errs := (MarkOrientSupported{}).Check(markOrientSpec("bar", "sideways"), validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_046" {
		t.Fatalf("expected one PRISM_SPEC_046, got: %+v", errs)
	}
}

func TestMarkOrientSupportedWalksLayers(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Layer: []*spec.Spec{
			markOrientSpec("bar", "horizontal"),
			markOrientSpec("line", "horizontal"),
		},
	}
	errs := (MarkOrientSupported{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_046" {
		t.Fatalf("expected one PRISM_SPEC_046 from layer[1], got: %+v", errs)
	}
	if got := errs[0].Context["Path"]; got != "layer[1]" {
		t.Errorf("Path = %v, want layer[1]", got)
	}
}
