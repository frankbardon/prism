package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func TestScaleTypeCompatAcceptsLogOnQuantitative(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Y: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative",
					Scale: &spec.Scale{Type: "log"}},
			},
		},
	}
	errs := ScaleTypeCompat{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

// TestScaleTypeCompatMatrix exhaustively asserts the 8 scale types
// × 4 measure types compatibility table (T06.17).
func TestScaleTypeCompatMatrix(t *testing.T) {
	cases := []struct {
		scaleType   string
		measureType string
		want        bool
	}{
		{"linear", "quantitative", true},
		{"log", "quantitative", true},
		{"pow", "quantitative", true},
		{"sqrt", "quantitative", true},
		{"time", "quantitative", false},
		{"band", "quantitative", false},
		{"point", "quantitative", false},
		{"ordinal", "quantitative", false},

		{"linear", "temporal", true},
		{"log", "temporal", false},
		{"pow", "temporal", false},
		{"sqrt", "temporal", false},
		{"time", "temporal", true},
		{"band", "temporal", false},
		{"point", "temporal", false},
		{"ordinal", "temporal", false},

		{"linear", "nominal", false},
		{"log", "nominal", false},
		{"pow", "nominal", false},
		{"sqrt", "nominal", false},
		{"time", "nominal", false},
		{"band", "nominal", true},
		{"point", "nominal", true},
		{"ordinal", "nominal", true},

		{"linear", "ordinal", false},
		{"log", "ordinal", false},
		{"pow", "ordinal", false},
		{"sqrt", "ordinal", false},
		{"time", "ordinal", false},
		{"band", "ordinal", true},
		{"point", "ordinal", true},
		{"ordinal", "ordinal", true},
	}
	for _, tc := range cases {
		tc := tc
		name := tc.scaleType + "-on-" + tc.measureType
		t.Run(name, func(t *testing.T) {
			got := scaleCompatible(tc.scaleType, tc.measureType)
			if got != tc.want {
				t.Errorf("scaleCompatible(%q, %q) = %v, want %v",
					tc.scaleType, tc.measureType, got, tc.want)
			}
		})
	}
}

func TestScaleTypeCompatRejectsLogOnNominal(t *testing.T) {
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{
				ChannelCommon: spec.ChannelCommon{Field: "brand_id", Type: "nominal",
					Scale: &spec.Scale{Type: "log"}},
			},
		},
	}
	errs := ScaleTypeCompat{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_007" {
		t.Fatalf("expected one PRISM_SPEC_007, got: %+v", errs)
	}
}
