package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// centerSpec builds a leaf spec whose y channel carries the given
// `stack` value on the given mark.
func centerSpec(mark string, stack any) *spec.Spec {
	y := &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{
		Field:     "visits",
		Type:      "quantitative",
		Aggregate: "sum",
	}}
	y.Stack = stack
	return &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: mark},
		Encoding: &spec.Encoding{
			X:     pos("week", "quantitative"),
			Y:     y,
			Color: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "source", Type: "nominal"}},
		},
	}
}

func TestStackCenterMarkAcceptsArea(t *testing.T) {
	if errs := (StackCenterMark{}).Check(centerSpec("area", "center"), validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors on an area streamgraph, got: %+v", errs)
	}
}

func TestStackCenterMarkRejectsBar(t *testing.T) {
	errs := (StackCenterMark{}).Check(centerSpec("bar", "center"), validate.EmptyLookup{})
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got: %+v", errs)
	}
	if errs[0].Code != "PRISM_SPEC_053" {
		t.Errorf("code = %q, want PRISM_SPEC_053", errs[0].Code)
	}
}

func TestStackCenterMarkIgnoresAnchoredOffsets(t *testing.T) {
	for _, stack := range []any{nil, "zero", "normalize", true, false} {
		if errs := (StackCenterMark{}).Check(centerSpec("bar", stack), validate.EmptyLookup{}); len(errs) != 0 {
			t.Errorf("stack %v on a bar: expected no errors, got %+v", stack, errs)
		}
	}
}

func TestStackCenterMarkWalksLayers(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Layer:  []*spec.Spec{centerSpec("area", "center"), centerSpec("bar", "center")},
	}
	errs := (StackCenterMark{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error from the bar layer, got: %+v", errs)
	}
}
