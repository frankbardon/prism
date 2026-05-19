package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeRuleHorizontal(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.4, 0.55, 0.7},
	})
	plot := plotRect()
	ys := &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}
	marks, _, err := Encode("rule", Inputs{
		Table:  tbl,
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
		if m.Type != scene.MarkRule || m.Rule == nil {
			t.Errorf("marks[%d] not a Rule: type=%s rule=%v", i, m.Type, m.Rule)
		}
		// Y1 == Y2 (horizontal); X1=plot.X, X2=plot.Right().
		if m.Rule.X1 != plot.X || m.Rule.X2 != plot.Right() {
			t.Errorf("marks[%d] rule X range = (%g,%g), want (%g,%g)", i, m.Rule.X1, m.Rule.X2, plot.X, plot.Right())
		}
		if m.Rule.Y1 != m.Rule.Y2 {
			t.Errorf("marks[%d] not horizontal: Y1=%g Y2=%g", i, m.Rule.Y1, m.Rule.Y2)
		}
	}
}

func TestPrismEncodeRuleNoBinding(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.4, 0.55},
	})
	plot := plotRect()
	_, _, err := Encode("rule", Inputs{
		Table:  tbl,
		Layout: plot,
	})
	if err == nil {
		t.Fatal("expected error for rule with no x/y bound, got nil")
	}
}
