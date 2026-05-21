package rules

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestPrismAnimationKeyUniquePassesOnSingleKey(t *testing.T) {
	s := &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Animation: &spec.Animation{},
		Mark:      &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "region", Type: "nominal", Key: true}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}},
		},
	}
	errs := AnimationKeyUnique{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismAnimationKeyUniqueRejectsMultiple(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Encoding: &spec.Encoding{
			X:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "a", Type: "nominal", Key: true}},
			Y:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "b", Type: "quantitative"}},
			Color: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "c", Type: "nominal", Key: true}},
		},
	}
	errs := AnimationKeyUnique{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_024" {
		t.Fatalf("expected one PRISM_SPEC_024 error, got: %+v", errs)
	}
	got := errs[0].Message
	if !strings.Contains(got, "color") || !strings.Contains(got, "x") {
		t.Errorf("expected message to list offending channels (got %q)", got)
	}
}

func TestPrismAnimationKeyUniqueFiresPerLayer(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Layer: []*spec.Spec{
			{Encoding: &spec.Encoding{
				X:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "a", Type: "nominal", Key: true}},
				Color: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "b", Type: "nominal", Key: true}},
			}},
			{Encoding: &spec.Encoding{
				X:    &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "c", Type: "nominal", Key: true}},
				Size: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "d", Type: "quantitative", Key: true}},
			}},
		},
	}
	errs := AnimationKeyUnique{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 2 {
		t.Fatalf("expected two PRISM_SPEC_024 errors (one per layer), got %d: %+v", len(errs), errs)
	}
}
