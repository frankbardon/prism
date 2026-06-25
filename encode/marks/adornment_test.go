package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// sample series: index 2 is the highest value (smallest y), index 3 the
// lowest value (largest y). Last point is index 4.
func adornSeries() [][2]float64 {
	return [][2]float64{
		{0, 50},
		{10, 40},
		{20, 10}, // high (min y)
		{30, 90}, // low (max y)
		{40, 60}, // last
	}
}

func TestEncodeAdornmentsNothingWhenUnset(t *testing.T) {
	out, err := encodeAdornments(adornSeries(), &linScale{0, 100, 540, 20}, plotRect(), scene.Style{}, Adornments{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil marks when no adornment enabled, got %d", len(out))
	}
}

func TestEncodeAdornmentsNothingWhenEmptySeries(t *testing.T) {
	out, err := encodeAdornments(nil, &linScale{0, 100, 540, 20}, plotRect(), scene.Style{}, Adornments{PointLast: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil marks for empty series, got %d", len(out))
	}
}

func TestEncodeAdornmentsPointLast(t *testing.T) {
	out, err := encodeAdornments(adornSeries(), nil, plotRect(), scene.Style{}, Adornments{PointLast: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 mark, got %d", len(out))
	}
	m := out[0]
	if m.ID != "adornment-last" || m.Type != scene.MarkPoint || m.Point == nil {
		t.Fatalf("unexpected last-point mark: %+v", m)
	}
	if m.Point.Cx != 40 || m.Point.Cy != 60 {
		t.Fatalf("last-point at wrong coords: cx=%v cy=%v", m.Point.Cx, m.Point.Cy)
	}
}

func TestEncodeAdornmentsPointExtent(t *testing.T) {
	out, err := encodeAdornments(adornSeries(), nil, plotRect(), scene.Style{}, Adornments{PointExtent: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 extent marks, got %d", len(out))
	}
	// First is the high-value (smallest y) marker, second the low-value.
	if out[0].ID != "adornment-max" || out[0].Point.Cy != 10 {
		t.Fatalf("max marker wrong: %+v", out[0])
	}
	if out[1].ID != "adornment-min" || out[1].Point.Cy != 90 {
		t.Fatalf("min marker wrong: %+v", out[1])
	}
}

func TestEncodeAdornmentsExtentSinglePoint(t *testing.T) {
	out, err := encodeAdornments([][2]float64{{5, 5}}, nil, plotRect(), scene.Style{}, Adornments{PointExtent: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// High and low collapse to the same index → single marker.
	if len(out) != 1 {
		t.Fatalf("expected 1 mark for single-point extent, got %d", len(out))
	}
	if out[0].ID != "adornment-max" {
		t.Fatalf("expected adornment-max, got %q", out[0].ID)
	}
}

func TestEncodeAdornmentsReferenceBand(t *testing.T) {
	// linScale maps data [0,100] → pixel [540,20] (inverted). from=20
	// → 436, to=40 → 332. top=332, height=104. Band spans plot width.
	ys := &linScale{0, 100, 540, 20}
	plot := plotRect()
	out, err := encodeAdornments(adornSeries(), ys, plot, scene.Style{}, Adornments{
		ReferenceBand: &spec.ReferenceBand{From: 20, To: 40},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 band mark, got %d", len(out))
	}
	m := out[0]
	if m.ID != "adornment-band" || m.Type != scene.MarkRect || m.Rect == nil {
		t.Fatalf("unexpected band mark: %+v", m)
	}
	r := m.Rect
	if r.X != plot.X || r.W != plot.W {
		t.Fatalf("band should span plot width: x=%v w=%v", r.X, r.W)
	}
	if r.Y != 332 || r.H != 104 {
		t.Fatalf("band bounds wrong: y=%v h=%v (want y=332 h=104)", r.Y, r.H)
	}
}

func TestEncodeAdornmentsBandIgnoredWithoutScale(t *testing.T) {
	out, err := encodeAdornments(adornSeries(), nil, plotRect(), scene.Style{}, Adornments{
		ReferenceBand: &spec.ReferenceBand{From: 20, To: 40},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != nil {
		t.Fatalf("band needs a y scale; expected nil, got %d marks", len(out))
	}
}

func TestEncodeAdornmentsCombinedOrder(t *testing.T) {
	ys := &linScale{0, 100, 540, 20}
	out, err := encodeAdornments(adornSeries(), ys, plotRect(), scene.Style{}, Adornments{
		PointLast:     true,
		PointExtent:   true,
		ReferenceBand: &spec.ReferenceBand{From: 20, To: 40},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Band first (behind), then the two extent dots, then last-point.
	wantIDs := []string{"adornment-band", "adornment-max", "adornment-min", "adornment-last"}
	if len(out) != len(wantIDs) {
		t.Fatalf("expected %d marks, got %d", len(wantIDs), len(out))
	}
	for i, want := range wantIDs {
		if out[i].ID != want {
			t.Fatalf("mark %d: want id %q, got %q", i, want, out[i].ID)
		}
	}
}

func TestAdornmentsFromMark(t *testing.T) {
	if (adornmentsFromMark(nil)).enabled() {
		t.Fatal("nil mark should yield no adornments")
	}
	ad := adornmentsFromMark(&spec.MarkDef{PointLast: true, PointExtent: true, ReferenceBand: &spec.ReferenceBand{From: 1, To: 2}})
	if !ad.PointLast || !ad.PointExtent || ad.ReferenceBand == nil {
		t.Fatalf("adornments not carried through: %+v", ad)
	}
}
