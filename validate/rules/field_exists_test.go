package rules

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestFieldExistsHappyPath(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data:   &spec.Data{Name: "cohort"},
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "brand_id", Type: "nominal"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}},
		},
	}
	lookup := validate.NewStaticLookup()
	lookup.Register("cohort", &validate.SchemaShim{Fields: []validate.FieldShim{
		{Name: "brand_id", Type: "nominal"},
		{Name: "score", Type: "quantitative"},
	}})
	errs := FieldExists{}.Check(s, lookup)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestFieldExistsFiresOnUnknownField(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data:   &spec.Data{Name: "cohort"},
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "xfield", Type: "nominal"}},
		},
	}
	lookup := validate.NewStaticLookup()
	lookup.Register("cohort", &validate.SchemaShim{Fields: []validate.FieldShim{
		{Name: "brand_id", Type: "nominal"},
	}})
	errs := FieldExists{}.Check(s, lookup)
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %d: %+v", len(errs), errs)
	}
	if errs[0].Code != "PRISM_SPEC_001" {
		t.Errorf("expected PRISM_SPEC_001, got %q", errs[0].Code)
	}
	if !strings.Contains(errs[0].Message, "xfield") {
		t.Errorf("expected message to mention xfield, got %q", errs[0].Message)
	}
}

func TestFieldExistsAcceptsTransformOutputs(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data:   &spec.Data{Name: "cohort"},
		Transform: []spec.Transform{{
			Aggregate: &spec.AggregateTransform{
				Groupby:   []string{"brand_id"},
				Aggregate: []spec.AggregateOp{{Op: "mean", Field: "score", As: "score_mean"}},
			},
		}},
		Mark: &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "brand_id", Type: "nominal"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score_mean", Type: "quantitative"}},
		},
	}
	lookup := validate.NewStaticLookup()
	lookup.Register("cohort", &validate.SchemaShim{Fields: []validate.FieldShim{
		{Name: "brand_id", Type: "nominal"},
		{Name: "score", Type: "quantitative"},
	}})
	errs := FieldExists{}.Check(s, lookup)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, transform output should satisfy field, got: %+v", errs)
	}
}

func TestFieldExistsNoOpWithEmptyLookup(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data:   &spec.Data{Name: "cohort"},
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "anything", Type: "nominal"}},
		},
	}
	errs := FieldExists{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected silent no-op with EmptyLookup, got: %+v", errs)
	}
}

// TestFieldExistsAcceptsInnerJoinRightField reproduces the
// docs/src/playground/examples/join.json scenario: an inner join widens
// the checked field set with the right-hand ("with") dataset's own
// fields, so an encoding referencing a right-only field must not fire
// PRISM_SPEC_001.
func TestFieldExistsAcceptsInnerJoinRightField(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data:   &spec.Data{Name: "orders"},
		Transform: []spec.Transform{{
			Join: &spec.JoinTransform{Join: "inner", With: "customers", On: "customer_id", Data: "orders", As: "joined"},
		}},
		Mark: &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "region", Type: "nominal"}},
			Y:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "amount", Type: "quantitative"}},
			Color: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "region", Type: "nominal"}},
		},
	}
	lookup := validate.NewStaticLookup()
	lookup.Register("orders", &validate.SchemaShim{Fields: []validate.FieldShim{
		{Name: "customer_id", Type: "nominal"},
		{Name: "amount", Type: "quantitative"},
	}})
	lookup.Register("customers", &validate.SchemaShim{Fields: []validate.FieldShim{
		{Name: "customer_id", Type: "nominal"},
		{Name: "region", Type: "nominal"},
	}})
	errs := FieldExists{}.Check(s, lookup)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, joined right-side field should satisfy the check, got: %+v", errs)
	}
}

// TestFieldExistsRejectsAntiJoinRightField ensures an anti join — which
// keeps only the left schema — does not admit right-side fields as valid
// references.
func TestFieldExistsRejectsAntiJoinRightField(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data:   &spec.Data{Name: "orders"},
		Transform: []spec.Transform{{
			Join: &spec.JoinTransform{Join: "anti", With: "customers", On: "customer_id", Data: "orders", As: "joined"},
		}},
		Mark: &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "region", Type: "nominal"}},
		},
	}
	lookup := validate.NewStaticLookup()
	lookup.Register("orders", &validate.SchemaShim{Fields: []validate.FieldShim{
		{Name: "customer_id", Type: "nominal"},
		{Name: "amount", Type: "quantitative"},
	}})
	lookup.Register("customers", &validate.SchemaShim{Fields: []validate.FieldShim{
		{Name: "customer_id", Type: "nominal"},
		{Name: "region", Type: "nominal"},
	}})
	errs := FieldExists{}.Check(s, lookup)
	if len(errs) != 1 {
		t.Fatalf("expected one error, anti join should not expose right-side fields, got %d: %+v", len(errs), errs)
	}
	if errs[0].Code != "PRISM_SPEC_001" {
		t.Errorf("expected PRISM_SPEC_001, got %q", errs[0].Code)
	}
}
