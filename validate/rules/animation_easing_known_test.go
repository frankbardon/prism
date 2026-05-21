package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestPrismAnimationEasingKnownPasses(t *testing.T) {
	cases := []string{"", "linear", "cubic_in_out", "expo_out"}
	for _, e := range cases {
		s := &spec.Spec{
			Schema:    "urn:prism:schema:v1:spec",
			Animation: &spec.Animation{Easing: e},
		}
		errs := AnimationEasingKnown{}.Check(s, validate.EmptyLookup{})
		if len(errs) != 0 {
			t.Errorf("easing=%q expected no errors, got: %+v", e, errs)
		}
	}
}

func TestPrismAnimationEasingKnownRejects(t *testing.T) {
	s := &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Animation: &spec.Animation{Easing: "elastic_in_out"},
	}
	errs := AnimationEasingKnown{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_022" {
		t.Fatalf("expected one PRISM_SPEC_022 error, got: %+v", errs)
	}
}

func TestPrismAnimationEasingKnownIgnoresAbsentBlock(t *testing.T) {
	s := &spec.Spec{Schema: "urn:prism:schema:v1:spec"}
	errs := AnimationEasingKnown{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Errorf("no-animation spec expected no errors, got: %+v", errs)
	}
}
