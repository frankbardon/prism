package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeDispatchUnimplementedWarns(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.4},
	})
	plot := plotRect()
	marks, warn, err := Encode("arc", Inputs{Table: tbl, Layout: plot})
	if err != nil {
		t.Fatalf("Encode(arc): %v", err)
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
