package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// bulletMarkDef builds a bullet mark-def with the supplied orientation.
func bulletMarkDef(orientation string) *spec.MarkDef {
	return &spec.MarkDef{
		Orientation: orientation,
		Bands:       []float64{150, 225, 300},
		Comparative: float64(220),
		Target:      float64(260),
	}
}

// assertBulletLayers checks the ordered scene marks a bullet emits:
// band rects (one per bound) → measure bar → comparative bar → target
// rule. Returns the measure / comparative / target marks for further
// geometry assertions.
func assertBulletLayers(t *testing.T, marks []scene.Mark) (measure, comparative, target scene.Mark) {
	t.Helper()
	// 3 bands + measure + comparative + target = 6 marks.
	if len(marks) != 6 {
		t.Fatalf("len(marks) = %d, want 6", len(marks))
	}
	for i := 0; i < 3; i++ {
		if marks[i].Type != scene.MarkRect || marks[i].Rect == nil {
			t.Fatalf("marks[%d] expected band rect, got %s", i, marks[i].Type)
		}
		if marks[i].ID == "" {
			t.Errorf("band mark %d missing ID", i)
		}
	}
	measure, comparative, target = marks[3], marks[4], marks[5]
	if measure.ID != "bullet-measure" || measure.Rect == nil {
		t.Fatalf("marks[3] expected measure bar, got id=%q type=%s", measure.ID, measure.Type)
	}
	if comparative.ID != "bullet-comparative" || comparative.Rect == nil {
		t.Fatalf("marks[4] expected comparative bar, got id=%q type=%s", comparative.ID, comparative.Type)
	}
	if target.ID != "bullet-target" || target.Rule == nil {
		t.Fatalf("marks[5] expected target rule, got id=%q type=%s", target.ID, target.Type)
	}
	return measure, comparative, target
}

func TestPrismEncodeBulletHorizontal(t *testing.T) {
	tbl := buildTable(t, map[string]any{"revenue": []float64{270}})
	plot := plotRect()
	// Horizontal: x is the quantitative measure axis (value → x pixel).
	xs := &linScale{dmin: 0, dmax: 300, rmin: plot.X, rmax: plot.Right()}
	marks, _, err := Encode("bullet", Inputs{
		Table:  tbl,
		X:      Channel{Field: "revenue", Scale: xs},
		Layout: plot,
		Style:  scene.Style{Fill: &scene.Color{R: 0x33, G: 0x66, B: 0x99, A: 255}},
		Mark:   bulletMarkDef("horizontal"),
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	measure, comparative, target := assertBulletLayers(t, marks)

	// Comparative bar must be visibly thinner than the measure bar.
	if comparative.Rect.H >= measure.Rect.H {
		t.Errorf("comparative H=%g should be < measure H=%g", comparative.Rect.H, measure.Rect.H)
	}
	// Measure bar grows along x from the value-0 baseline.
	base, _ := xs.Apply(float64(0))
	if measure.Rect.X != base {
		t.Errorf("measure bar X=%g, want baseline %g", measure.Rect.X, base)
	}
	mPix, _ := xs.Apply(float64(270))
	if got := measure.Rect.X + measure.Rect.W; got != mPix {
		t.Errorf("measure bar end X=%g, want %g", got, mPix)
	}
	// Target is a vertical tick for a horizontal bullet.
	if target.Rule.X1 != target.Rule.X2 {
		t.Errorf("horizontal target tick should be vertical: X1=%g X2=%g", target.Rule.X1, target.Rule.X2)
	}
	tPix, _ := xs.Apply(float64(260))
	if target.Rule.X1 != tPix {
		t.Errorf("target tick X=%g, want %g", target.Rule.X1, tPix)
	}
}

func TestPrismEncodeBulletVertical(t *testing.T) {
	tbl := buildTable(t, map[string]any{"revenue": []float64{270}})
	plot := plotRect()
	// Vertical: y is the quantitative measure axis, inverted (0 at bottom).
	ys := &linScale{dmin: 0, dmax: 300, rmin: plot.Bottom(), rmax: plot.Y}
	marks, _, err := Encode("bullet", Inputs{
		Table:  tbl,
		Y:      Channel{Field: "revenue", Scale: ys},
		Layout: plot,
		Style:  scene.Style{Fill: &scene.Color{R: 0x33, G: 0x66, B: 0x99, A: 255}},
		Mark:   bulletMarkDef("vertical"),
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	measure, comparative, target := assertBulletLayers(t, marks)

	// Comparative bar must be visibly thinner (narrower along x) than the
	// measure bar in vertical orientation.
	if comparative.Rect.W >= measure.Rect.W {
		t.Errorf("comparative W=%g should be < measure W=%g", comparative.Rect.W, measure.Rect.W)
	}
	// Measure bar grows up from the bottom baseline to the value pixel.
	base, _ := ys.Apply(float64(0))
	mPix, _ := ys.Apply(float64(270))
	if measure.Rect.Y != mPix {
		t.Errorf("measure bar top Y=%g, want %g", measure.Rect.Y, mPix)
	}
	if got := measure.Rect.Y + measure.Rect.H; got != base {
		t.Errorf("measure bar bottom Y=%g, want baseline %g", got, base)
	}
	// Target is a horizontal tick for a vertical bullet.
	if target.Rule.Y1 != target.Rule.Y2 {
		t.Errorf("vertical target tick should be horizontal: Y1=%g Y2=%g", target.Rule.Y1, target.Rule.Y2)
	}
}

func TestPrismEncodeBulletTargetFieldRef(t *testing.T) {
	// target as a string resolves to a data field read from row 0.
	tbl := buildTable(t, map[string]any{
		"revenue": []float64{270},
		"goal":    []float64{255},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 300, rmin: plot.X, rmax: plot.Right()}
	md := &spec.MarkDef{Bands: []float64{300}, Target: "goal"}
	marks, _, err := Encode("bullet", Inputs{
		Table:  tbl,
		X:      Channel{Field: "revenue", Scale: xs},
		Layout: plot,
		Mark:   md,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// 1 band + measure + target (no comparative) = 3 marks.
	if len(marks) != 3 {
		t.Fatalf("len(marks) = %d, want 3", len(marks))
	}
	target := marks[2]
	if target.ID != "bullet-target" || target.Rule == nil {
		t.Fatalf("expected target rule, got id=%q", target.ID)
	}
	want, _ := xs.Apply(float64(255))
	if target.Rule.X1 != want {
		t.Errorf("field-ref target X=%g, want %g (from goal=255)", target.Rule.X1, want)
	}
}
