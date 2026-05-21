package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestConditionTestParsesHappyPath(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Color: &spec.MarkChannel{
				ChannelCommon: spec.ChannelCommon{
					Condition: &spec.Condition{Single: &spec.ConditionTest{Test: "score >= 0.7", Value: "green"}},
				},
			},
		},
	}
	errs := ConditionTestParses{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
}

func TestConditionTestParsesFiresOnGarbage(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Color: &spec.MarkChannel{
				ChannelCommon: spec.ChannelCommon{
					Condition: &spec.Condition{Single: &spec.ConditionTest{Test: "this ?? is not (valid", Value: "red"}},
				},
			},
		},
	}
	errs := ConditionTestParses{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_026" {
		t.Fatalf("expected one PRISM_SPEC_026, got %+v", errs)
	}
}
