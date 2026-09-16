package marks

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// E9-S2: every cartesian family beyond bar resolves its axes through
// MarkOrientation. These tests pin the horizontal geometry — the
// vertical direction is pinned byte-for-byte by the committed gallery
// and testdata goldens, which is the stronger guarantee.

// categoryBandInputs builds a fixture whose band sits on y (running
// bottom-to-top, so its step is negative) and whose measure sits on a
// linear x straddling zero — the shape every horizontal mark needs.
func categoryBandInputs(t *testing.T, catField, valField string, cats []string, vals []float64, def *spec.MarkDef) Inputs {
	t.Helper()
	anyCats := make([]string, len(cats))
	copy(anyCats, cats)
	tbl := buildTable(t, map[string]any{catField: anyCats, valField: vals})
	plot := plotRect()
	uniq := []string{}
	seen := map[string]bool{}
	for _, c := range cats {
		if !seen[c] {
			seen[c] = true
			uniq = append(uniq, c)
		}
	}
	return Inputs{
		Table:  tbl,
		X:      Channel{Field: valField, Scale: &linScale{dmin: -1, dmax: 1, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: catField, Scale: &bandScaleT{cats: uniq, rmin: plot.Bottom(), rmax: plot.Y, padding: 0.1}},
		Layout: plot,
		Style:  scene.Style{},
		Mark:   def,
	}
}

func TestPrismTickOrientationHorizontal(t *testing.T) {
	in := categoryBandInputs(t, "group", "value",
		[]string{"a", "b"}, []float64{0.5, -0.5}, nil)

	got, err := encodeTick(in)
	if err != nil {
		t.Fatalf("encodeTick: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d ticks, want 2", len(got))
	}
	// A horizontal tick runs along x (the measure axis) and sits at
	// the centre of its y band.
	for i, m := range got {
		p := m.Line.Points
		if len(p) != 2 {
			t.Fatalf("tick %d: got %d points, want 2", i, len(p))
		}
		if p[0][1] != p[1][1] {
			t.Errorf("tick %d: endpoints differ in y (%v vs %v); horizontal tick must be level", i, p[0][1], p[1][1])
		}
		if width := p[1][0] - p[0][0]; math.Abs(width-10) > 1e-9 {
			t.Errorf("tick %d: length %v along x, want the 10px default", i, width)
		}
	}
	// Band centres: the y band runs bottom-to-top with 10% padding.
	band := in.Y.Scale.(*bandScaleT)
	for i, cat := range []string{"a", "b"} {
		start, _ := band.Apply(cat)
		want := start + band.BandWidth()/2
		if diff := math.Abs(got[i].Line.Points[0][1] - want); diff > 1e-9 {
			t.Errorf("tick %d: y = %v, want band centre %v", i, got[i].Line.Points[0][1], want)
		}
	}
}

func TestPrismTickOrientationDefaultsUnchangedWithoutBand(t *testing.T) {
	// Two continuous axes have no band to infer from. Tick has always
	// drawn the horizontal strip shape there, and MarkOrientationOr's
	// fallback must keep doing so rather than erroring.
	plot := plotRect()
	tbl := buildTable(t, map[string]any{
		"a": []float64{1, 2},
		"b": []float64{3, 4},
	})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "a", Scale: &linScale{dmin: 0, dmax: 4, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: "b", Scale: &linScale{dmin: 0, dmax: 4, rmin: plot.Bottom(), rmax: plot.Y}},
		Layout: plot,
	}
	got, err := encodeTick(in)
	if err != nil {
		t.Fatalf("encodeTick: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d ticks, want 2", len(got))
	}
	for i, m := range got {
		p := m.Line.Points
		if p[0][1] != p[1][1] {
			t.Errorf("tick %d: expected a level (horizontal) tick, got %v", i, p)
		}
	}
}

func TestPrismAreaOrientationHorizontal(t *testing.T) {
	in := categoryBandInputs(t, "step", "vol",
		[]string{"a", "b"}, []float64{0.5, 1}, &spec.MarkDef{Type: "area", Orient: "horizontal"})
	// An area's series axis is normally continuous; swap the band out
	// so the explicit orient is doing the work.
	plot := in.Layout
	in.Y = Channel{Field: "step", Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}}
	in.Table = buildTable(t, map[string]any{
		"step": []float64{0, 1},
		"vol":  []float64{0.5, 1},
	})

	got, err := encodeArea(in)
	if err != nil {
		t.Fatalf("encodeArea: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d area marks, want 1", len(got))
	}
	// The baseline edge is x = 0's pixel, so every Lower point shares
	// that x while the Upper points carry the measured value.
	baseX, _ := in.X.Scale.Apply(float64(0))
	for i, p := range got[0].Area.Lower {
		if math.Abs(p[0]-baseX) > 1e-9 {
			t.Errorf("lower[%d] x = %v, want the x=0 baseline %v", i, p[0], baseX)
		}
	}
	if len(got[0].Area.Upper) != 2 {
		t.Fatalf("got %d upper points, want 2", len(got[0].Area.Upper))
	}
	if got[0].Area.Upper[0][0] <= baseX {
		t.Errorf("upper[0] x = %v, want it right of the baseline %v", got[0].Area.Upper[0][0], baseX)
	}
}

func TestPrismAreaRejectsY2WhenHorizontal(t *testing.T) {
	in := categoryBandInputs(t, "step", "vol",
		[]string{"a", "b"}, []float64{0.5, 1}, &spec.MarkDef{Type: "area", Orient: "horizontal"})
	plot := in.Layout
	yScale := &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}
	in.Y = Channel{Field: "step", Scale: yScale}
	in.Y2 = Channel{Field: "step2", Scale: yScale}
	in.Table = buildTable(t, map[string]any{
		"step":  []float64{0, 1},
		"step2": []float64{0, 1},
		"vol":   []float64{0.5, 1},
	})
	if _, err := encodeArea(in); err == nil {
		t.Fatal("expected a horizontal area with y2 bound to be rejected, got no error")
	}
}

func TestPrismBoxplotOrientationHorizontal(t *testing.T) {
	in := categoryBandInputs(t, "group", "score",
		[]string{"a", "a", "a", "b", "b", "b"},
		[]float64{0.1, 0.3, 0.45, 0.5, 0.7, 0.9}, nil)

	got, err := encodeBoxplot(in)
	if err != nil {
		t.Fatalf("encodeBoxplot: %v", err)
	}
	var boxes int
	for _, m := range got {
		if m.Rect == nil {
			continue
		}
		boxes++
		if m.Rect.W <= 0 || m.Rect.H <= 0 {
			t.Errorf("%s: non-positive rect extent %+v", m.ID, *m.Rect)
		}
		// Thickness lands on y (the category axis) and the IQR on x.
		if math.Abs(m.Rect.H-math.Abs(in.Y.Scale.(*bandScaleT).BandWidth())) > 1e-9 {
			t.Errorf("%s: height %v, want the band width", m.ID, m.Rect.H)
		}
	}
	if boxes != 2 {
		t.Fatalf("got %d boxes, want 2", boxes)
	}
	// The median rule spans the band across y at a single x.
	for _, m := range got {
		if m.Rule == nil || m.ID != "boxplot-a-median" {
			continue
		}
		if m.Rule.X1 != m.Rule.X2 {
			t.Errorf("median rule should sit at one x, got %v→%v", m.Rule.X1, m.Rule.X2)
		}
		if m.Rule.Y1 == m.Rule.Y2 {
			t.Error("median rule should span the band across y")
		}
	}
}

func TestPrismBoxplotSummariesFollowOrientation(t *testing.T) {
	in := categoryBandInputs(t, "group", "score",
		[]string{"a", "a", "b", "b"},
		[]float64{0.1, 0.3, 0.5, 0.9}, nil)
	summaries, err := ComputeBoxplotSummaries(in)
	if err != nil {
		t.Fatalf("ComputeBoxplotSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d summaries, want 2", len(summaries))
	}
	if summaries[0].Group != "a" || summaries[1].Group != "b" {
		t.Errorf("groups = %q/%q, want a/b — a horizontal boxplot groups by y", summaries[0].Group, summaries[1].Group)
	}
	if summaries[1].Max != 0.9 {
		t.Errorf("group b max = %v, want 0.9 read from x", summaries[1].Max)
	}
}

func TestPrismViolinOrientationHorizontal(t *testing.T) {
	in := categoryBandInputs(t, "group", "score",
		[]string{"a", "a", "a", "b", "b", "b"},
		[]float64{0.1, 0.3, 0.45, 0.5, 0.7, 0.9}, nil)

	got, err := encodeViolin(in)
	if err != nil {
		t.Fatalf("encodeViolin: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d violins, want 2", len(got))
	}
	band := in.Y.Scale.(*bandScaleT)
	half := math.Abs(band.BandWidth()) / 2
	for _, m := range got {
		if m.Area == nil || len(m.Area.Upper) == 0 {
			t.Fatalf("%s: no area geometry", m.ID)
		}
		// The density fans out across y; the sampled value walks x.
		for i := range m.Area.Upper {
			if m.Area.Upper[i][0] != m.Area.Lower[i][0] {
				t.Fatalf("%s: sample %d should share one x across the fan", m.ID, i)
			}
			if spread := m.Area.Upper[i][1] - m.Area.Lower[i][1]; spread < 0 || spread > 2*half+1e-9 {
				t.Fatalf("%s: sample %d spread %v outside [0, band width]", m.ID, i, spread)
			}
		}
	}
}

func TestPrismWinlossOrientationHorizontal(t *testing.T) {
	in := categoryBandInputs(t, "game", "result",
		[]string{"g1", "g2", "g3"}, []float64{1, -1, 0}, nil)

	got, err := encodeWinloss(in)
	if err != nil {
		t.Fatalf("encodeWinloss: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d bars, want 3", len(got))
	}
	baseX, _ := in.X.Scale.Apply(float64(0))
	wantLen := in.Layout.W * winlossHeightRatio
	// A win grows rightward from the baseline, a loss leftward, a draw
	// is a flat marker on the baseline itself.
	if r := got[0].Rect; math.Abs(r.X-baseX) > 1e-9 || math.Abs(r.W-wantLen) > 1e-9 {
		t.Errorf("win rect = %+v, want x=%v w=%v", *r, baseX, wantLen)
	}
	if r := got[1].Rect; math.Abs(r.X-(baseX-wantLen)) > 1e-9 || math.Abs(r.W-wantLen) > 1e-9 {
		t.Errorf("loss rect = %+v, want x=%v w=%v", *r, baseX-wantLen, wantLen)
	}
	if r := got[2].Rect; r.W != 0 || math.Abs(r.X-baseX) > 1e-9 {
		t.Errorf("draw rect = %+v, want a zero-width marker on the baseline %v", *r, baseX)
	}
	for _, m := range got {
		if m.Rect.H <= 0 {
			t.Errorf("%s: non-positive height %v — the y band step is negative and must be normalised", m.ID, m.Rect.H)
		}
	}
}

func TestPrismCategoryCentersToleratesContinuousAxis(t *testing.T) {
	// The spark encoders position against a continuous x. An axis with
	// no band has no slot to centre in, so the midpoint is the value's
	// own pixel — which is what keeps sparkline output byte-identical.
	plot := plotRect()
	in := Inputs{
		Table:  buildTable(t, map[string]any{"t": []float64{0, 4}}),
		X:      Channel{Field: "t", Scale: &linScale{dmin: 0, dmax: 4, rmin: plot.X, rmax: plot.Right()}},
		Layout: plot,
	}
	got, err := CategoryCenters(in, OrientVertical)
	if err != nil {
		t.Fatalf("CategoryCenters: %v", err)
	}
	want := []float64{plot.X, plot.Right()}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("center[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestPrismHeatmapBandYPositiveHeight(t *testing.T) {
	// Heatmap's inline band normaliser was folded onto rectAxisExtent
	// in E9-S2. A y band runs bottom-to-top, so its step is negative;
	// the shared helper must still hand back a positive height.
	plot := plotRect()
	in := Inputs{
		Table: buildTable(t, map[string]any{
			"col": []string{"c1", "c2"},
			"row": []string{"r1", "r2"},
			"v":   []float64{1, 2},
		}),
		X:      Channel{Field: "col", Scale: &bandScaleT{cats: []string{"c1", "c2"}, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: "row", Scale: &bandScaleT{cats: []string{"r1", "r2"}, rmin: plot.Bottom(), rmax: plot.Y}},
		Color:  &ColorChannel{Field: "v"},
		Layout: plot,
	}
	got, err := encodeHeatmap(in)
	if err != nil {
		t.Fatalf("encodeHeatmap: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d cells, want 2", len(got))
	}
	for _, m := range got {
		if m.Rect.W <= 0 || m.Rect.H <= 0 {
			t.Errorf("%s: non-positive cell extent %+v", m.ID, *m.Rect)
		}
	}
}
