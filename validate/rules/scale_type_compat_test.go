package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestScaleTypeCompatAcceptsLogOnQuantitative(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Y: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative",
					Scale: &spec.Scale{Type: "log"}},
			},
		},
	}
	errs := ScaleTypeCompat{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestScaleTypeCompatRejectsLogOnNominal(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "brand_id", Type: "nominal",
					Scale: &spec.Scale{Type: "log"}},
			},
		},
	}
	errs := ScaleTypeCompat{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_007" {
		t.Fatalf("expected one PRISM_SPEC_007, got: %+v", errs)
	}
}
