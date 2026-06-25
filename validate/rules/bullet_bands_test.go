package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestBulletBandsAcceptsAscending(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Def: &spec.MarkDef{Type: "bullet", Bands: []float64{150, 225, 300}}},
	}
	if errs := (BulletBands{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestBulletBandsFiresOnFlatPair(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Def: &spec.MarkDef{Type: "bullet", Bands: []float64{150, 150, 300}}},
	}
	errs := BulletBands{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_036" {
		t.Fatalf("expected exactly one PRISM_SPEC_036, got: %+v", errs)
	}
}

func TestBulletBandsFiresOnDescending(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Def: &spec.MarkDef{Type: "bullet", Bands: []float64{300, 225, 150}}},
	}
	errs := BulletBands{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 2 {
		t.Fatalf("expected two PRISM_SPEC_036 (one per out-of-order pair), got: %+v", errs)
	}
}

func TestBulletBandsIgnoresOtherMarks(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
	}
	if errs := (BulletBands{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors for non-bullet mark, got: %+v", errs)
	}
}
