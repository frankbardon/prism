package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestConditionSelectionRefHappyPath(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"brush": {Interval: &spec.IntervalSelection{Type: "interval"}},
		},
		Mark: &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Color: &spec.MarkChannel{
				ChannelCommon: spec.ChannelCommon{
					Condition: &spec.Condition{Single: &spec.ConditionTest{Selection: "brush", Value: "red"}},
				},
			},
		},
	}
	errs := ConditionSelectionRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
}

func TestConditionSelectionRefFiresOnUnknown(t *testing.T) {
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
						{Selection: "typo", Value: "red"},
					}},
				},
			},
		},
	}
	errs := ConditionSelectionRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_025" {
		t.Fatalf("expected one PRISM_SPEC_025, got %+v", errs)
	}
}

func TestConditionSelectionRefIgnoresTestEntries(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Color: &spec.MarkChannel{
				ChannelCommon: spec.ChannelCommon{
					Condition: &spec.Condition{Single: &spec.ConditionTest{Test: "score > 0", Value: "red"}},
				},
			},
		},
	}
	errs := ConditionSelectionRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors for test entries, got %+v", errs)
	}
}

func TestConditionSelectionRefWalksLayers(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Layer: []*spec.Spec{{
			Encoding: &spec.Encoding{
				Color: &spec.MarkChannel{
					ChannelCommon: spec.ChannelCommon{
						Condition: &spec.Condition{Single: &spec.ConditionTest{Selection: "ghost", Value: "red"}},
					},
				},
			},
		}},
	}
	errs := ConditionSelectionRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_025" {
		t.Fatalf("expected one PRISM_SPEC_025 from layer, got %+v", errs)
	}
}
