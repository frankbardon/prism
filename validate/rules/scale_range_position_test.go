package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
)

// rangePositionSpec builds a minimal bar spec whose y channel carries
// the given scale block, plus an optional color channel scale.
func rangePositionSpec(y, color *spec.Scale) *spec.Spec {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "k", Type: "nominal"},
			},
			Y: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "v", Type: "quantitative", Scale: y},
			},
		},
	}
	if color != nil {
		s.Encoding.Color = &spec.MarkChannel{
			ChannelCommon: spec.ChannelCommon{Field: "k", Type: "nominal", Scale: color},
		}
	}
	return s
}

func TestPrismScaleRangePositionRejected(t *testing.T) {
	errs := ScaleRangePosition{}.Check(
		rangePositionSpec(&spec.Scale{Range: []any{0.0, 200.0}}, nil), nil)
	if len(errs) != 1 {
		t.Fatalf("want 1 error for a position scale.range, got %d", len(errs))
	}
	if errs[0].Code != "PRISM_SPEC_044" {
		t.Fatalf("want PRISM_SPEC_044, got %s", errs[0].Code)
	}
	if got := errs[0].Context["Channel"]; got != "y" {
		t.Fatalf("want Channel=y, got %v", got)
	}
}

func TestPrismScaleRangeColorAllowed(t *testing.T) {
	s := rangePositionSpec(nil, &spec.Scale{Range: []any{"#aaa", "#bbb"}})
	if errs := (ScaleRangePosition{}).Check(s, nil); len(errs) != 0 {
		t.Fatalf("a color scale.range must pass, got %d errors: %v", len(errs), errs)
	}
}

func TestPrismScaleRangePositionAbsentPasses(t *testing.T) {
	s := rangePositionSpec(&spec.Scale{Zero: new(bool)}, nil)
	if errs := (ScaleRangePosition{}).Check(s, nil); len(errs) != 0 {
		t.Fatalf("a scale block without range must pass, got %v", errs)
	}
}

func TestPrismScaleRangePositionWalksLayers(t *testing.T) {
	child := rangePositionSpec(&spec.Scale{Range: []any{0.0, 10.0}}, nil)
	root := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Layer:  []*spec.Spec{child},
	}
	errs := ScaleRangePosition{}.Check(root, nil)
	if len(errs) != 1 {
		t.Fatalf("want 1 error from the layer child, got %d", len(errs))
	}
}

func TestPrismScaleRangePositionEveryChannel(t *testing.T) {
	rng := &spec.Scale{Range: []any{1.0, 2.0}}
	enc := &spec.Encoding{
		X:      &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "a", Type: "quantitative", Scale: rng}},
		Y:      &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "b", Type: "quantitative", Scale: rng}},
		X2:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "c", Type: "quantitative", Scale: rng}},
		Y2:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "d", Type: "quantitative", Scale: rng}},
		Theta:  &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "e", Type: "quantitative", Scale: rng}},
		Radius: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "f", Type: "quantitative", Scale: rng}},
	}
	errs := checkScaleRangePosition(enc)
	if len(errs) != 6 {
		t.Fatalf("want one error per position channel (6), got %d", len(errs))
	}
}
