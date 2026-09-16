package encode

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// defaultOpts is the standard cartesian input: x on the bottom, y on
// the left, no out-of-plot legend reservation.
func defaultOpts(w, h float64, title bool) LayoutOpts {
	return LayoutOpts{Width: w, Height: h, Title: title, Sides: DefaultAxisPlacement().Sides()}
}

func TestPrismLayoutDefaults(t *testing.T) {
	l := Compute(defaultOpts(800, 600, false))
	if l.Frame.W != 800 || l.Frame.H != 600 {
		t.Errorf("Frame = %+v, want {0,0,800,600}", l.Frame)
	}
	if l.Plot.X != 40 || l.Plot.Y != 20 || l.Plot.W != 740 || l.Plot.H != 540 {
		t.Errorf("Plot = %+v, want {40,20,740,540}", l.Plot)
	}
}

func TestPrismLayoutWithTitle(t *testing.T) {
	l := Compute(defaultOpts(800, 600, true))
	if l.Plot.Y != 50 || l.Plot.H != 510 {
		t.Errorf("Plot with title = %+v, want Y=50 H=510", l.Plot)
	}
}

func TestPrismLayoutCustomDimensions(t *testing.T) {
	l := Compute(defaultOpts(1200, 400, false))
	if l.Plot.W != 1140 || l.Plot.H != 340 {
		t.Errorf("Plot = %+v, want W=1140 H=340", l.Plot)
	}
}

// TestPrismLayoutDefaultPaddingUnchanged pins the exact padding the
// pre-refactor DefaultPadding returned, so the derived computation
// cannot drift the plot rect for default input.
func TestPrismLayoutDefaultPaddingUnchanged(t *testing.T) {
	got := defaultOpts(800, 600, false).Padding()
	want := Padding{Top: 20, Right: 20, Bottom: 40, Left: 40}
	if got != want {
		t.Errorf("Padding = %+v, want %+v", got, want)
	}
	gotTitle := defaultOpts(800, 600, true).Padding()
	wantTitle := Padding{Top: 50, Right: 20, Bottom: 40, Left: 40}
	if gotTitle != wantTitle {
		t.Errorf("Padding with title = %+v, want %+v", gotTitle, wantTitle)
	}
}

func TestPrismLayoutPaddingFollowsAxisPlacement(t *testing.T) {
	flipped := AxisPlacement{X: scene.AxisPositionTop, Y: scene.AxisPositionRight}
	got := LayoutOpts{Width: 800, Height: 600, Sides: flipped.Sides()}.Padding()
	want := Padding{Top: 40, Right: 40, Bottom: 20, Left: 20}
	if got != want {
		t.Errorf("flipped Padding = %+v, want %+v", got, want)
	}
}

func TestPrismLayoutNoAxisReservesNothing(t *testing.T) {
	got := LayoutOpts{Width: 800, Height: 600}.Padding()
	want := Padding{Top: 20, Right: 20, Bottom: 20, Left: 20}
	if got != want {
		t.Errorf("bare Padding = %+v, want %+v", got, want)
	}
}

func TestPrismLayoutLegendReservation(t *testing.T) {
	sides := DefaultAxisPlacement().Sides()
	sides.Right.Legend = true
	got := LayoutOpts{Width: 800, Height: 600, Sides: sides}.Padding()
	want := Padding{Top: 20, Right: 20 + layoutLegendReserve, Bottom: 40, Left: 40}
	if got != want {
		t.Errorf("Padding with right legend = %+v, want %+v", got, want)
	}
}

// TestPrismLayoutSparkline pins D067: 4 px all sides, no axis /
// legend / title reservation.
func TestPrismLayoutSparkline(t *testing.T) {
	l := ComputeSparkline(120, 40)
	want := Padding{Top: 4, Right: 4, Bottom: 4, Left: 4}
	if l.Padding != want {
		t.Errorf("sparkline Padding = %+v, want %+v", l.Padding, want)
	}
	if l.Plot.X != 4 || l.Plot.Y != 4 || l.Plot.W != 112 || l.Plot.H != 32 {
		t.Errorf("sparkline Plot = %+v, want {4,4,112,32}", l.Plot)
	}
}
