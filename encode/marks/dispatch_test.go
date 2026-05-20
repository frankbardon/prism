package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeDispatchUnimplementedWarns(t *testing.T) {
	// P11 implemented path/image/sankey alongside the P10 composite
	// set; the warn-and-skip path covers funnel/sparkline until they
	// land later in P11. Use "funnel" as the canary.
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.4},
	})
	plot := plotRect()
	marks, warn, err := Encode("funnel", Inputs{Table: tbl, Layout: plot})
	if err != nil {
		t.Fatalf("Encode(funnel): %v", err)
	}
	if len(marks) != 0 {
		t.Errorf("marks for unsupported type = %d, want 0", len(marks))
	}
	if warn == nil || warn.Code != scene.WarnMarkNotImplemented {
		t.Fatalf("expected WarnMarkNotImplemented, got %+v", warn)
	}
}

func TestPrismEncodeDispatchUnknownErrors(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.4},
	})
	plot := plotRect()
	_, _, err := Encode("totally-bogus", Inputs{Table: tbl, Layout: plot})
	if err == nil {
		t.Fatal("expected error for unknown mark type, got nil")
	}
}
