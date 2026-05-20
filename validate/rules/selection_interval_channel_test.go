package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestPrismSelectionIntervalChannelAcceptsPositionChannels(t *testing.T) {
	cases := [][]string{
		{"x"}, {"y"}, {"x", "y"}, {"theta"}, {"x2", "y2"},
	}
	for _, encs := range cases {
		s := &spec.Spec{
			Schema: "urn:prism:schema:v1:spec",
			Selection: map[string]spec.Selection{
				"b": {Interval: &spec.IntervalSelection{Type: "interval", Encodings: encs}},
			},
		}
		if errs := (SelectionIntervalChannel{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
			t.Errorf("encs %v: expected no errors, got: %+v", encs, errs)
		}
	}
}

func TestPrismSelectionIntervalChannelRejectsColor(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"b": {Interval: &spec.IntervalSelection{Type: "interval", Encodings: []string{"color"}}},
		},
	}
	errs := SelectionIntervalChannel{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_020" {
		t.Fatalf("expected one PRISM_SPEC_020, got: %+v", errs)
	}
	if got := errs[0].Context["Channel"]; got != "color" {
		t.Errorf("Channel = %v, want color", got)
	}
}

func TestPrismSelectionIntervalChannelIgnoresPointSelection(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"p": {Point: &spec.PointSelection{Type: "point", Encodings: []string{"color"}}},
		},
	}
	if errs := (SelectionIntervalChannel{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Errorf("point selections should be ignored, got: %+v", errs)
	}
}

func TestPrismSelectionIntervalChannelMultipleNonPositionEntries(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Selection: map[string]spec.Selection{
			"b": {Interval: &spec.IntervalSelection{Type: "interval", Encodings: []string{"color", "shape", "x"}}},
		},
	}
	errs := SelectionIntervalChannel{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors (color + shape; x is allowed), got: %+v", errs)
	}
	for _, e := range errs {
		if e.Code != "PRISM_SPEC_020" {
			t.Errorf("unexpected code %s", e.Code)
		}
	}
}
