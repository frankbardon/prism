package scene

import "testing"

// TestEstimateTextWidthCoversChrome pins the estimate against widths
// Chrome measured for the default sans stack (system-ui, which
// resolves to the same face as Inter's fallback on macOS) at 11 px.
// The estimate must never read narrower than the real string — a
// narrower one is what clipped long category labels at the frame edge
// — and should not overshoot by more than a quarter.
func TestEstimateTextWidthCoversChrome(t *testing.T) {
	measured := map[string]float64{
		"control":             36.8,
		"variant":             35.7,
		"z (std. residual)":   82.1,
		"income":              38.0,
		"age":                 19.3,
		"1000":                26.1,
		"0.5":                 16.4,
		"-0.5":                21.7,
		"Observed":            50.8,
		"WWWWWWWW":            85.7,
		"mmmmmmmm":            77.1,
		"iiiiiiii":            22.3,
		"Mean Absolute Error": 107.2,
		"ABCDEFGHIJ":          69.2,
		"Category":            48.1,
		"8888888":             49.6,
	}
	for s, want := range measured {
		got := EstimateTextWidth(s, 11)
		if got < want {
			t.Errorf("%q: estimate %.1f under Chrome's %.1f", s, got, want)
		}
		if got > want*1.25 {
			t.Errorf("%q: estimate %.1f overshoots Chrome's %.1f by more than 25%%", s, got, want)
		}
	}
}

func TestAxisLabelBand(t *testing.T) {
	a := Axis{Ticks: []Tick{
		{Label: "age"},
		{Label: "z (std. residual)"},
		{Label: "WWWWWWWWWWWWWWWWWWWW", LabelHidden: true},
		{Label: "WWWWWWWWWWWWWWWWWWWW", Minor: true},
	}}
	if got, want := AxisLabelBand(a), EstimateTextWidth("z (std. residual)", AxisLabelFontSize); got != want {
		t.Errorf("band = %v, want the widest shown label's %v", got, want)
	}
	a.HideLabels = true
	if got := AxisLabelBand(a); got != 0 {
		t.Errorf("hidden labels: band = %v, want 0", got)
	}
	vertical := Axis{LabelAngle: 90, Ticks: []Tick{{Label: "a very long label"}}}
	if got := AxisLabelBand(vertical); got < LabelLineHeight-1e-9 || got > LabelLineHeight+1e-9 {
		t.Errorf("label rotated 90: band = %v, want one line height", got)
	}
}
