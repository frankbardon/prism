package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestPieDonutAcceptsThetaColor(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "pie"},
		Encoding: &spec.Encoding{
			Theta: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "value", Type: "quantitative"}},
			Color: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "category", Type: "nominal"}},
		},
	}
	errs := PieDonutEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPieDonutFiresOnXY(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "pie"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "category", Type: "nominal"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "value", Type: "quantitative"}},
		},
	}
	errs := PieDonutEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) < 1 {
		t.Fatalf("expected at least one PRISM_SPEC_008, got: %+v", errs)
	}
	for _, e := range errs {
		if e.Code != "PRISM_SPEC_008" {
			t.Errorf("unexpected code: %s", e.Code)
		}
	}
}

func TestPieDonutFiresOnMissingTheta(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "donut"},
		Encoding: &spec.Encoding{
			Color: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "category", Type: "nominal"}},
		},
	}
	errs := PieDonutEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_008" {
		t.Fatalf("expected exactly one PRISM_SPEC_008, got: %+v", errs)
	}
}
