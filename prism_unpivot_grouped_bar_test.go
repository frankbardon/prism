package prism_test

import (
	"context"
	"math"
	"testing"

	prism "github.com/frankbardon/prism"
	"github.com/frankbardon/prism/encode/scene"
)

// E5-S2 — the end-to-end check that `unpivot` (E5-S1) and the offset
// position channel (E1-S4) deliver the chart neither half delivers
// alone: a WIDE stored table reshaped into long form and then drawn as
// a grouped (dodged) bar.
//
// The starting fixture below is genuinely wide on purpose — two rows,
// one per series, and one COLUMN per metric, which is how a source
// schema stores a metric that is a column rather than a dimension. A
// long-form fixture would skip the reshape entirely and prove nothing
// about the unpivot half.
const wideBrandTrackerSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "values": [
      {"series": "Northwind",        "awareness": 68, "consideration": 47, "familiarity": 61, "trust": 72},
      {"series": "Category average", "awareness": 54, "consideration": 51, "familiarity": 57, "trust": 58}
    ]
  },
  "transform": [
    {
      "unpivot": ["awareness", "consideration", "familiarity", "trust"],
      "as": ["metric", "score"]
    }
  ],
  "mark": "bar",
  "encoding": {
    "x": {"field": "metric", "type": "nominal"},
    "y": {"field": "score", "type": "quantitative"},
    "x_offset": {
      "field": "series",
      "type": "nominal",
      "scale": {"domain": ["Northwind", "Category average"]}
    },
    "color": {
      "field": "series",
      "type": "nominal",
      "scale": {"domain": ["Northwind", "Category average"]}
    }
  }
}`

// The wide fixture's own contents, restated so the assertions can be
// read against the source table rather than against pixel constants.
// wideSeries is in the order pinned by `scale.domain` on both the
// offset and the colour channel, which is also the sub-band order.
// wideMetrics is in `unpivot` declaration order, which (the reshape
// being row-major) is also the first-seen order the x band scale
// assigns its slots from.
var (
	wideSeries  = []string{"Northwind", "Category average"}
	wideMetrics = []string{"awareness", "consideration", "familiarity", "trust"}
	// wideScores[seriesIndex][metricIndex] — the cell values exactly
	// as the wide table stores them.
	wideScores = [][]float64{
		{68, 47, 61, 72},
		{54, 51, 57, 58},
	}
)

const pixelEpsilon = 1e-6

// compileWide runs the whole Validate → Plan → Execute → Encode
// pipeline over the wide spec and returns the single cell's scene.
func compileWide(t *testing.T) (*prism.CompiledPlan, scene.Scene) {
	t.Helper()
	p, err := prism.CompileJSON(context.Background(), []byte(wideBrandTrackerSpec), prism.CompileOptions{})
	if err != nil {
		t.Fatalf("CompileJSON: %v", err)
	}
	if p == nil || p.Scene == nil {
		t.Fatal("CompileJSON returned no scene")
	}
	if got := len(p.Scene.Grid.Cells); got != 1 {
		t.Fatalf("grid cells = %d want 1 (flat spec)", got)
	}
	return p, p.Scene.Grid.Cells[0].Scene
}

// barRects returns the single layer's rect geometries in mark order.
func barRects(t *testing.T, sc scene.Scene) []scene.Mark {
	t.Helper()
	if len(sc.Layers) != 1 {
		t.Fatalf("layers = %d want 1", len(sc.Layers))
	}
	marks := sc.Layers[0].Marks
	for i := range marks {
		if marks[i].Rect == nil {
			t.Fatalf("mark %d (%s) has no rect geometry; type=%q", i, marks[i].ID, marks[i].Type)
		}
	}
	return marks
}

// rowIndex is the row the reshape produces for one (series, metric)
// cell. E5-S1 pins unpivot output as row-major: every measure of input
// row 0 in declaration order, then every measure of input row 1.
func rowIndex(seriesIdx, metricIdx int) int64 {
	return int64(seriesIdx*len(wideMetrics) + metricIdx)
}

// TestUnpivotFeedsGroupedBarOneRectPerPair is the headline assertion:
// a two-row wide table with four metric columns draws exactly one bar
// per (series, metric) pair, and each bar carries the value the wide
// table stored in that cell.
func TestUnpivotFeedsGroupedBarOneRectPerPair(t *testing.T) {
	p, sc := compileWide(t)
	marks := barRects(t, sc)

	wantRects := len(wideSeries) * len(wideMetrics)
	if len(marks) != wantRects {
		t.Fatalf("rect count = %d want %d (%d series x %d metrics)",
			len(marks), wantRects, len(wideSeries), len(wideMetrics))
	}
	if len(p.Marks) != 1 || p.Marks[0].InstanceCount != wantRects {
		t.Errorf("CompiledPlan.Marks = %+v; want a single rect layer with %d instances", p.Marks, wantRects)
	}

	// Every rect must be reachable by its (series, metric) row id, and
	// no row id may appear twice — that is what "one per pair" means.
	byRow := map[int64]scene.Mark{}
	for _, m := range marks {
		if m.Datum == nil {
			t.Fatalf("mark %s carries no datum back-reference", m.ID)
		}
		if prev, dup := byRow[m.Datum.RowID]; dup {
			t.Fatalf("row %d drawn twice: %s and %s", m.Datum.RowID, prev.ID, m.ID)
		}
		byRow[m.Datum.RowID] = m
	}

	// The reshape moved a wide CELL into a long ROW. Read each bar's
	// height back through the resolved y scale and require it to be
	// the value that cell held — this is the assertion that fails if
	// unpivot routes the wrong column's value to a pair.
	toValue := yInverse(t, p, sc)
	for si, series := range wideSeries {
		for mi, metric := range wideMetrics {
			m, ok := byRow[rowIndex(si, mi)]
			if !ok {
				t.Fatalf("no bar for (%s, %s)", series, metric)
			}
			got := toValue(m.Rect.Y)
			if math.Abs(got-wideScores[si][mi]) > 1e-9 {
				t.Errorf("(%s, %s): bar top decodes to %g, want the stored cell %g",
					series, metric, got, wideScores[si][mi])
			}
		}
	}
}

// TestUnpivotGroupedBarIsDodgedNotStacked pins the three properties
// that separate a dodged chart from a stacked one: every bar shares
// one baseline, the pair inside a metric sits at two DIFFERENT x
// positions, and those two sub-bands are adjacent and equally wide.
func TestUnpivotGroupedBarIsDodgedNotStacked(t *testing.T) {
	p, sc := compileWide(t)
	marks := barRects(t, sc)

	// The baseline is read OFF THE SCENE, never hard-coded: a stacked
	// chart would not have every bar sitting on it.
	baseline := sc.Plot.Y + sc.Plot.H
	if baseline <= 0 {
		t.Fatalf("plot rect looks unresolved: %+v", sc.Plot)
	}
	for _, m := range marks {
		if bottom := m.Rect.Y + m.Rect.H; math.Abs(bottom-baseline) > pixelEpsilon {
			t.Errorf("%s bottom = %g, want the shared baseline Plot.Y+Plot.H = %g (a stacked bar would float above it)",
				m.ID, bottom, baseline)
		}
	}

	// Equal widths: every sub-band is the parent slot divided by the
	// number of offset categories, so all eight must agree.
	width := marks[0].Rect.W
	if width <= 0 {
		t.Fatalf("first rect has non-positive width %g", width)
	}
	for _, m := range marks {
		if math.Abs(m.Rect.W-width) > pixelEpsilon {
			t.Errorf("%s width = %g, want %g (all sub-bands are equal)", m.ID, m.Rect.W, width)
		}
	}

	byRow := map[int64]scene.Mark{}
	for _, m := range marks {
		byRow[m.Datum.RowID] = m
	}

	// Within each metric the two series must be side by side: distinct
	// x, adjacent (the offset scale's padding defaults to inner 0 /
	// outer 0, so sub-bands touch and together fill the slot), and in
	// the order pinned by scale.domain.
	for mi, metric := range wideMetrics {
		first := byRow[rowIndex(0, mi)]
		second := byRow[rowIndex(1, mi)]
		if math.Abs(first.Rect.X-second.Rect.X) <= pixelEpsilon {
			t.Errorf("%s: both series drawn at x=%g — that is a stacked (or overplotted) bar, not a dodged pair",
				metric, first.Rect.X)
			continue
		}
		if first.Rect.X >= second.Rect.X {
			t.Errorf("%s: %q at x=%g is not left of %q at x=%g; scale.domain pins the sub-band order",
				metric, wideSeries[0], first.Rect.X, wideSeries[1], second.Rect.X)
		}
		if gap := second.Rect.X - (first.Rect.X + first.Rect.W); math.Abs(gap) > pixelEpsilon {
			t.Errorf("%s: sub-bands are %g px apart, want adjacent (offset padding defaults to 0)", metric, gap)
		}
	}

	// Across metrics, the pairs must not overlap: the whole point of
	// the band scale is that each metric owns a slot.
	for mi := 1; mi < len(wideMetrics); mi++ {
		prevRight := byRow[rowIndex(1, mi-1)].Rect.X + width
		thisLeft := byRow[rowIndex(0, mi)].Rect.X
		if thisLeft < prevRight-pixelEpsilon {
			t.Errorf("%s starts at x=%g, inside %s which ends at x=%g",
				wideMetrics[mi], thisLeft, wideMetrics[mi-1], prevRight)
		}
	}

	// A dodged bar draws each row's full value from the baseline. If
	// the chart had stacked, the upper segment's height would be its
	// own value but its bottom would sit on the lower segment.
	toValue := yInverse(t, p, sc)
	for si := range wideSeries {
		for mi := range wideMetrics {
			m := byRow[rowIndex(si, mi)]
			full := baseline - m.Rect.H
			if math.Abs(toValue(full)-wideScores[si][mi]) > 1e-9 {
				t.Errorf("(%s, %s): height %g does not span from the baseline to the row's own value",
					wideSeries[si], wideMetrics[mi], m.Rect.H)
			}
		}
	}
}

// TestUnpivotGroupedBarColoursBySeries confirms the colour channel and
// the offset channel agree: all four bars of a series share one fill,
// and the two series differ.
func TestUnpivotGroupedBarColoursBySeries(t *testing.T) {
	_, sc := compileWide(t)
	marks := barRects(t, sc)

	byRow := map[int64]scene.Mark{}
	for _, m := range marks {
		byRow[m.Datum.RowID] = m
	}

	fills := make([]scene.Color, len(wideSeries))
	for si, series := range wideSeries {
		for mi, metric := range wideMetrics {
			m := byRow[rowIndex(si, mi)]
			if m.Style.Fill == nil {
				t.Fatalf("(%s, %s) has no resolved fill", series, metric)
			}
			if mi == 0 {
				fills[si] = *m.Style.Fill
				continue
			}
			if *m.Style.Fill != fills[si] {
				t.Errorf("(%s, %s) fill = %+v, want the series fill %+v", series, metric, *m.Style.Fill, fills[si])
			}
		}
	}
	if fills[0] == fills[1] {
		t.Errorf("both series painted %+v; a grouped bar must distinguish them", fills[0])
	}
}

// TestUnpivotGroupedBarCompilesWithoutWarnings keeps the feature pair
// off the diagnostics channel: a chart that works must not also tell
// the author something is inert or dropped.
func TestUnpivotGroupedBarCompilesWithoutWarnings(t *testing.T) {
	p, sc := compileWide(t)
	if len(p.Diagnostics) != 0 {
		t.Errorf("CompiledPlan.Diagnostics = %+v, want none", p.Diagnostics)
	}
	if len(p.Scene.Warnings) != 0 {
		t.Errorf("SceneDoc.Warnings = %+v, want none", p.Scene.Warnings)
	}
	if len(sc.Legends) == 0 {
		t.Error("no legend built for the colour channel")
	}
}

// yInverse builds the domain-value ⟵ pixel inverse of the resolved y
// scale, read off the CompiledPlan rather than recomputed from the
// layout, so the assertions above never hard-code a pixel.
func yInverse(t *testing.T, p *prism.CompiledPlan, sc scene.Scene) func(float64) float64 {
	t.Helper()
	for _, s := range p.Scales {
		if s.Channel != "y" {
			continue
		}
		if s.Type != "linear" {
			t.Fatalf("y scale type = %q want linear", s.Type)
		}
		if len(s.Domain) != 2 {
			t.Fatalf("y domain = %v want two bounds", s.Domain)
		}
		lo, ok0 := numeric(s.Domain[0])
		hi, ok1 := numeric(s.Domain[1])
		if !ok0 || !ok1 {
			t.Fatalf("y domain %v is not numeric", s.Domain)
		}
		p0, p1 := s.Range[0], s.Range[1]
		if p1 == p0 {
			t.Fatalf("y range is degenerate: %v", s.Range)
		}
		return func(px float64) float64 {
			return lo + (px-p0)/(p1-p0)*(hi-lo)
		}
	}
	t.Fatalf("no y scale in CompiledPlan (plot=%+v)", sc.Plot)
	return nil
}

func numeric(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
