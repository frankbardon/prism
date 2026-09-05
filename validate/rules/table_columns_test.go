package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestTableColumnsAcceptsPopulatedColumns(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Def: &spec.MarkDef{Type: "table"}},
		Encoding: &spec.Encoding{
			Columns: []spec.TableColumn{
				{ChannelCommon: spec.ChannelCommon{Field: "name", Type: "nominal"}},
			},
		},
	}
	if errs := (TableColumns{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestTableColumnsFiresOnMissingEncoding(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Def: &spec.MarkDef{Type: "table"}},
	}
	errs := TableColumns{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_040" {
		t.Fatalf("expected exactly one PRISM_SPEC_040, got: %+v", errs)
	}
}

func TestTableColumnsFiresOnEmptyColumns(t *testing.T) {
	s := &spec.Spec{
		Schema:   "urn:prism:schema:v1:spec",
		Mark:     &spec.Mark{Def: &spec.MarkDef{Type: "table"}},
		Encoding: &spec.Encoding{},
	}
	errs := TableColumns{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_040" {
		t.Fatalf("expected exactly one PRISM_SPEC_040, got: %+v", errs)
	}
}

func TestTableColumnsIgnoresOtherMarks(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
	}
	if errs := (TableColumns{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors for non-table mark, got: %+v", errs)
	}
}
