package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
)

// seriesTable builds a two-brand, two-period table in interleaved row
// order — the order a pivoted query returns, and the order that made
// the old single-polyline encoder zig-zag across the chart.
func seriesTable(t *testing.T) *table.Table {
	t.Helper()
	return buildTable(t, map[string]any{
		"p": []string{"Q1", "Q1", "Q2", "Q2"},
		"v": []float64{10, 40, 20, 50},
		"b": []string{"Aurora", "Borealis", "Aurora", "Borealis"},
	})
}

type fixedScale struct{ m map[any]float64 }

func (f fixedScale) Apply(v any) (float64, error) { return f.m[v], nil }
func (f fixedScale) Domain() []any                { return nil }

func seriesInputs(t *testing.T) Inputs {
	t.Helper()
	blue, _ := scene.ColorFromHex("#3366CC")
	orange, _ := scene.ColorFromHex("#DD6B0D")
	return Inputs{
		Table: seriesTable(t),
		X:     Channel{Field: "p", Scale: fixedScale{m: map[any]float64{"Q1": 0.0, "Q2": 100.0}}},
		Y:     Channel{Field: "v", Scale: fixedScale{m: map[any]float64{10.0: 90.0, 20.0: 80.0, 40.0: 60.0, 50.0: 50.0}}},
		Color: &ColorChannel{
			Field:      "b",
			Categories: []string{"Aurora", "Borealis"},
			Palette:    []*scene.Color{blue, orange},
		},
		Layout: scene.Rect{X: 0, Y: 0, W: 100, H: 100},
	}
}

// TestEncodeLineSplitsBySeries is the regression guard for a line
// chart that drew one polyline through every row: the path ran to the
// end of the first brand, jumped back across the chart, and continued
// — asserting connections between measurements that have nothing to do
// with each other, all in one colour.
func TestEncodeLineSplitsBySeries(t *testing.T) {
	got, err := encodeLine(seriesInputs(t))
	if err != nil {
		t.Fatalf("encodeLine: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d marks, want one per brand", len(got))
	}
	seen := map[string]bool{}
	for _, m := range got {
		if m.Line == nil || len(m.Line.Points) != 2 {
			t.Fatalf("series %q: got %v points, want 2", m.Series, m.Line)
		}
		if m.Style.Stroke == nil {
			t.Errorf("series %q has no stroke colour", m.Series)
		}
		seen[m.Series] = true
	}
	if !seen["Aurora"] || !seen["Borealis"] {
		t.Errorf("series names = %v, want Aurora and Borealis", seen)
	}
	if got[0].Style.Stroke.CSS() == got[1].Style.Stroke.CSS() {
		t.Error("two series drawn in the same colour; the legend would be lying")
	}
}

// TestEncodeLineWithoutColourIsOneRun asserts the single-series case is
// untouched: one mark, every row, the themed default stroke.
func TestEncodeLineWithoutColourIsOneRun(t *testing.T) {
	in := seriesInputs(t)
	in.Color = nil
	got, err := encodeLine(in)
	if err != nil {
		t.Fatalf("encodeLine: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d marks, want 1", len(got))
	}
	if len(got[0].Line.Points) != 4 {
		t.Errorf("got %d points, want all 4 rows", len(got[0].Line.Points))
	}
	if got[0].Series != "" {
		t.Errorf("Series = %q, want empty for a single-series chart", got[0].Series)
	}
}

// TestEncodeAreaSplitsBySeries mirrors the line case: an area polygon
// over every row closes across unrelated points and shades a region
// that means nothing.
func TestEncodeAreaSplitsBySeries(t *testing.T) {
	got, err := encodeArea(seriesInputs(t))
	if err != nil {
		t.Fatalf("encodeArea: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d marks, want one per brand", len(got))
	}
	for _, m := range got {
		if m.Area == nil || len(m.Area.Upper) != 2 {
			t.Errorf("series %q: upper edge = %v, want 2 points", m.Series, m.Area)
		}
	}
}

// TestSplitSeriesDropsUnknownCategories asserts a row whose colour
// value names no listed category is dropped rather than folded into
// the first series — guessing would draw a segment that does not exist.
func TestSplitSeriesDropsUnknownCategories(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"p": []string{"Q1", "Q2"},
		"v": []float64{1, 2},
		"b": []string{"Aurora", "Ghost"},
	})
	in := Inputs{
		Table: tbl,
		Color: &ColorChannel{Field: "b", Categories: []string{"Aurora"}},
	}
	runs := splitSeries(in, 2)
	if len(runs) != 1 || len(runs[0].Rows) != 1 || runs[0].Rows[0] != 0 {
		t.Errorf("runs = %+v, want only Aurora's row 0", runs)
	}
}

// TestBarRoundsOnlyTheValueEnd pins that a bar keeps its baseline
// square, and that a bar too short to hold the radius loses it rather
// than becoming a lozenge.
func TestBarRoundsOnlyTheValueEnd(t *testing.T) {
	if got := valueEndSide(5.0); got != "top" {
		t.Errorf("positive bar rounds %q, want top", got)
	}
	if got := valueEndSide(-5.0); got != "bottom" {
		t.Errorf("negative bar rounds %q, want bottom", got)
	}
	if got := clampCornerRadius(2, 40, 3); got != 1.5 {
		t.Errorf("radius on a 3px bar = %g, want 1.5 (half its height)", got)
	}
	if got := clampCornerRadius(2, 40, 0.6); got != 0 {
		t.Errorf("radius on a sub-pixel bar = %g, want 0", got)
	}
}
