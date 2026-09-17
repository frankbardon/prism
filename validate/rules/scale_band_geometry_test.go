package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
)

// bandGeometrySpec builds a minimal bar spec whose x channel carries
// the given scale block.
func bandGeometrySpec(x *spec.Scale) *spec.Spec {
	return &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "k", Type: "nominal", Scale: x},
			},
			Y: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "v", Type: "quantitative"},
			},
		},
	}
}

func f64(v float64) *float64 { return &v }

func TestPrismScaleBandGeometryInRange(t *testing.T) {
	sc := &spec.Scale{
		Padding:      f64(0),
		PaddingInner: f64(0.999),
		PaddingOuter: f64(0.5),
		Align:        f64(1),
	}
	if errs := (ScaleBandGeometry{}).Check(bandGeometrySpec(sc), nil); len(errs) != 0 {
		t.Fatalf("legal geometry must pass, got %d errors: %v", len(errs), errs)
	}
}

func TestPrismScaleBandGeometryRejectsOutOfRange(t *testing.T) {
	cases := []struct {
		name     string
		scale    *spec.Scale
		property string
	}{
		{"padding at 1", &spec.Scale{Padding: f64(1)}, "padding"},
		{"inner above 1", &spec.Scale{PaddingInner: f64(1.5)}, "padding_inner"},
		{"outer negative", &spec.Scale{PaddingOuter: f64(-0.1)}, "padding_outer"},
		{"align above 1", &spec.Scale{Align: f64(1.5)}, "align"},
		{"align negative", &spec.Scale{Align: f64(-1)}, "align"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := (ScaleBandGeometry{}).Check(bandGeometrySpec(tc.scale), nil)
			if len(errs) != 1 {
				t.Fatalf("want 1 error, got %d: %v", len(errs), errs)
			}
			if errs[0].Code != "PRISM_SPEC_049" {
				t.Fatalf("want PRISM_SPEC_049, got %s", errs[0].Code)
			}
			if got := errs[0].Context["Property"]; got != tc.property {
				t.Fatalf("want Property=%s, got %v", tc.property, got)
			}
			if got := errs[0].Context["Channel"]; got != "x" {
				t.Fatalf("want Channel=x, got %v", got)
			}
		})
	}
}

func TestPrismScaleBandGeometryAbsentPasses(t *testing.T) {
	if errs := (ScaleBandGeometry{}).Check(bandGeometrySpec(&spec.Scale{Zero: new(bool)}), nil); len(errs) != 0 {
		t.Fatalf("a scale block without band geometry must pass, got %v", errs)
	}
	if errs := (ScaleBandGeometry{}).Check(bandGeometrySpec(nil), nil); len(errs) != 0 {
		t.Fatalf("no scale block at all must pass, got %v", errs)
	}
}

func TestPrismScaleBandGeometryWalksLayers(t *testing.T) {
	child := bandGeometrySpec(&spec.Scale{PaddingInner: f64(2)})
	parent := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Layer:  []*spec.Spec{child},
	}
	errs := (ScaleBandGeometry{}).Check(parent, nil)
	if len(errs) != 1 {
		t.Fatalf("want 1 error from the layer child, got %d: %v", len(errs), errs)
	}
}
