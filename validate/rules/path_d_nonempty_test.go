package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestPrismPathDAcceptsNonEmpty(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Def: &spec.MarkDef{Type: "path", Path: "M 0 0 L 10 10 Z"}},
	}
	errs := PathDNonEmpty{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("non-empty d must be allowed, got: %+v", errs)
	}
}

func TestPrismPathDRejectsEmpty(t *testing.T) {
	for _, d := range []string{"", "   ", "\t\n"} {
		s := &spec.Spec{
			Schema: "urn:prism:schema:v1:spec",
			Mark:   &spec.Mark{Def: &spec.MarkDef{Type: "path", Path: d}},
		}
		errs := PathDNonEmpty{}.Check(s, validate.EmptyLookup{})
		if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_017" {
			t.Errorf("d=%q expected PRISM_SPEC_017, got: %+v", d, errs)
		}
	}
}

func TestPrismPathDIgnoresNonPathMark(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
	}
	errs := PathDNonEmpty{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Errorf("non-path mark should be ignored, got: %+v", errs)
	}
}
