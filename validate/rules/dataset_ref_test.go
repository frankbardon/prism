package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestDatasetRefAcceptsDeclaredDataset(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Datasets: map[string]*spec.Data{
			"cohort": {Source: "cohorts/x.pulse"},
		},
		Data: &spec.Data{Name: "cohort"},
		Mark: &spec.Mark{Shorthand: "bar"},
	}
	errs := DatasetRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestDatasetRefFiresOnUndeclared(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data:   &spec.Data{Name: "missing"},
		Mark:   &spec.Mark{Shorthand: "bar"},
	}
	errs := DatasetRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_005" {
		t.Fatalf("expected one PRISM_SPEC_005, got: %+v", errs)
	}
}

func TestDatasetRefAcceptsExternalLookup(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data:   &spec.Data{Name: "registered"},
		Mark:   &spec.Mark{Shorthand: "bar"},
	}
	lookup := validate.NewStaticLookup()
	lookup.Register("registered", &validate.PulseSchemaShim{})
	errs := DatasetRef{}.Check(s, lookup)
	if len(errs) != 0 {
		t.Fatalf("expected no errors with external lookup, got: %+v", errs)
	}
}

func TestDatasetRefAcceptsJoinAndUnionTargets(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Datasets: map[string]*spec.Data{
			"left":  {Source: "a.pulse"},
			"right": {Source: "b.pulse"},
			"extra": {Source: "c.pulse"},
		},
		Transform: []spec.Transform{
			{Join: &spec.JoinTransform{Join: "inner", With: "right", On: "id", Data: "left", As: "joined"}},
			{Union: &spec.UnionTransform{Union: []string{"joined", "extra"}, As: "merged"}},
		},
		Mark: &spec.Mark{Shorthand: "bar"},
	}
	errs := DatasetRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}
