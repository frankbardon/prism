package marks

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// idsOf returns the IDs of the adornment marks (those whose ID is
// prefixed "adornment-") in a spark encoder's output.
func idsOf(marks []scene.Mark) []string {
	var ids []string
	for _, m := range marks {
		if strings.HasPrefix(m.ID, "adornment-") {
			ids = append(ids, m.ID)
		}
	}
	return ids
}

func assertHasAll(t *testing.T, mark string, got []string, want ...string) {
	t.Helper()
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s: missing adornment %q (got %v)", mark, w, got)
		}
	}
}

// TestSparkAdornmentsDefaultOff confirms that, with no adornment field
// set, each spark encoder emits only its base geometry — no adornment
// marks leak in, preserving byte-identical output.
func TestSparkAdornmentsDefaultOff(t *testing.T) {
	yLin := &linScale{dmin: 0, dmax: 30, rmin: 540, rmax: 20}
	xLin := &linScale{dmin: 0, dmax: 4, rmin: 0, rmax: 100}
	lineTbl := buildTable(t, map[string]any{
		"t": []float64{0, 1, 2, 3, 4},
		"v": []float64{10, 20, 15, 30, 25},
	})
	baseIn := Inputs{
		Table:  lineTbl,
		Layout: plotRect(),
		X:      Channel{Field: "t", Scale: xLin},
		Y:      Channel{Field: "v", Scale: yLin},
	}

	if got, err := encodeSparkline(baseIn); err != nil {
		t.Fatalf("encodeSparkline: %v", err)
	} else if ids := idsOf(got); len(ids) != 0 {
		t.Errorf("sparkline default-off leaked adornments: %v", ids)
	}
	if got, err := encodeSparkarea(baseIn); err != nil {
		t.Fatalf("encodeSparkarea: %v", err)
	} else if ids := idsOf(got); len(ids) != 0 {
		t.Errorf("sparkarea default-off leaked adornments: %v", ids)
	}

	barTbl := buildTable(t, map[string]any{
		"t": []string{"a", "b", "c", "d", "e"},
		"v": []float64{10, 20, 15, 30, 25},
	})
	xBand := &bandScaleT{cats: []string{"a", "b", "c", "d", "e"}, rmin: 0, rmax: 100, padding: 0.1}
	barIn := Inputs{
		Table:  barTbl,
		Layout: plotRect(),
		X:      Channel{Field: "t", Scale: xBand},
		Y:      Channel{Field: "v", Scale: yLin},
	}
	if got, err := encodeSparkbar(barIn); err != nil {
		t.Fatalf("encodeSparkbar: %v", err)
	} else if ids := idsOf(got); len(ids) != 0 {
		t.Errorf("sparkbar default-off leaked adornments: %v", ids)
	}
}

// TestSparkAdornmentsWiredThrough confirms each opt-in field surfaces
// the matching adornment marks appended after the base geometry, across
// all three spark families.
func TestSparkAdornmentsWiredThrough(t *testing.T) {
	yLin := &linScale{dmin: 0, dmax: 30, rmin: 540, rmax: 20}
	xLin := &linScale{dmin: 0, dmax: 4, rmin: 0, rmax: 100}
	lineTbl := buildTable(t, map[string]any{
		"t": []float64{0, 1, 2, 3, 4},
		"v": []float64{10, 20, 15, 30, 25},
	})

	// sparkline + point_last + point_extent
	lineIn := Inputs{
		Table:  lineTbl,
		Layout: plotRect(),
		X:      Channel{Field: "t", Scale: xLin},
		Y:      Channel{Field: "v", Scale: yLin},
		Mark:   &spec.MarkDef{PointLast: true, PointExtent: true},
	}
	got, err := encodeSparkline(lineIn)
	if err != nil {
		t.Fatalf("encodeSparkline: %v", err)
	}
	assertHasAll(t, "sparkline", idsOf(got), "adornment-max", "adornment-min", "adornment-last")
	if got[0].Line == nil {
		t.Errorf("sparkline base geometry should precede adornments, got %+v", got[0])
	}

	// sparkbar + point_extent (band x is centred on the column)
	barTbl := buildTable(t, map[string]any{
		"t": []string{"a", "b", "c", "d", "e"},
		"v": []float64{10, 20, 15, 30, 25},
	})
	xBand := &bandScaleT{cats: []string{"a", "b", "c", "d", "e"}, rmin: 0, rmax: 100, padding: 0.1}
	barIn := Inputs{
		Table:  barTbl,
		Layout: plotRect(),
		X:      Channel{Field: "t", Scale: xBand},
		Y:      Channel{Field: "v", Scale: yLin},
		Mark:   &spec.MarkDef{PointExtent: true},
	}
	gotBar, err := encodeSparkbar(barIn)
	if err != nil {
		t.Fatalf("encodeSparkbar: %v", err)
	}
	assertHasAll(t, "sparkbar", idsOf(gotBar), "adornment-max", "adornment-min")
	// Band-x centring: the max dot (value 30, index "d") sits at the
	// column centre = band start + bandwidth/2.
	bw := xBand.BandWidth()
	start, _ := xBand.Apply("d")
	wantCx := roundTo(start+bw/2, 3)
	for _, m := range gotBar {
		if m.ID == "adornment-max" && m.Point != nil && m.Point.Cx != wantCx {
			t.Errorf("sparkbar extent dot not centred: cx=%v want %v", m.Point.Cx, wantCx)
		}
	}

	// sparkarea + reference_band + point_last
	areaIn := Inputs{
		Table:  lineTbl,
		Layout: plotRect(),
		X:      Channel{Field: "t", Scale: xLin},
		Y:      Channel{Field: "v", Scale: yLin},
		Mark:   &spec.MarkDef{PointLast: true, ReferenceBand: &spec.ReferenceBand{From: 15, To: 22}},
	}
	gotArea, err := encodeSparkarea(areaIn)
	if err != nil {
		t.Fatalf("encodeSparkarea: %v", err)
	}
	assertHasAll(t, "sparkarea", idsOf(gotArea), "adornment-band", "adornment-last")
}
