package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestPrismSelectionEncodingChannelAcceptsBound(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"hi": {Point: &spec.PointSelection{Type: "point", Encodings: []string{"color"}}},
		},
		Encoding: &spec.Encoding{
			X:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "a", Type: "nominal"}},
			Y:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "b", Type: "quantitative"}},
			Color: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "c", Type: "nominal"}},
		},
	}
	if errs := (SelectionEncodingChannel{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismSelectionEncodingChannelRejectsUnbound(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"hi": {Point: &spec.PointSelection{Type: "point", Encodings: []string{"color"}}},
		},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "a", Type: "nominal"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "b", Type: "quantitative"}},
		},
	}
	errs := SelectionEncodingChannel{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_019" {
		t.Fatalf("expected one PRISM_SPEC_019, got: %+v", errs)
	}
	if got := errs[0].Context["Channel"]; got != "color" {
		t.Errorf("Channel = %v, want color", got)
	}
}

func TestPrismSelectionEncodingChannelIntervalForm(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"brush": {Interval: &spec.IntervalSelection{Type: "interval", Encodings: []string{"opacity"}}},
		},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "a", Type: "nominal"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "b", Type: "quantitative"}},
		},
	}
	errs := SelectionEncodingChannel{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_019" {
		t.Fatalf("expected one PRISM_SPEC_019, got: %+v", errs)
	}
}

func TestPrismSelectionEncodingChannelEmptyEncodingsList(t *testing.T) {
	// Selection without an Encodings list is fine — the rule only
	// fires on listed channels that don't resolve.
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"hi": {Point: &spec.PointSelection{Type: "point"}},
		},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "a", Type: "nominal"}},
		},
	}
	if errs := (SelectionEncodingChannel{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Errorf("expected no errors, got: %+v", errs)
	}
}

func TestPrismSelectionEncodingChannelNoSelectionsNoErrors(t *testing.T) {
	s := &spec.Spec{Schema: "urn:prism:schema:v1:spec"}
	if errs := (SelectionEncodingChannel{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Errorf("expected no errors on spec without selections, got: %+v", errs)
	}
}
