package encode

import "testing"

// TestPrismLayoutMeasuresLabels asserts the plot rect is derived from
// the labels it has to make room for, not from a fixed inset.
func TestPrismLayoutMeasuresLabels(t *testing.T) {
	st := DefaultLayoutStyle()
	narrow := Compute(LayoutInputs{
		Width: 800, Height: 600, Style: st,
		HasXAxis: true, HasYAxis: true,
		YLabels: []string{"0", "5", "10"},
		XLabels: []string{"a", "b", "c"},
	})
	wide := Compute(LayoutInputs{
		Width: 800, Height: 600, Style: st,
		HasXAxis: true, HasYAxis: true,
		YLabels: []string{"0", "1,284,000", "2,568,000"},
		XLabels: []string{"a", "b", "c"},
	})
	if wide.Plot.X <= narrow.Plot.X {
		t.Errorf("wide y labels must claim more left margin: narrow X=%g wide X=%g",
			narrow.Plot.X, wide.Plot.X)
	}
	if wide.Plot.W >= narrow.Plot.W {
		t.Errorf("the margin has to come out of the plot: narrow W=%g wide W=%g",
			narrow.Plot.W, wide.Plot.W)
	}
	if narrow.Frame.W != 800 || narrow.Frame.H != 600 {
		t.Errorf("Frame = %+v, want 800x600", narrow.Frame)
	}
}

// TestPrismLayoutTitleReservesTop asserts a titled chart pushes the
// plot down and a bare one does not.
func TestPrismLayoutTitleReservesTop(t *testing.T) {
	st := DefaultLayoutStyle()
	base := LayoutInputs{Width: 800, Height: 600, Style: st, HasXAxis: true, HasYAxis: true}
	bare := Compute(base)
	base.Title = "Brand awareness"
	titled := Compute(base)
	if titled.Plot.Y <= bare.Plot.Y {
		t.Errorf("title must reserve top space: bare Y=%g titled Y=%g", bare.Plot.Y, titled.Plot.Y)
	}
	if titled.Plot.H >= bare.Plot.H {
		t.Errorf("title space comes out of the plot: bare H=%g titled H=%g", bare.Plot.H, titled.Plot.H)
	}
}

// TestPrismLayoutReservesLegendOutsidePlot is the regression guard for
// the overlapping legend: the reserved frame must begin at or after
// the plot's right edge, never inside it.
func TestPrismLayoutReservesLegendOutsidePlot(t *testing.T) {
	st := DefaultLayoutStyle()
	res := ReserveSymbolLegend(LegendInputs{
		Channel:    "color",
		Title:      "brand",
		Categories: []string{"Aurora", "Borealis", "Cinder"},
		Style:      st,
	}, 800, 600)
	if res == nil {
		t.Fatal("three categories must reserve a legend")
	}
	l := Compute(LayoutInputs{
		Width: 800, Height: 600, Style: st,
		HasXAxis: true, HasYAxis: true,
		YLabels: []string{"0", "20"},
		XLabels: []string{"Q1", "Q2", "Q3"},
		Legend:  res,
	})
	if l.LegendFrame.X < l.Plot.Right() {
		t.Errorf("legend frame overlaps the plot: legend X=%g plot right=%g",
			l.LegendFrame.X, l.Plot.Right())
	}
	if l.LegendFrame.Right() > l.Frame.W {
		t.Errorf("legend frame overflows the SVG: legend right=%g frame W=%g",
			l.LegendFrame.Right(), l.Frame.W)
	}
}

// TestPrismLayoutSingleCategoryNeverPresentsAsWhole asserts the
// degenerate case reads as one measurement rather than as a chart that
// failed to draw: one band must not fill the plot.
func TestPrismLayoutSingleCategoryBandIsCapped(t *testing.T) {
	b := &BandScale{Categories: []string{"Aurora"}, RangeMin: 40, RangeMax: 780}
	applyBandShape(b, 0.28, 96)
	if got := b.BandWidth(); got > 96.5 {
		t.Errorf("single band width = %g, want <= 96", got)
	}
	if got := b.BandWidth(); got < 10 {
		t.Errorf("single band width = %g, want a visible bar", got)
	}
}

// TestPrismLayoutSparklineUnchanged pins the spark path, which must
// keep its tight 4px inset and skip every reservation above.
func TestPrismLayoutSparklineUnchanged(t *testing.T) {
	l := ComputeSparkline(200, 40)
	if l.Plot.X != 4 || l.Plot.Y != 4 || l.Plot.W != 192 || l.Plot.H != 32 {
		t.Errorf("sparkline Plot = %+v, want {4,4,192,32}", l.Plot)
	}
}
