package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestPrismCompositeHistogramRequiresX(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "histogram"},
		Encoding: &spec.Encoding{
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}},
		},
	}
	errs := CompositeMarkEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_013" {
		t.Fatalf("expected one PRISM_SPEC_013, got: %+v", errs)
	}
}

func TestPrismCompositeHistogramRejectsCategoricalX(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "histogram"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "category", Type: "nominal"}},
		},
	}
	errs := CompositeMarkEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_013" {
		t.Fatalf("expected one PRISM_SPEC_013, got: %+v", errs)
	}
}

func TestPrismCompositeHistogramAcceptsQuantitativeX(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "histogram"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Aggregate: "count", Type: "quantitative"}},
		},
	}
	errs := CompositeMarkEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismCompositeHeatmapRequiresXAndY(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "heatmap"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "region", Type: "nominal"}},
		},
	}
	errs := CompositeMarkEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_013" {
		t.Fatalf("expected one PRISM_SPEC_013, got: %+v", errs)
	}
}

func TestPrismCompositeHeatmapAcceptsBoth(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "heatmap"},
		Encoding: &spec.Encoding{
			X:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "region", Type: "nominal"}},
			Y:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "bucket", Type: "ordinal"}},
			Color: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "count", Type: "quantitative"}},
		},
	}
	errs := CompositeMarkEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismCompositeBoxplotRequiresOneCategoryAxis(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "boxplot"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "a", Type: "quantitative"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "b", Type: "quantitative"}},
		},
	}
	errs := CompositeMarkEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_013" {
		t.Fatalf("expected one PRISM_SPEC_013, got: %+v", errs)
	}
}

func TestPrismCompositeBoxplotAcceptsCategoryXQuantitativeY(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "boxplot"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "group", Type: "nominal"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}},
		},
	}
	errs := CompositeMarkEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismCompositeViolinAcceptsCategoryXQuantitativeY(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "violin"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "group", Type: "nominal"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}},
		},
	}
	errs := CompositeMarkEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismCompositeNonCompositeMarkIgnored(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "x", Type: "nominal"}},
		},
	}
	errs := CompositeMarkEncoding{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors for non-composite mark, got: %+v", errs)
	}
}
