package encode

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// TestBulletMeasureExtras locks the clip fix: a bullet's bands, literal
// target / comparative, and field-name references all widen the measure
// axis so nothing past the data range gets clipped.
func TestBulletMeasureExtras(t *testing.T) {
	tbl, _, err := table.FromInline("kpi", []map[string]any{
		{"actual": 4.2, "prior": 3.6, "quota": 5.0},
	}, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}

	def := &spec.MarkDef{
		Type:        "bullet",
		Bands:       []float64{2.5, 4, 6},
		Comparative: "prior",
		Target:      "quota",
	}
	got := bulletMeasureExtras(def, tbl)
	// bands (3) + target field row0 (quota=5.0) + comparative field row0
	// (prior=3.6) = 5 values, in that order.
	want := []float64{2.5, 4, 6, 5.0, 3.6}
	if len(got) != len(want) {
		t.Fatalf("extras len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i, w := range want {
		f, ok := got[i].(float64)
		if !ok || f != w {
			t.Errorf("extras[%d] = %v, want %v", i, got[i], w)
		}
	}
}

// TestBulletMeasureExtrasLiteral covers literal numeric target /
// comparative (no field lookup).
func TestBulletMeasureExtrasLiteral(t *testing.T) {
	tbl, _, err := table.FromInline("kpi", []map[string]any{{"actual": 275.0}}, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	def := &spec.MarkDef{Type: "bullet", Bands: []float64{150, 300}, Target: 260.0, Comparative: 240.0}
	got := bulletMeasureExtras(def, tbl)
	if len(got) != 4 {
		t.Fatalf("extras = %v, want 4 values", got)
	}
}
