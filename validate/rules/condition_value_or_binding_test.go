package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestConditionValueOrBindingHappyPath(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"brush": {Interval: &spec.IntervalSelection{Type: "interval"}},
		},
		Mark: &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Color: &spec.MarkChannel{
				ChannelCommon: spec.ChannelCommon{
					Condition: &spec.Condition{Multi: []spec.ConditionTest{
						{Selection: "brush", Value: "red"},
						{Test: "score < 0", Field: "score_bucket", Type: "ordinal"},
						{Selection: "brush"}, // inherit channel field — allowed
					}},
				},
			},
		},
	}
	errs := ConditionValueOrBinding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
}

func TestConditionValueOrBindingRejectsBoth(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Color: &spec.MarkChannel{
				ChannelCommon: spec.ChannelCommon{
					Condition: &spec.Condition{Single: &spec.ConditionTest{
						Test: "x > 0", Value: "red", Field: "score",
					}},
				},
			},
		},
	}
	errs := ConditionValueOrBinding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_027" {
		t.Fatalf("expected one PRISM_SPEC_027, got %+v", errs)
	}
}

func TestConditionValueOrBindingRejectsNeitherOnTest(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Color: &spec.MarkChannel{
				ChannelCommon: spec.ChannelCommon{
					Condition: &spec.Condition{Single: &spec.ConditionTest{Test: "x > 0"}},
				},
			},
		},
	}
	errs := ConditionValueOrBinding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_027" {
		t.Fatalf("expected one PRISM_SPEC_027, got %+v", errs)
	}
}
