package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// scaleDomainSpec builds a minimal bar spec whose y channel carries
// the given scale block.
func scaleDomainSpec(measure string, sc *spec.Scale) *spec.Spec {
	return &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "k", Type: "nominal"},
			},
			Y: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "v", Type: measure, Scale: sc},
			},
		},
	}
}

func TestPrismScaleDomainRejectsMalformed(t *testing.T) {
	cases := []struct {
		name    string
		measure string
		scale   *spec.Scale
	}{
		{"wrong arity", "quantitative", &spec.Scale{Domain: []any{1.0, 2.0, 3.0}}},
		{"single bound", "quantitative", &spec.Scale{Domain: []any{1.0}}},
		{"non numeric", "quantitative", &spec.Scale{Domain: []any{"lo", "hi"}}},
		{"reversed", "quantitative", &spec.Scale{Domain: []any{130.0, 90.0}}},
		{"zero width", "quantitative", &spec.Scale{Domain: []any{5.0, 5.0}}},
		{"not an array", "quantitative", &spec.Scale{Domain: "wide"}},
		{"empty categories", "nominal", &spec.Scale{Domain: []any{}}},
		{"non string category", "nominal", &spec.Scale{Domain: []any{1.0}}},
		{"time arity", "temporal", &spec.Scale{Domain: []any{"2024-01-01T00:00:00Z"}}},
		{"time bad bound", "temporal", &spec.Scale{Domain: []any{true, false}}},
		{"typed log reversed", "", &spec.Scale{Type: "log", Domain: []any{100.0, 10.0}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := ScaleDomain{}.Check(scaleDomainSpec(tc.measure, tc.scale), validate.EmptyLookup{})
			if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_041" {
				t.Fatalf("expected one PRISM_SPEC_041, got: %+v", errs)
			}
		})
	}
}

func TestPrismScaleDomainAcceptsWellFormed(t *testing.T) {
	cases := []struct {
		name    string
		measure string
		scale   *spec.Scale
	}{
		{"no domain", "quantitative", &spec.Scale{Zero: new(bool)}},
		{"nil scale", "quantitative", nil},
		{"ascending numbers", "quantitative", &spec.Scale{Domain: []any{90.0, 130.0}}},
		{"int bounds", "quantitative", &spec.Scale{Domain: []any{0, 10}}},
		{"categories", "nominal", &spec.Scale{Domain: []any{"c", "b", "a"}}},
		{"iso times", "temporal", &spec.Scale{Domain: []any{"2024-01-01T00:00:00Z", "2024-12-31T00:00:00Z"}}},
		{"epoch times", "temporal", &spec.Scale{Domain: []any{0.0, 1000.0}}},
		{"unknown family", "", &spec.Scale{Domain: []any{"anything"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := ScaleDomain{}.Check(scaleDomainSpec(tc.measure, tc.scale), validate.EmptyLookup{})
			if len(errs) != 0 {
				t.Fatalf("expected no errors, got: %+v", errs)
			}
		})
	}
}

// TestPrismScaleDomainWalksLayers proves the rule descends into
// composition children rather than only inspecting the root encoding.
func TestPrismScaleDomainWalksLayers(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Layer: []*spec.Spec{
			scaleDomainSpec("quantitative", &spec.Scale{Domain: []any{0.0, 10.0}}),
			scaleDomainSpec("quantitative", &spec.Scale{Domain: []any{10.0, 0.0}}),
		},
	}
	errs := ScaleDomain{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_041" {
		t.Fatalf("expected one PRISM_SPEC_041 from the second layer, got: %+v", errs)
	}
}
