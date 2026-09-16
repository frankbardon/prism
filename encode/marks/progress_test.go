package marks

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// progressInputs builds the canonical metric-row shape: a nominal y
// band against a quantitative x, which MarkOrientation infers as
// horizontal without any orient declared.
func progressInputs(t *testing.T) Inputs {
	t.Helper()
	tbl := buildTable(t, map[string]any{
		"metric": []string{"Familiarity", "Consideration", "Preference"},
		"score":  []float64{90, 60, 30},
		"cap":    []float64{100, 120, 60},
	})
	plot := plotRect()
	return Inputs{
		Table: tbl,
		X:     Channel{Field: "score", Scale: &linScale{dmin: 0, dmax: 100, rmin: plot.X, rmax: plot.Right()}},
		Y: Channel{Field: "metric", Scale: &bandScaleT{
			cats: []string{"Familiarity", "Consideration", "Preference"},
			rmin: plot.Bottom(), rmax: plot.Y,
		}},
		Layout: plot,
	}
}

func TestPrismEncodeProgressTrackAndBarPerRow(t *testing.T) {
	in := progressInputs(t)
	in.TrackStyle = scene.Style{Fill: &scene.Color{R: 1, G: 2, B: 3, A: 255}}
	in.Mark = &spec.MarkDef{Type: "progress", Total: float64(100)}

	marks, _, err := Encode("progress", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 6 {
		t.Fatalf("len(marks) = %d, want 6 (3 tracks + 3 bars)", len(marks))
	}
	// Tracks come first so they paint behind, and carry their own Style.
	for i := 0; i < 3; i++ {
		if got, want := marks[i].ID, "progress-track-"+string(rune('0'+i)); got != want {
			t.Errorf("marks[%d].ID = %q, want %q", i, got, want)
		}
		if marks[i].Style.Fill != in.TrackStyle.Fill {
			t.Errorf("marks[%d] track did not take TrackStyle", i)
		}
	}
	for i := 3; i < 6; i++ {
		if got, want := marks[i].ID, "progress-"+string(rune('0'+i-3)); got != want {
			t.Errorf("marks[%d].ID = %q, want %q", i, got, want)
		}
	}
	// Every track is the same full-scale length; the bars are shorter
	// in descending order (90 / 60 / 30 against a total of 100).
	full := marks[0].Rect.W
	for i := 1; i < 3; i++ {
		if math.Abs(marks[i].Rect.W-full) > 1e-9 {
			t.Errorf("track %d width = %g, want %g", i, marks[i].Rect.W, full)
		}
	}
	if !(marks[3].Rect.W > marks[4].Rect.W && marks[4].Rect.W > marks[5].Rect.W) {
		t.Errorf("bar widths not descending: %g %g %g", marks[3].Rect.W, marks[4].Rect.W, marks[5].Rect.W)
	}
	if marks[3].Rect.W >= full {
		t.Errorf("bar 0 (%g) should be shorter than its track (%g)", marks[3].Rect.W, full)
	}
	// Track and bar share the row's slot exactly.
	for i := 0; i < 3; i++ {
		if marks[i].Rect.Y != marks[i+3].Rect.Y || marks[i].Rect.H != marks[i+3].Rect.H {
			t.Errorf("row %d: track slot (%g,%g) != bar slot (%g,%g)",
				i, marks[i].Rect.Y, marks[i].Rect.H, marks[i+3].Rect.Y, marks[i+3].Rect.H)
		}
	}
}

func TestPrismEncodeProgressBothHalvesCarryDatum(t *testing.T) {
	in := progressInputs(t)
	in.Mark = &spec.MarkDef{Type: "progress", Total: float64(100)}
	marks, _, err := Encode("progress", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for i, m := range marks {
		if m.Datum == nil {
			t.Fatalf("marks[%d] (%s) has no Datum", i, m.ID)
		}
		want := int64(i % 3)
		if m.Datum.RowID != want {
			t.Errorf("marks[%d] (%s) RowID = %d, want %d", i, m.ID, m.Datum.RowID, want)
		}
	}
}

func TestPrismEncodeProgressTotalPerRowField(t *testing.T) {
	in := progressInputs(t)
	in.Mark = &spec.MarkDef{Type: "progress", Total: "cap"}
	marks, _, err := Encode("progress", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// cap = 100 / 120 / 60 against an x domain of [0, 100], so the
	// tracks must differ per row rather than share one length.
	w0, w1, w2 := marks[0].Rect.W, marks[1].Rect.W, marks[2].Rect.W
	if !(w1 > w0 && w0 > w2) {
		t.Errorf("per-row track widths = %g %g %g, want row1 > row0 > row2", w0, w1, w2)
	}
}

func TestPrismEncodeProgressTotalDefaultsToDomainMax(t *testing.T) {
	in := progressInputs(t)
	in.Mark = &spec.MarkDef{Type: "progress"}
	marks, _, err := Encode("progress", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// The x scale's domain max is 100 → the plot's right edge.
	if got, want := marks[0].Rect.X+marks[0].Rect.W, in.Layout.Right(); math.Abs(got-want) > 1e-9 {
		t.Errorf("track far edge = %g, want the plot edge %g", got, want)
	}
}

func TestPrismEncodeProgressThickness(t *testing.T) {
	half := 0.5
	quarter := 0.25
	cases := []struct {
		name  string
		def   *spec.MarkDef
		ratio float64
	}{
		{"default", &spec.MarkDef{Type: "progress"}, spec.ProgressThicknessDefault},
		{"explicit", &spec.MarkDef{Type: "progress", Thickness: &quarter}, quarter},
		{"half", &spec.MarkDef{Type: "progress", Thickness: &half}, half},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := progressInputs(t)
			in.Mark = tc.def
			marks, _, err := Encode("progress", in)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			band, ok := in.Y.Scale.(BandScaler)
			if !ok {
				t.Fatal("y scale is not a band scale")
			}
			want := math.Abs(band.BandWidth()) * tc.ratio
			if got := marks[0].Rect.H; math.Abs(got-want) > 1e-9 {
				t.Errorf("row height = %g, want %g", got, want)
			}
		})
	}
}

// A progress mark takes its orientation from the shared mark.orient
// primitive (encode/marks/orient.go), not from a per-mark field:
// declaring "vertical" must move the category onto x.
func TestPrismEncodeProgressOrientVerticalUsesMarkOrient(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"cluster": []string{"eu", "us", "ap"},
		"used":    []float64{60, 90, 30},
	})
	plot := plotRect()
	in := Inputs{
		Table: tbl,
		X: Channel{Field: "cluster", Scale: &bandScaleT{
			cats: []string{"eu", "us", "ap"}, rmin: plot.X, rmax: plot.Right(),
		}},
		Y:      Channel{Field: "used", Scale: &linScale{dmin: 0, dmax: 100, rmin: plot.Bottom(), rmax: plot.Y}},
		Layout: plot,
		Mark:   &spec.MarkDef{Type: "progress", Orient: "vertical", Total: float64(100)},
	}
	marks, _, err := Encode("progress", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 6 {
		t.Fatalf("len(marks) = %d, want 6", len(marks))
	}
	// Vertical: the track spans the full measure axis (height), and the
	// bars grow up from the bottom of that track.
	if got, want := marks[0].Rect.H, plot.H; math.Abs(got-want) > 1e-9 {
		t.Errorf("vertical track height = %g, want the full plot height %g", got, want)
	}
	if marks[3].Rect.H >= marks[0].Rect.H {
		t.Errorf("bar height %g should be under the track height %g", marks[3].Rect.H, marks[0].Rect.H)
	}
	if got, want := marks[3].Rect.Y+marks[3].Rect.H, plot.Bottom(); math.Abs(got-want) > 1e-9 {
		t.Errorf("vertical bar does not sit on the baseline: %g, want %g", got, want)
	}
}

// A continuous pair has no axis that can host the category, so the
// shared primitive rejects it rather than guessing.
func TestPrismEncodeProgressRejectsUndrawableOrient(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"a": []float64{1, 2},
		"b": []float64{3, 4},
	})
	plot := plotRect()
	_, _, err := Encode("progress", Inputs{
		Table:  tbl,
		X:      Channel{Field: "a", Scale: &linScale{dmin: 0, dmax: 5, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: "b", Scale: &linScale{dmin: 0, dmax: 5, rmin: plot.Bottom(), rmax: plot.Y}},
		Layout: plot,
		Mark:   &spec.MarkDef{Type: "progress"},
	})
	if err == nil {
		t.Fatal("want an error for a progress mark with no band scale on either axis")
	}
}
