package marks

import (
	"math"
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// horizontalInputs builds the canonical horizontal-bar fixture: a band
// scale on y (inverted, bottom-to-top, so its step is negative) and a
// linear scale on x spanning a domain that straddles zero.
func horizontalInputs(t *testing.T, def *spec.MarkDef) Inputs {
	t.Helper()
	tbl := buildTable(t, map[string]any{
		"channel": []string{"paid", "organic", "display"},
		"delta":   []float64{0.4, 0.8, -0.2},
	})
	plot := plotRect()
	return Inputs{
		Table: tbl,
		// y band runs plot.Bottom() → plot.Y, so step (and BandWidth)
		// is negative — the exact shape that used to yield a negative
		// rect height.
		X:      Channel{Field: "delta", Scale: &linScale{dmin: -1, dmax: 1, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: "channel", Scale: &bandScaleT{cats: []string{"paid", "organic", "display"}, rmin: plot.Bottom(), rmax: plot.Y, padding: 0.1}},
		Layout: plot,
		Style:  scene.Style{},
		Mark:   def,
	}
}

func TestPrismMarkOrientationInferred(t *testing.T) {
	plot := plotRect()
	band := &bandScaleT{cats: []string{"a", "b"}, rmin: plot.X, rmax: plot.Right()}
	yBand := &bandScaleT{cats: []string{"a", "b"}, rmin: plot.Bottom(), rmax: plot.Y}
	lin := &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}
	xLin := &linScale{dmin: 0, dmax: 1, rmin: plot.X, rmax: plot.Right()}

	cases := []struct {
		name string
		x, y Scale
		want Orientation
	}{
		{"band x, continuous y", band, lin, OrientVertical},
		{"continuous x, band y", xLin, yBand, OrientHorizontal},
		{"band on both axes falls back to vertical", band, yBand, OrientVertical},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MarkOrientation(Inputs{X: Channel{Scale: tc.x}, Y: Channel{Scale: tc.y}}, "bar")
			if err != nil {
				t.Fatalf("MarkOrientation: %v", err)
			}
			if got != tc.want {
				t.Errorf("orientation = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPrismMarkOrientationNeitherAxisBanded(t *testing.T) {
	plot := plotRect()
	in := Inputs{
		X: Channel{Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.X, rmax: plot.Right()}},
		Y: Channel{Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}},
	}
	_, err := MarkOrientation(in, "bar")
	if err == nil {
		t.Fatal("expected an error when neither axis carries a band scale")
	}
	// The pre-E9-S1 message blamed x alone; it must now name both.
	if !strings.Contains(err.Error(), "x (vertical) or on y (horizontal)") {
		t.Errorf("error should name both axes, got: %v", err)
	}
}

func TestPrismMarkOrientationExplicitOverrides(t *testing.T) {
	plot := plotRect()
	// Band on BOTH axes, which infers vertical; an explicit
	// "horizontal" must win.
	in := Inputs{
		X:    Channel{Scale: &bandScaleT{cats: []string{"a"}, rmin: plot.X, rmax: plot.Right()}},
		Y:    Channel{Scale: &bandScaleT{cats: []string{"a"}, rmin: plot.Bottom(), rmax: plot.Y}},
		Mark: &spec.MarkDef{Type: "bar", Orient: "horizontal"},
	}
	got, err := MarkOrientation(in, "bar")
	if err != nil {
		t.Fatalf("MarkOrientation: %v", err)
	}
	if got != OrientHorizontal {
		t.Errorf("orientation = %q, want %q", got, OrientHorizontal)
	}
}

func TestPrismMarkOrientationExplicitUndrawable(t *testing.T) {
	plot := plotRect()
	in := Inputs{
		X:    Channel{Scale: &bandScaleT{cats: []string{"a"}, rmin: plot.X, rmax: plot.Right()}},
		Y:    Channel{Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}},
		Mark: &spec.MarkDef{Type: "bar", Orient: "horizontal"},
	}
	_, err := MarkOrientation(in, "bar")
	if err == nil {
		t.Fatal("expected an error: horizontal needs a band scale on y")
	}
	if !strings.Contains(err.Error(), "band scale on y") {
		t.Errorf("error should name the axis needing the band, got: %v", err)
	}
}

func TestPrismMarkOrientationRadialRejected(t *testing.T) {
	plot := plotRect()
	in := Inputs{
		X:    Channel{Scale: &bandScaleT{cats: []string{"a"}, rmin: plot.X, rmax: plot.Right()}},
		Y:    Channel{Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}},
		Mark: &spec.MarkDef{Type: "bar", Orient: "radial"},
	}
	_, err := MarkOrientation(in, "bar")
	if err == nil {
		t.Fatal("orient radial must be rejected, never silently ignored")
	}
	if !strings.Contains(err.Error(), "radial") {
		t.Errorf("error should mention radial, got: %v", err)
	}
	// An unknown value is rejected too, rather than falling back.
	in.Mark = &spec.MarkDef{Type: "bar", Orient: "sideways"}
	if _, err := MarkOrientation(in, "bar"); err == nil {
		t.Fatal("an unknown orient must be rejected")
	}
}

func TestPrismEncodeBarHorizontal(t *testing.T) {
	in := horizontalInputs(t, nil) // orientation inferred from the y band
	marks, _, err := Encode("bar", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 3 {
		t.Fatalf("len(marks) = %d, want 3", len(marks))
	}

	yBand := in.Y.Scale.(BandScaler)
	wantH := math.Abs(yBand.BandWidth())
	baseline := BaselinePixel(in, OrientHorizontal)

	for i, m := range marks {
		if m.Rect == nil {
			t.Fatalf("marks[%d] has no rect", i)
		}
		if m.Rect.W < 0 || m.Rect.H < 0 {
			t.Errorf("marks[%d] is undrawable: W=%g H=%g", i, m.Rect.W, m.Rect.H)
		}
		// Thickness comes from the (negative-step) y band, normalised.
		if math.Abs(m.Rect.H-wantH) > 1e-9 {
			t.Errorf("marks[%d].H = %g, want the band width %g", i, m.Rect.H, wantH)
		}
	}
	// Positive values start at the baseline and run right.
	if math.Abs(marks[0].Rect.X-baseline) > 1e-9 {
		t.Errorf("positive bar starts at %g, want the baseline %g", marks[0].Rect.X, baseline)
	}
	// delta 0.8 is longer than delta 0.4.
	if marks[1].Rect.W <= marks[0].Rect.W {
		t.Errorf("bar1.W=%g should exceed bar0.W=%g", marks[1].Rect.W, marks[0].Rect.W)
	}
	// The negative value crosses the baseline leftward: it ends there.
	neg := marks[2].Rect
	if math.Abs(neg.X+neg.W-baseline) > 1e-9 {
		t.Errorf("negative bar ends at %g, want the baseline %g", neg.X+neg.W, baseline)
	}
}

func TestPrismEncodeBarHorizontalColorGrouping(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"channel": []string{"paid", "organic", "display"},
		"delta":   []float64{0.4, 0.8, -0.2},
		"region":  []string{"emea", "amer", "emea"},
	})
	plot := plotRect()
	red := &scene.Color{R: 255, A: 255}
	blue := &scene.Color{B: 255, A: 255}
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "delta", Scale: &linScale{dmin: -1, dmax: 1, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: "channel", Scale: &bandScaleT{cats: []string{"paid", "organic", "display"}, rmin: plot.Bottom(), rmax: plot.Y, padding: 0.1}},
		Color:  &ColorChannel{Field: "region", Categories: []string{"emea", "amer"}, Palette: []*scene.Color{red, blue}},
		Layout: plot,
	}
	marks, _, err := Encode("bar", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 3 {
		t.Fatalf("len(marks) = %d, want 3", len(marks))
	}
	want := []*scene.Color{red, blue, red}
	for i, m := range marks {
		if m.Style.Fill == nil || *m.Style.Fill != *want[i] {
			t.Errorf("marks[%d].Fill = %v, want %v", i, m.Style.Fill, *want[i])
		}
	}
}

func TestPrismEncodeBarHorizontalCornerRadius(t *testing.T) {
	r := 4.0
	in := horizontalInputs(t, &spec.MarkDef{Type: "bar", Orient: "horizontal", CornerRadius: &r})
	marks, _, err := Encode("bar", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for i, m := range marks {
		if m.Rect.CornerR != r {
			t.Errorf("marks[%d].CornerR = %g, want %g", i, m.Rect.CornerR, r)
		}
	}
}

// TestPrismEncodeRectBandYPositiveHeight pins the bug E9-S3 handed to
// E9-S1: the non-span rect path read the y band's BandWidth() straight
// into RectGeom.H, and a y band scale's step is negative, so the rect
// was undrawable. No golden covered it. Both the one-band (horizontal
// bar variant) and the two-band (heatmap-lite cell) shapes are checked.
func TestPrismEncodeRectBandYPositiveHeight(t *testing.T) {
	plot := plotRect()
	yBand := &bandScaleT{cats: []string{"r1", "r2"}, rmin: plot.Bottom(), rmax: plot.Y, padding: 0.1}
	if yBand.BandWidth() >= 0 {
		t.Fatal("fixture precondition: the y band step must be negative")
	}

	t.Run("band y, continuous x", func(t *testing.T) {
		tbl := buildTable(t, map[string]any{
			"row":   []string{"r1", "r2"},
			"value": []float64{0.3, 0.9},
		})
		marks, _, err := Encode("rect", Inputs{
			Table:  tbl,
			X:      Channel{Field: "value", Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.X, rmax: plot.Right()}},
			Y:      Channel{Field: "row", Scale: yBand},
			Layout: plot,
		})
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		for i, m := range marks {
			if m.Rect.H <= 0 || m.Rect.W < 0 {
				t.Errorf("marks[%d] undrawable: W=%g H=%g", i, m.Rect.W, m.Rect.H)
			}
		}
	})

	t.Run("band on both axes", func(t *testing.T) {
		tbl := buildTable(t, map[string]any{
			"col": []string{"c1", "c2"},
			"row": []string{"r1", "r2"},
		})
		marks, _, err := Encode("rect", Inputs{
			Table:  tbl,
			X:      Channel{Field: "col", Scale: &bandScaleT{cats: []string{"c1", "c2"}, rmin: plot.X, rmax: plot.Right(), padding: 0.1}},
			Y:      Channel{Field: "row", Scale: yBand},
			Layout: plot,
		})
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		for i, m := range marks {
			if m.Rect.H <= 0 || m.Rect.W <= 0 {
				t.Errorf("marks[%d] undrawable: W=%g H=%g", i, m.Rect.W, m.Rect.H)
			}
			// The cell must start at the band's TOP edge, not its
			// bottom one — the normalisation, not just abs().
			if m.Rect.Y < plot.Y-1e-9 || m.Rect.Y+m.Rect.H > plot.Bottom()+1e-9 {
				t.Errorf("marks[%d] cell [%g, %g] escapes the plot region", i, m.Rect.Y, m.Rect.Y+m.Rect.H)
			}
		}
	})
}
