package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestPrismAnimationKeyPresentPassesOnKeyChannel(t *testing.T) {
	s := &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Animation: &spec.Animation{},
		Mark:      &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "region", Type: "nominal", Key: true}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}},
		},
	}
	errs := AnimationKeyPresent{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismAnimationKeyPresentRejectsMissingKey(t *testing.T) {
	s := &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Animation: &spec.Animation{},
		Mark:      &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "region", Type: "nominal"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}},
		},
	}
	errs := AnimationKeyPresent{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_023" {
		t.Fatalf("expected one PRISM_SPEC_023, got: %+v", errs)
	}
}

func TestPrismAnimationKeyPresentIgnoresAbsentBlock(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
	}
	errs := AnimationKeyPresent{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Errorf("no-animation spec expected no errors, got: %+v", errs)
	}
}

func TestPrismAnimationKeyPresentLooksIntoLayers(t *testing.T) {
	s := &spec.Spec{
		Schema:    "urn:prism:schema:v1:spec",
		Animation: &spec.Animation{},
		Layer: []*spec.Spec{
			{
				Mark: &spec.Mark{Shorthand: "bar"},
				Encoding: &spec.Encoding{
					X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "region", Type: "nominal", Key: true}},
				},
			},
		},
	}
	errs := AnimationKeyPresent{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("layered key channel expected to satisfy rule, got: %+v", errs)
	}
}
