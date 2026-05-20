package marks

import (
	"testing"
)

func TestPrismSparklineEmitsLine(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"t": []float64{0, 1, 2, 3, 4},
		"v": []float64{10, 20, 15, 30, 25},
	})
	xScale := &linScale{dmin: 0, dmax: 4, rmin: 0, rmax: 100}
	yScale := &linScale{dmin: 10, dmax: 30, rmin: 100, rmax: 0}
	in := Inputs{
		Table:  tbl,
		Layout: plotRect(),
		X:      Channel{Field: "t", Scale: xScale},
		Y:      Channel{Field: "v", Scale: yScale},
	}
	marks, err := encodeSparkline(in)
	if err != nil {
		t.Fatalf("encodeSparkline: %v", err)
	}
	if len(marks) != 1 {
		t.Fatalf("want 1 line mark, got %d", len(marks))
	}
	if marks[0].Line == nil {
		t.Fatalf("expected Line geom, got %+v", marks[0])
	}
	if len(marks[0].Line.Points) != 5 {
		t.Errorf("expected 5 points, got %d", len(marks[0].Line.Points))
	}
}
