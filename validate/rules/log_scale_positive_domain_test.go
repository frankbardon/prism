package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// TestPrismLogScaleNonPositiveDomain — required by PHASE.md.
// Asserts PRISM_SPEC_010 fires when a log-scaled channel binds a
// field with a zero or negative value in inline data.values.
func TestPrismLogScaleNonPositiveDomain(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data: &spec.Data{
			Name: "d",
			Values: []map[string]any{
				{"x": "a", "y": 1.0},
				{"x": "b", "y": 0.0}, // zero — should trip the rule
				{"x": "c", "y": 100.0},
			},
		},
		Mark: &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "x", Type: "nominal"},
			},
			Y: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{
					Field: "y", Type: "quantitative",
					Scale: &spec.Scale{Type: "log"},
				},
			},
		},
	}
	errs := LogScalePositiveDomain{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_010" {
		t.Fatalf("expected one PRISM_SPEC_010, got: %+v", errs)
	}
}

// TestLogScaleAllPositivePasses ensures the rule does NOT fire on a
// strictly-positive domain.
func TestLogScaleAllPositivePasses(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data: &spec.Data{
			Name: "d",
			Values: []map[string]any{
				{"x": "a", "y": 1.0},
				{"x": "b", "y": 10.0},
				{"x": "c", "y": 100.0},
			},
		},
		Mark: &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Y: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{
					Field: "y", Type: "quantitative",
					Scale: &spec.Scale{Type: "log"},
				},
			},
		},
	}
	errs := LogScalePositiveDomain{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Errorf("expected no errors on positive domain, got: %+v", errs)
	}
}

// TestLogScaleNegativeValueTrips asserts a negative value also fires.
func TestLogScaleNegativeValueTrips(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Data: &spec.Data{
			Name: "d",
			Values: []map[string]any{
				{"x": "a", "y": -5.0},
				{"x": "b", "y": 10.0},
			},
		},
		Mark: &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Y: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{
					Field: "y", Type: "quantitative",
					Scale: &spec.Scale{Type: "log"},
				},
			},
		},
	}
	errs := LogScalePositiveDomain{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_010" {
		t.Errorf("expected one PRISM_SPEC_010, got: %+v", errs)
	}
}
