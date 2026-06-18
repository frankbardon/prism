package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeHeatmapCategorical(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"region": []string{"NA", "NA", "EU", "EU"},
		"bucket": []string{"0-25", "26-50", "0-25", "26-50"},
		"count":  []float64{10, 18, 7, 22},
	})
	xBand := &bandScaleT{cats: []string{"EU", "NA"}, rmin: 0, rmax: 400, padding: 0.1}
	yBand := &bandScaleT{cats: []string{"0-25", "26-50"}, rmin: 0, rmax: 300, padding: 0.1}
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "region", Scale: xBand},
		Y:      Channel{Field: "bucket", Scale: yBand},
		Color:  &ColorChannel{Field: "count"},
		Layout: scene.Rect{W: 400, H: 300},
		Style:  scene.Style{},
	}
	marks, err := encodeHeatmap(in)
	if err != nil {
		t.Fatalf("encodeHeatmap: %v", err)
	}
	if len(marks) != 4 {
		t.Fatalf("len marks = %d, want 4", len(marks))
	}
	// All marks should be Rect and have a Fill set (color via gradient).
	for i, m := range marks {
		if m.Rect == nil {
			t.Errorf("mark[%d] not Rect", i)
		}
		if m.Style.Fill == nil {
			t.Errorf("mark[%d] missing fill color", i)
		}
	}
	// Different counts → different fills.
	if *marks[0].Style.Fill == *marks[3].Style.Fill {
		t.Errorf("min count (10) and max count (22) cells should have different colors")
	}
}

func TestPrismEncodeHeatmapOpacityChannel(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"region": []string{"NA", "NA", "EU", "EU"},
		"bucket": []string{"0-25", "26-50", "0-25", "26-50"},
		"count":  []float64{10, 18, 7, 22},
		"z":      []float64{0, 1, 2, 4},
	})
	xBand := &bandScaleT{cats: []string{"EU", "NA"}, rmin: 0, rmax: 400, padding: 0.1}
	yBand := &bandScaleT{cats: []string{"0-25", "26-50"}, rmin: 0, rmax: 300, padding: 0.1}
	in := Inputs{
		Table:   tbl,
		X:       Channel{Field: "region", Scale: xBand},
		Y:       Channel{Field: "bucket", Scale: yBand},
		Color:   &ColorChannel{Field: "count"},
		Opacity: &OpacityChannel{Field: "z"},
		Layout:  scene.Rect{W: 400, H: 300},
		Style:   scene.Style{},
	}
	marks, err := encodeHeatmap(in)
	if err != nil {
		t.Fatalf("encodeHeatmap: %v", err)
	}
	if len(marks) != 4 {
		t.Fatalf("len marks = %d, want 4", len(marks))
	}
	// z=0 (min) maps to the floor; z=4 (max) maps to full opacity.
	if got := marks[0].Style.Opacity; got != OpacityFloor {
		t.Errorf("min-z cell opacity = %v, want floor %v", got, OpacityFloor)
	}
	if got := marks[3].Style.Opacity; got != 1.0 {
		t.Errorf("max-z cell opacity = %v, want 1.0", got)
	}
	// Monotonic in z across the four cells (0,1,2,4).
	for i := 1; i < len(marks); i++ {
		if marks[i].Style.Opacity < marks[i-1].Style.Opacity {
			t.Errorf("opacity not monotonic with z at cell %d (%v < %v)", i, marks[i].Style.Opacity, marks[i-1].Style.Opacity)
		}
	}
}

func TestPrismEncodeHeatmapWithoutColor(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"region": []string{"NA", "EU"},
		"bucket": []string{"x", "y"},
	})
	xBand := &bandScaleT{cats: []string{"EU", "NA"}, rmin: 0, rmax: 100, padding: 0}
	yBand := &bandScaleT{cats: []string{"x", "y"}, rmin: 0, rmax: 100, padding: 0}
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "region", Scale: xBand},
		Y:      Channel{Field: "bucket", Scale: yBand},
		Layout: scene.Rect{W: 100, H: 100},
		Style:  scene.Style{},
	}
	marks, err := encodeHeatmap(in)
	if err != nil {
		t.Fatalf("encodeHeatmap: %v", err)
	}
	if len(marks) != 2 {
		t.Fatalf("len marks = %d, want 2", len(marks))
	}
}

func TestPrismEncodeHeatmapRejectsContinuousScale(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"x": []float64{1, 2},
		"y": []float64{3, 4},
	})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "x", Scale: &linScale{0, 10, 0, 100}},
		Y:      Channel{Field: "y", Scale: &linScale{0, 10, 0, 100}},
		Layout: scene.Rect{W: 100, H: 100},
	}
	_, err := encodeHeatmap(in)
	if err == nil {
		t.Fatal("expected error for non-band scale on heatmap")
	}
}
