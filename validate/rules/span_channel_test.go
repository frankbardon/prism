package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func pos(field, ty string) *spec.PositionChannel {
	return &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: field, Type: ty}}
}

func spanSpec(mark string, enc *spec.Encoding) *spec.Spec {
	return &spec.Spec{
		Schema:   "urn:prism:schema:v1:spec",
		Mark:     &spec.Mark{Shorthand: mark},
		Encoding: enc,
	}
}

func TestSpanChannelSupportedAcceptsRangedBar(t *testing.T) {
	s := spanSpec("bar", &spec.Encoding{
		Y:  pos("task", "nominal"),
		X:  pos("start", "quantitative"),
		X2: pos("end", "quantitative"),
	})
	if errs := (SpanChannelSupported{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestSpanChannelSupportedAcceptsAreaY2(t *testing.T) {
	s := spanSpec("area", &spec.Encoding{
		X:  pos("t", "quantitative"),
		Y:  pos("hi", "quantitative"),
		Y2: pos("lo", "quantitative"),
	})
	if errs := (SpanChannelSupported{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestSpanChannelSupportedRejectsAreaX2(t *testing.T) {
	s := spanSpec("area", &spec.Encoding{
		X:  pos("t", "quantitative"),
		X2: pos("t_end", "quantitative"),
		Y:  pos("hi", "quantitative"),
	})
	errs := (SpanChannelSupported{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_042" {
		t.Fatalf("expected one PRISM_SPEC_042, got: %+v", errs)
	}
}

func TestSpanChannelSupportedRejectsNonSpanMarks(t *testing.T) {
	for _, mark := range []string{"line", "point", "tick", "image", "heatmap", "histogram", "boxplot", "violin"} {
		s := spanSpec(mark, &spec.Encoding{
			X:  pos("start", "quantitative"),
			X2: pos("end", "quantitative"),
			Y:  pos("v", "quantitative"),
		})
		errs := (SpanChannelSupported{}).Check(s, validate.EmptyLookup{})
		if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_042" {
			t.Errorf("mark %q: expected one PRISM_SPEC_042, got: %+v", mark, errs)
		}
	}
}

func TestSpanChannelSupportedRejectsUnpairedSpan(t *testing.T) {
	s := spanSpec("bar", &spec.Encoding{
		Y:  pos("task", "nominal"),
		X2: pos("end", "quantitative"),
	})
	errs := (SpanChannelSupported{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_042" {
		t.Fatalf("expected one PRISM_SPEC_042, got: %+v", errs)
	}
}

func TestSpanChannelSupportedRejectsFieldlessSpan(t *testing.T) {
	s := spanSpec("bar", &spec.Encoding{
		Y:  pos("task", "nominal"),
		X:  pos("start", "quantitative"),
		X2: pos("", "quantitative"),
	})
	errs := (SpanChannelSupported{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_042" {
		t.Fatalf("expected one PRISM_SPEC_042, got: %+v", errs)
	}
}

func TestSpanChannelSupportedAllowsRepeatRef(t *testing.T) {
	ch := pos("", "quantitative")
	ch.FieldRef = &spec.RepeatRef{Axis: "column"}
	s := spanSpec("bar", &spec.Encoding{
		Y:  pos("task", "nominal"),
		X:  pos("start", "quantitative"),
		X2: ch,
	})
	if errs := (SpanChannelSupported{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors for an unsubstituted repeat ref, got: %+v", errs)
	}
}

func TestSpanChannelSupportedWalksLayers(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Layer: []*spec.Spec{
			spanSpec("bar", &spec.Encoding{
				Y:  pos("task", "nominal"),
				X:  pos("start", "quantitative"),
				X2: pos("end", "quantitative"),
			}),
			spanSpec("line", &spec.Encoding{
				X:  pos("start", "quantitative"),
				X2: pos("end", "quantitative"),
				Y:  pos("v", "quantitative"),
			}),
		},
	}
	errs := (SpanChannelSupported{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_042" {
		t.Fatalf("expected one PRISM_SPEC_042 from layer[1], got: %+v", errs)
	}
	if got := errs[0].Context["Path"]; got != "layer[1]" {
		t.Errorf("Path = %v, want layer[1]", got)
	}
}

func TestSpanChannelSupportedSilentWithoutSpanChannels(t *testing.T) {
	s := spanSpec("bar", &spec.Encoding{
		X: pos("region", "nominal"),
		Y: pos("total", "quantitative"),
	})
	if errs := (SpanChannelSupported{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestSpanChannelTypeMatchRejectsMismatch(t *testing.T) {
	s := spanSpec("bar", &spec.Encoding{
		Y:  pos("task", "nominal"),
		X:  pos("start", "quantitative"),
		X2: pos("end", "temporal"),
	})
	errs := (SpanChannelTypeMatch{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_043" {
		t.Fatalf("expected one PRISM_SPEC_043, got: %+v", errs)
	}
}

func TestSpanChannelTypeMatchAcceptsMatch(t *testing.T) {
	s := spanSpec("bar", &spec.Encoding{
		Y:  pos("task", "nominal"),
		X:  pos("start", "temporal"),
		X2: pos("end", "temporal"),
	})
	if errs := (SpanChannelTypeMatch{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestSpanChannelTypeMatchSkipsMissingType(t *testing.T) {
	s := spanSpec("bar", &spec.Encoding{
		Y:  pos("task", "nominal"),
		X:  pos("start", "quantitative"),
		X2: pos("end", ""),
	})
	if errs := (SpanChannelTypeMatch{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors when a type is absent, got: %+v", errs)
	}
}
