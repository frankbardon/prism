package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestSchemaRefAcceptsURN(t *testing.T) {
	s := &spec.Spec{Schema: "urn:prism:schema:v1:spec", Mark: &spec.Mark{Shorthand: "bar"}}
	errs := SchemaRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestSchemaRefAcceptsRelativePath(t *testing.T) {
	for _, ref := range []string{
		"./.prism/schemas/spec.schema.json",
		"../schemas/spec.schema.json",
		"/etc/prism/spec.schema.json",
		"file:///opt/prism/spec.schema.json",
	} {
		s := &spec.Spec{Schema: ref, Mark: &spec.Mark{Shorthand: "bar"}}
		errs := SchemaRef{}.Check(s, validate.EmptyLookup{})
		if len(errs) != 0 {
			t.Errorf("expected %q to be accepted, got: %+v", ref, errs)
		}
	}
}

func TestSchemaRefRejectsHTTPS(t *testing.T) {
	s := &spec.Spec{Schema: "https://example.com/spec.schema.json", Mark: &spec.Mark{Shorthand: "bar"}}
	errs := SchemaRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_009" {
		t.Fatalf("expected one PRISM_SPEC_009, got: %+v", errs)
	}
}

func TestSchemaRefRejectsWrongURN(t *testing.T) {
	s := &spec.Spec{Schema: "urn:prism:schema:v2:spec", Mark: &spec.Mark{Shorthand: "bar"}}
	errs := SchemaRef{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 {
		t.Fatalf("expected one error, got: %+v", errs)
	}
}
