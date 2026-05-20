package rules

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestPrismSankeyAcceptsThreeChannels(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "sankey"},
		Encoding: &spec.Encoding{
			Source: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "src", Type: "nominal"}},
			Target: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "tgt", Type: "nominal"}},
			Value:  &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "v", Type: "quantitative"}},
		},
	}
	errs := SankeyChannels{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismSankeyFiresOnMissingValue(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "sankey"},
		Encoding: &spec.Encoding{
			Source: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "src", Type: "nominal"}},
			Target: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "tgt", Type: "nominal"}},
		},
	}
	errs := SankeyChannels{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_018" {
		t.Fatalf("expected PRISM_SPEC_018, got: %+v", errs)
	}
	if !strings.Contains(errs[0].Message, "value") {
		t.Errorf("missing list should include 'value': %s", errs[0].Message)
	}
}

func TestPrismSankeyFiresOnAllMissing(t *testing.T) {
	s := &spec.Spec{
		Schema:   "urn:prism:schema:v1:spec",
		Mark:     &spec.Mark{Shorthand: "sankey"},
		Encoding: &spec.Encoding{},
	}
	errs := SankeyChannels{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_018" {
		t.Fatalf("expected PRISM_SPEC_018, got: %+v", errs)
	}
	for _, want := range []string{"source", "target", "value"} {
		if !strings.Contains(errs[0].Message, want) {
			t.Errorf("message must include %q: %s", want, errs[0].Message)
		}
	}
}

func TestPrismSankeyIgnoresNonSankey(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
	}
	errs := SankeyChannels{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Errorf("non-sankey mark should be ignored, got: %+v", errs)
	}
}
