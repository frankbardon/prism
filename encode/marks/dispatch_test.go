package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismEncodeDispatchAllImplemented(t *testing.T) {
	// P11 implemented sankey/funnel/sparkline/image/path so the
	// warn-and-skip list is now empty. Any unknown mark type
	// errors (see TestPrismEncodeDispatchUnknownErrors below).
	// Keep this slot for future warn-and-skip flow if a later
	// phase introduces a deferred mark.
	_ = scene.WarnMarkNotImplemented
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
