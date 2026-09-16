package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

func TestPrismCurveFor(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	cases := []struct {
		name        string
		mark        *spec.MarkDef
		wantCurve   scene.CurveType
		wantTension float64
	}{
		{name: "nil mark def", mark: nil, wantCurve: scene.CurveLinear},
		{name: "unset interpolate", mark: &spec.MarkDef{Type: "line"}, wantCurve: scene.CurveLinear},
		{name: "explicit linear", mark: &spec.MarkDef{Interpolate: "linear"}, wantCurve: scene.CurveLinear},
		{name: "monotone", mark: &spec.MarkDef{Interpolate: "monotone"}, wantCurve: scene.CurveMonotone},
		{name: "step", mark: &spec.MarkDef{Interpolate: "step"}, wantCurve: scene.CurveStep},
		{name: "step-before", mark: &spec.MarkDef{Interpolate: "step-before"}, wantCurve: scene.CurveStepBefore},
		{name: "step-after", mark: &spec.MarkDef{Interpolate: "step-after"}, wantCurve: scene.CurveStepAfter},
		{name: "cardinal", mark: &spec.MarkDef{Interpolate: "cardinal"}, wantCurve: scene.CurveCardinal},
		{
			name:        "cardinal with tension",
			mark:        &spec.MarkDef{Interpolate: "cardinal", Tension: f(0.75)},
			wantCurve:   scene.CurveCardinal,
			wantTension: 0.75,
		},
		{
			name:        "tension clamps high",
			mark:        &spec.MarkDef{Interpolate: "cardinal", Tension: f(4)},
			wantCurve:   scene.CurveCardinal,
			wantTension: 1,
		},
		{
			name:        "tension clamps low",
			mark:        &spec.MarkDef{Interpolate: "cardinal", Tension: f(-2)},
			wantCurve:   scene.CurveCardinal,
			wantTension: 0,
		},
		{
			// tension only parameterises cardinal — every other curve
			// reports 0 so the scene JSON stays free of the field.
			name:      "tension ignored off cardinal",
			mark:      &spec.MarkDef{Interpolate: "monotone", Tension: f(0.5)},
			wantCurve: scene.CurveMonotone,
		},
		{
			// out-of-vocabulary values are rejected by the schema; if
			// one reaches the encoder anyway it degrades to linear.
			name:      "out-of-scope basis degrades",
			mark:      &spec.MarkDef{Interpolate: "basis"},
			wantCurve: scene.CurveLinear,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			curve, tension := curveFor(Inputs{Mark: tc.mark})
			if curve != tc.wantCurve {
				t.Errorf("curve = %q, want %q", curve, tc.wantCurve)
			}
			if tension != tc.wantTension {
				t.Errorf("tension = %g, want %g", tension, tc.wantTension)
			}
		})
	}
}

// TestPrismEncodeLineCarriesCurve checks the line encoder threads the
// mark-level interpolate/tension onto every emitted LineGeom.
func TestPrismEncodeLineCarriesCurve(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"x":     []float64{0, 50, 100},
		"score": []float64{0.4, 0.6, 0.8},
	})
	plot := plotRect()
	tension := 0.25
	marks, _, err := Encode("line", Inputs{
		Table:  tbl,
		X:      Channel{Field: "x", Scale: &linScale{dmin: 0, dmax: 100, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: "score", Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}},
		Layout: plot,
		Mark:   &spec.MarkDef{Type: "line", Interpolate: "cardinal", Tension: &tension},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 1 || marks[0].Line == nil {
		t.Fatalf("expected one line mark, got %d", len(marks))
	}
	if marks[0].Line.Curve != scene.CurveCardinal {
		t.Errorf("Curve = %q, want cardinal", marks[0].Line.Curve)
	}
	if marks[0].Line.Tension != 0.25 {
		t.Errorf("Tension = %g, want 0.25", marks[0].Line.Tension)
	}
}

// TestPrismEncodeAreaCarriesCurve does the same for the area encoder,
// whose geom drives both the upper and the lower edge.
func TestPrismEncodeAreaCarriesCurve(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"x":     []float64{0, 50, 100},
		"score": []float64{0.4, 0.6, 0.8},
	})
	plot := plotRect()
	marks, _, err := Encode("area", Inputs{
		Table:  tbl,
		X:      Channel{Field: "x", Scale: &linScale{dmin: 0, dmax: 100, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: "score", Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}},
		Layout: plot,
		Mark:   &spec.MarkDef{Type: "area", Interpolate: "step-after"},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 1 || marks[0].Area == nil {
		t.Fatalf("expected one area mark, got %d", len(marks))
	}
	if marks[0].Area.Curve != scene.CurveStepAfter {
		t.Errorf("Curve = %q, want step-after", marks[0].Area.Curve)
	}
}
