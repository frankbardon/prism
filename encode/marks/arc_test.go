package marks

import (
	"math"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeArcPrimitive(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"weight": []float64{1, 2, 3},
	})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "weight"},
		Layout: plotRect(),
		Style:  scene.Style{},
	}
	marks, err := encodeArc(in, "arc")
	if err != nil {
		t.Fatalf("encodeArc: %v", err)
	}
	if len(marks) != 3 {
		t.Fatalf("len marks = %d, want 3", len(marks))
	}
	for i, m := range marks {
		if m.Arc == nil {
			t.Fatalf("mark[%d] has no Arc geom", i)
		}
		if m.Arc.OuterR <= 0 {
			t.Errorf("mark[%d].OuterR = %g, want > 0", i, m.Arc.OuterR)
		}
		if m.Arc.InnerR != 0 {
			t.Errorf("mark[%d].InnerR = %g, want 0 (arc primitive)", i, m.Arc.InnerR)
		}
	}
}

func TestPrismEncodeArcPie(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"value": []float64{42, 31, 27},
	})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "value"},
		Layout: plotRect(),
		Style:  scene.Style{},
	}
	marks, err := encodeArc(in, "pie")
	if err != nil {
		t.Fatalf("encodeArc: %v", err)
	}
	// Sum of slice angular extents must equal 2π exactly.
	sum := 0.0
	for _, m := range marks {
		sum += m.Arc.EndAngle - m.Arc.StartAngle
	}
	if math.Abs(sum-2*math.Pi) > 1e-9 {
		t.Errorf("sum(EndAngle - StartAngle) = %g, want 2π (%g)", sum, 2*math.Pi)
	}
}

func TestPrismEncodeArcDonut(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"value": []float64{1, 2, 3},
	})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "value"},
		Layout: plotRect(),
		Style:  scene.Style{},
	}
	marks, err := encodeArc(in, "donut")
	if err != nil {
		t.Fatalf("encodeArc: %v", err)
	}
	for i, m := range marks {
		if m.Arc.InnerR <= 0 {
			t.Errorf("mark[%d].InnerR = %g, want > 0 for donut", i, m.Arc.InnerR)
		}
		ratio := m.Arc.InnerR / m.Arc.OuterR
		if math.Abs(ratio-0.55) > 1e-9 {
			t.Errorf("mark[%d] InnerR/OuterR = %g, want 0.55", i, ratio)
		}
	}
}

func TestPrismEncodeArcWithColor(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"region": []string{"NA", "EU", "APAC"},
		"value":  []float64{42, 31, 27},
	})
	in := Inputs{
		Table: tbl,
		X:     Channel{Field: "value"},
		Color: &ColorChannel{
			Field:      "region",
			Categories: []string{"NA", "EU", "APAC"},
			Palette: []*scene.Color{
				{R: 1, G: 0, B: 0, A: 0xff},
				{R: 0, G: 1, B: 0, A: 0xff},
				{R: 0, G: 0, B: 1, A: 0xff},
			},
		},
		Layout: plotRect(),
		Style:  scene.Style{},
	}
	marks, err := encodeArc(in, "pie")
	if err != nil {
		t.Fatalf("encodeArc: %v", err)
	}
	if marks[0].Style.Fill == nil || marks[1].Style.Fill == nil {
		t.Fatal("color channel not applied")
	}
	if *marks[0].Style.Fill == *marks[1].Style.Fill {
		t.Errorf("slices should have different colors")
	}
}

func TestPrismEncodeArcRejectsNegative(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"value": []float64{1, -2, 3},
	})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "value"},
		Layout: plotRect(),
	}
	_, err := encodeArc(in, "pie")
	if err == nil {
		t.Fatal("expected error for negative theta value")
	}
}

func TestPrismEncodeArcRejectsZeroSum(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"value": []float64{0, 0, 0},
	})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "value"},
		Layout: plotRect(),
	}
	_, err := encodeArc(in, "pie")
	if err == nil {
		t.Fatal("expected error for zero-sum theta values")
	}
}
