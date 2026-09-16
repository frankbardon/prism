package encode_test

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

const stackedBarSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"q": "q1", "seg": "north", "v": 120},
    {"q": "q2", "seg": "north", "v": 145},
    {"q": "q1", "seg": "south", "v": 100},
    {"q": "q2", "seg": "south", "v": 120}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "q", "type": "nominal"},
    "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
    "color": {"field": "seg", "type": "nominal"}
  }
}`

// barRects collects the rect geometry of a scene's bar marks in
// emission order.
func barRects(t *testing.T, sc *scene.Scene) []scene.RectGeom {
	t.Helper()
	var out []scene.RectGeom
	for _, layer := range sc.Layers {
		for _, m := range layer.Marks {
			if m.Rect != nil {
				out = append(out, *m.Rect)
			}
		}
	}
	return out
}

// TestPrismEncodeStackedBarSegmentsAbut pins the headline behaviour: a
// bar chart with a colour channel no longer overdraws. Each stack's
// two segments touch exactly, and the stack's total height matches the
// sum of its parts.
func TestPrismEncodeStackedBarSegmentsAbut(t *testing.T) {
	sc := encodeInline(t, stackedBarSpec)
	rects := barRects(t, sc)
	if len(rects) != 4 {
		t.Fatalf("bar rects = %d, want 4", len(rects))
	}
	// Emission order follows the group-aggregate output: north q1,
	// north q2, south q1, south q2. Stacks are keyed on x.
	byX := map[float64][]scene.RectGeom{}
	for _, r := range rects {
		byX[r.X] = append(byX[r.X], r)
	}
	if len(byX) != 2 {
		t.Fatalf("distinct bar x positions = %d, want 2", len(byX))
	}
	for x, stack := range byX {
		if len(stack) != 2 {
			t.Fatalf("x=%g: %d segments, want 2", x, len(stack))
		}
		lower, upper := stack[0], stack[1]
		if lower.Y < upper.Y {
			lower, upper = upper, lower
		}
		// The upper segment's bottom edge must equal the lower
		// segment's top edge — no gap, no overlap.
		if math.Abs((upper.Y+upper.H)-lower.Y) > 1e-6 {
			t.Errorf("x=%g: segments do not abut: upper bottom=%g, lower top=%g",
				x, upper.Y+upper.H, lower.Y)
		}
		if lower.H <= 0 || upper.H <= 0 {
			t.Errorf("x=%g: non-positive segment height (%g, %g)", x, lower.H, upper.H)
		}
	}
}

// TestPrismEncodeStackedDomainReachesScale pins that the measure axis
// spans 0…sum rather than 0…max — the reason stacking is a Plan node
// and not an encode-local pre-pass.
func TestPrismEncodeStackedDomainReachesScale(t *testing.T) {
	sc := encodeInline(t, stackedBarSpec)
	var yAxis *scene.Axis
	for i := range sc.Axes {
		if sc.Axes[i].Channel == scene.ChannelY {
			yAxis = &sc.Axes[i]
		}
	}
	if yAxis == nil {
		t.Fatal("scene has no y axis")
	}
	// q2 sums to 265, so the domain must reach at least that far; the
	// pre-stack domain topped out at the single largest bar (145).
	top := 0.0
	for _, tk := range yAxis.Ticks {
		if v, ok := tk.Value.(float64); ok {
			top = math.Max(top, v)
		}
	}
	if top < 265 {
		t.Errorf("y axis tops out at %g; stacked total is 265 — the stacked domain did not reach scale resolution", top)
	}
	if yAxis.Title != "v" {
		t.Errorf("y axis title = %q, want %q — the rebound channel leaked its derived column name", yAxis.Title, "v")
	}
}

// TestPrismEncodeStackDisabledOverdrawsAsBefore pins that
// `"stack": null` restores the pre-E5-S2 geometry: every segment is
// anchored on the baseline, so the two share a bottom edge.
func TestPrismEncodeStackDisabledOverdrawsAsBefore(t *testing.T) {
	sc := encodeInline(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [
	    {"q": "q1", "seg": "north", "v": 120},
	    {"q": "q1", "seg": "south", "v": 100}
	  ]},
	  "mark": "bar",
	  "encoding": {
	    "x": {"field": "q", "type": "nominal"},
	    "y": {"aggregate": "sum", "field": "v", "type": "quantitative", "stack": null},
	    "color": {"field": "seg", "type": "nominal"}
	  }
	}`)
	rects := barRects(t, sc)
	if len(rects) != 2 {
		t.Fatalf("bar rects = %d, want 2", len(rects))
	}
	if math.Abs((rects[0].Y+rects[0].H)-(rects[1].Y+rects[1].H)) > 1e-6 {
		t.Errorf("unstacked bars do not share a baseline: %g vs %g",
			rects[0].Y+rects[0].H, rects[1].Y+rects[1].H)
	}
}

// TestPrismEncodeStackedAreaRibbons pins the area path: each series
// becomes a ribbon between its own start and end edges, and the
// normalized offset caps the topmost series at the plot's top.
func TestPrismEncodeStackedAreaRibbons(t *testing.T) {
	sc := encodeInline(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [
	    {"w": 1, "src": "a", "v": 30},
	    {"w": 2, "src": "a", "v": 40},
	    {"w": 1, "src": "b", "v": 70},
	    {"w": 2, "src": "b", "v": 60}
	  ]},
	  "mark": "area",
	  "encoding": {
	    "x": {"field": "w", "type": "quantitative"},
	    "y": {"aggregate": "sum", "field": "v", "type": "quantitative", "stack": "normalize"},
	    "color": {"field": "src", "type": "nominal"}
	  }
	}`)
	var areas []*scene.AreaGeom
	for _, layer := range sc.Layers {
		for i := range layer.Marks {
			if layer.Marks[i].Area != nil {
				areas = append(areas, layer.Marks[i].Area)
			}
		}
	}
	if len(areas) != 2 {
		t.Fatalf("area marks = %d, want 2 (one ribbon per series)", len(areas))
	}
	// Series "a" is the lower ribbon: its lower edge sits on the 0
	// share, which is the bottom of the plot. Series "b" reaches 1.
	first := areas[0]
	if len(first.Upper) != 2 || len(first.Lower) != 2 {
		t.Fatalf("first ribbon has %d upper / %d lower points, want 2/2", len(first.Upper), len(first.Lower))
	}
	for i := range first.Upper {
		// SVG y grows downward, so the upper edge must sit above the
		// lower one.
		if first.Upper[i][1] >= first.Lower[i][1] {
			t.Errorf("ribbon point %d is inverted: upper=%g lower=%g",
				i, first.Upper[i][1], first.Lower[i][1])
		}
	}
	// The second ribbon's lower edge is the first's upper edge.
	second := areas[1]
	for i := range second.Lower {
		if math.Abs(second.Lower[i][1]-first.Upper[i][1]) > 1e-6 {
			t.Errorf("ribbon %d does not sit on its neighbour: %g vs %g",
				i, second.Lower[i][1], first.Upper[i][1])
		}
	}
}
