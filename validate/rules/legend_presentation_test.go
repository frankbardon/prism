package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// legendChannel builds a color channel of the given type carrying the
// supplied legend block.
func legendChannel(ty string, lg *spec.Legend) *spec.MarkChannel {
	ch := &spec.MarkChannel{}
	ch.Field = "g"
	ch.Type = ty
	ch.Legend = lg
	return ch
}

func legendRuleSpec(ch *spec.MarkChannel) *spec.Spec {
	return &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X:     pos("cat", "nominal"),
			Y:     pos("val", "quantitative"),
			Color: ch,
		},
	}
}

func TestLegendTypeCoherentAcceptsMatchingOverrides(t *testing.T) {
	cases := []struct{ channelType, legendType string }{
		{"nominal", ""},
		{"quantitative", ""},
		{"nominal", "symbol"},
		{"ordinal", "symbol"},
		{"temporal", "symbol"},
		{"quantitative", "gradient"},
	}
	for _, c := range cases {
		s := legendRuleSpec(legendChannel(c.channelType, &spec.Legend{Type: c.legendType}))
		if errs := (LegendTypeCoherent{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
			t.Errorf("channel %q + legend.type %q: expected no errors, got %+v",
				c.channelType, c.legendType, errs)
		}
	}
}

func TestLegendTypeCoherentRejectsMismatchedOverrides(t *testing.T) {
	cases := []struct{ channelType, legendType string }{
		{"nominal", "gradient"},
		{"ordinal", "gradient"},
		{"temporal", "gradient"},
		{"quantitative", "symbol"},
	}
	for _, c := range cases {
		s := legendRuleSpec(legendChannel(c.channelType, &spec.Legend{Type: c.legendType}))
		errs := (LegendTypeCoherent{}).Check(s, validate.EmptyLookup{})
		if len(errs) != 1 {
			t.Fatalf("channel %q + legend.type %q: expected 1 error, got %+v",
				c.channelType, c.legendType, errs)
		}
		if errs[0].Code != "PRISM_SPEC_051" {
			t.Errorf("code = %q, want PRISM_SPEC_051", errs[0].Code)
		}
	}
}

func TestLegendTypeCoherentWalksLayers(t *testing.T) {
	child := legendRuleSpec(legendChannel("nominal", &spec.Legend{Type: "gradient"}))
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Layer:  []*spec.Spec{child},
	}
	errs := (LegendTypeCoherent{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error from the layer child, got %+v", errs)
	}
	if got := errs[0].Context["Path"]; got != "layer[0]" {
		t.Errorf("Path = %v, want layer[0]", got)
	}
}

func TestLegendSymbolTypeAcceptsTheShapeVocabulary(t *testing.T) {
	for _, shape := range []string{"", "circle", "square", "triangle", "cross", "diamond"} {
		s := legendRuleSpec(legendChannel("nominal", &spec.Legend{SymbolType: shape}))
		if errs := (LegendSymbolType{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
			t.Errorf("symbol_type %q: expected no errors, got %+v", shape, errs)
		}
	}
}

func TestLegendSymbolTypeRejectsUnknownShape(t *testing.T) {
	s := legendRuleSpec(legendChannel("nominal", &spec.Legend{SymbolType: "hexagon"}))
	errs := (LegendSymbolType{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %+v", errs)
	}
	if errs[0].Code != "PRISM_SPEC_052" {
		t.Errorf("code = %q, want PRISM_SPEC_052", errs[0].Code)
	}
	if got := errs[0].Context["Allowed"]; got == "" || got == nil {
		t.Error("error details carry no allowed-shape list")
	}
}

// A channel with no legend block at all has nothing to configure, so
// neither rule may fire on it.
func TestLegendRulesIgnoreChannelsWithoutALegendBlock(t *testing.T) {
	s := legendRuleSpec(legendChannel("quantitative", nil))
	if errs := (LegendTypeCoherent{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Errorf("PRISM_SPEC_051 fired without a legend block: %+v", errs)
	}
	if errs := (LegendSymbolType{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Errorf("PRISM_SPEC_052 fired without a legend block: %+v", errs)
	}
}
