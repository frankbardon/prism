package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestChannelForMarkAcceptsBarXY(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "x", Type: "nominal"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "y", Type: "quantitative"}},
		},
	}
	errs := ChannelForMark{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestChannelForMarkRejectsThetaOnBar(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Theta: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}},
		},
	}
	errs := ChannelForMark{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_003" {
		t.Fatalf("expected one PRISM_SPEC_003, got: %+v", errs)
	}
}

func TestChannelForMarkAcceptsThetaOnPie(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "pie"},
		Encoding: &spec.Encoding{
			Theta: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}},
			Color: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "brand_id", Type: "nominal"}},
		},
	}
	errs := ChannelForMark{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors on pie+theta+color, got: %+v", errs)
	}
}

// TestChannelForMarkAcceptsTableColumnsOnly asserts a table mark with
// only encoding.columns[] (no x/y) does not trip PRISM_SPEC_003 — a
// table has no position-channel requirement, and Columns is not among
// the scalar channels this rule inspects.
func TestChannelForMarkAcceptsTableColumnsOnly(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Def: &spec.MarkDef{Type: "table"}},
		Encoding: &spec.Encoding{
			Columns: []spec.TableColumn{
				{ChannelCommon: spec.ChannelCommon{Field: "name", Type: "nominal"}},
			},
		},
	}
	errs := ChannelForMark{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors on table with only columns, got: %+v", errs)
	}
}

// TestChannelForMarkRejectsXOnTable asserts a stray position channel
// on a table mark is rejected — table's allowlist carries no
// position/mark channels, only the common set.
func TestChannelForMarkRejectsXOnTable(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Def: &spec.MarkDef{Type: "table"}},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "x", Type: "nominal"}},
		},
	}
	errs := ChannelForMark{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_003" {
		t.Fatalf("expected one PRISM_SPEC_003, got: %+v", errs)
	}
}
