package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodePointBasic(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"latency_ms": []float64{120, 220, 300},
		"score":      []float64{0.81, 0.42, 0.30},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 300, rmin: plot.X, rmax: plot.Right()}
	ys := &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}
	marks, _, err := Encode("point", Inputs{
		Table:  tbl,
		X:      Channel{Field: "latency_ms", Scale: xs},
		Y:      Channel{Field: "score", Scale: ys},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 3 {
		t.Fatalf("len(marks) = %d, want 3", len(marks))
	}
	for i, m := range marks {
		if m.Type != scene.MarkPoint || m.Point == nil {
			t.Errorf("marks[%d] not a Point: type=%s point=%v", i, m.Type, m.Point)
		}
		if m.Point.R != 4 {
			t.Errorf("marks[%d].R = %g, want 4", i, m.Point.R)
		}
		if m.Point.Shape != scene.ShapeCircle {
			t.Errorf("marks[%d].Shape = %q, want circle", i, m.Point.Shape)
		}
	}
}

func TestPrismEncodePointWithColor(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"latency_ms": []float64{120, 220, 300},
		"score":      []float64{0.81, 0.42, 0.30},
		"group":      []string{"a", "b", "a"},
	})
	plot := plotRect()
	xs := &linScale{dmin: 0, dmax: 300, rmin: plot.X, rmax: plot.Right()}
	ys := &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}
	palette := []*scene.Color{
		mustColor("#3b82f6"),
		mustColor("#ef4444"),
	}
	marks, _, err := Encode("point", Inputs{
		Table:  tbl,
		X:      Channel{Field: "latency_ms", Scale: xs},
		Y:      Channel{Field: "score", Scale: ys},
		Layout: plot,
		Color: &ColorChannel{
			Field:      "group",
			Categories: []string{"a", "b"},
			Palette:    palette,
		},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if marks[0].Style.Fill == nil || marks[0].Style.Fill.Hex() != "#3b82f6" {
		t.Errorf("marks[0].Style.Fill = %v, want #3b82f6", marks[0].Style.Fill)
	}
	if marks[1].Style.Fill == nil || marks[1].Style.Fill.Hex() != "#ef4444" {
		t.Errorf("marks[1].Style.Fill = %v, want #ef4444", marks[1].Style.Fill)
	}
	if marks[2].Style.Fill == nil || marks[2].Style.Fill.Hex() != "#3b82f6" {
		t.Errorf("marks[2].Style.Fill = %v, want #3b82f6 (same group as [0])", marks[2].Style.Fill)
	}
}

func mustColor(hex string) *scene.Color {
	c, err := scene.ColorFromHex(hex)
	if err != nil {
		panic(err)
	}
	return c
}
