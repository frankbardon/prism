package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestAggCompatHappyPath(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data:   &spec.Data{Name: "cohort"},
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "brand_id", Type: "nominal"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative", Aggregate: "mean"}},
		},
	}
	lookup := validate.NewStaticLookup()
	lookup.Register("cohort", &validate.PulseSchemaShim{Fields: []validate.FieldShim{
		{Name: "brand_id", Type: "nominal"},
		{Name: "score", Type: "quantitative"},
	}})
	errs := AggCompat{}.Check(s, lookup)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestAggCompatFiresOnMeanOfNominal(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data:   &spec.Data{Name: "cohort"},
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "brand_id", Type: "nominal", Aggregate: "mean"}},
		},
	}
	lookup := validate.NewStaticLookup()
	lookup.Register("cohort", &validate.PulseSchemaShim{Fields: []validate.FieldShim{
		{Name: "brand_id", Type: "nominal"},
	}})
	errs := AggCompat{}.Check(s, lookup)
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_002" {
		t.Fatalf("expected one PRISM_SPEC_002, got: %+v", errs)
	}
}

func TestAggCompatTransformOps(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data:   &spec.Data{Name: "cohort"},
		Transform: []spec.Transform{{
			Aggregate: &spec.AggregateTransform{
				Aggregate: []spec.AggregateOp{{Op: "sum", Field: "brand_id", As: "sum_brand"}},
			},
		}},
		Mark: &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "sum_brand", Type: "quantitative"}},
		},
	}
	lookup := validate.NewStaticLookup()
	lookup.Register("cohort", &validate.PulseSchemaShim{Fields: []validate.FieldShim{
		{Name: "brand_id", Type: "nominal"},
	}})
	errs := AggCompat{}.Check(s, lookup)
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_002" {
		t.Fatalf("expected one PRISM_SPEC_002 on sum-of-nominal, got: %+v", errs)
	}
}
