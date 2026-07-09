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
					Condition: &spec.Condition{Single: &spec.ConditionTest{
						Test:  &spec.Predicate{Op: spec.PredGte, Field: "score", Value: 0.7},
						Value: "green",
					}},
				},
			},
		},
	}
	errs := ConditionTestParses{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
}

// TestConditionTestParsesFiresOnMalformedPredicate — a schema-aware
// problem (between with lo > hi) surfaces as PRISM_SPEC_026 via the
// shared checkPredicate helper, without needing a registered schema.
func TestConditionTestParsesFiresOnMalformedPredicate(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Color: &spec.MarkChannel{
				ChannelCommon: spec.ChannelCommon{
					Condition: &spec.Condition{Single: &spec.ConditionTest{
						Test:  &spec.Predicate{Op: spec.PredBetween, Field: "score", Lo: 10.0, Hi: 1.0},
						Value: "red",
					}},
				},
			},
		},
	}
	errs := ConditionTestParses{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_026" {
		t.Fatalf("expected one PRISM_SPEC_026, got %+v", errs)
	}
}
