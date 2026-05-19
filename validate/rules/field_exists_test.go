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
	lookup.Register("cohort", &validate.PulseSchemaShim{Fields: []validate.FieldShim{
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
	lookup.Register("cohort", &validate.PulseSchemaShim{Fields: []validate.FieldShim{
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
	lookup.Register("cohort", &validate.PulseSchemaShim{Fields: []validate.FieldShim{
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
