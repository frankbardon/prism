package svg

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// TestPrismWriteStyleAttrsStrokeDash covers the E7-S4 emitter:
// render/svg emitted no stroke-dasharray at all before this, which is
// what made both spec mark_def.stroke_dash and the theme mark block's
// stroke_dash token dead ends.
func TestPrismWriteStyleAttrsStrokeDash(t *testing.T) {
	cases := []struct {
		name string
		dash []float64
		want string
	}{
		{"nil is unset", nil, ""},
		{"empty is unset", []float64{}, ""},
		{"all zero is unset", []float64{0, 0}, ""},
		{"pair", []float64{4, 2}, ` stroke-dasharray="4 2"`},
		{"triple", []float64{6, 3, 1}, ` stroke-dasharray="6 3 1"`},
		{"fractional pins to 3 decimals", []float64{1.23456, 2}, ` stroke-dasharray="1.235 2"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWriter()
			writeStyleAttrs(w, scene.Style{StrokeDash: tc.dash})
			if got := w.String(); got != tc.want {
				t.Errorf("writeStyleAttrs = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPrismStrokeDashOrdersAfterStrokeWidth keeps the attribute order
// deterministic — the goldens are byte-compared, so a reshuffle here
// would move every dashed mark's committed bytes.
func TestPrismStrokeDashOrdersAfterStrokeWidth(t *testing.T) {
	w := NewWriter()
	writeStyleAttrs(w, scene.Style{StrokeWidth: 2, StrokeDash: []float64{4, 2}, Opacity: 0.5})
	got := w.String()
	want := ` stroke-width="2" stroke-dasharray="4 2" opacity="0.5"`
	if got != want {
		t.Fatalf("writeStyleAttrs = %q, want %q", got, want)
	}
	if !strings.Contains(got, `stroke-dasharray`) {
		t.Fatal("stroke-dasharray missing")
	}
}
