package encode

import "testing"

func TestPrismLayoutDefaults(t *testing.T) {
	l := Compute(800, 600, false)
	if l.Frame.W != 800 || l.Frame.H != 600 {
		t.Errorf("Frame = %+v, want {0,0,800,600}", l.Frame)
	}
	if l.Plot.X != 40 || l.Plot.Y != 20 || l.Plot.W != 740 || l.Plot.H != 540 {
		t.Errorf("Plot = %+v, want {40,20,740,540}", l.Plot)
	}
}

func TestPrismLayoutWithTitle(t *testing.T) {
	l := Compute(800, 600, true)
	if l.Plot.Y != 50 || l.Plot.H != 510 {
		t.Errorf("Plot with title = %+v, want Y=50 H=510", l.Plot)
	}
}

func TestPrismLayoutCustomDimensions(t *testing.T) {
	l := Compute(1200, 400, false)
	if l.Plot.W != 1140 || l.Plot.H != 340 {
		t.Errorf("Plot = %+v, want W=1140 H=340", l.Plot)
	}
}
