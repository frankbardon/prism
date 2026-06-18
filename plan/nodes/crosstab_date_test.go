package nodes

import (
	"encoding/json"
	"testing"

	pulsetypes "github.com/frankbardon/pulse/types"

	"github.com/frankbardon/prism/spec"
)

// TestBuildCrosstabRequestDateGrouper verifies that a date grouper is
// translated to GROUP_DATE with the period folded into Params, that an
// empty period defaults to month, and that an invalid period is
// rejected. Category groupers stay GROUP_CATEGORY with no Params.
func TestBuildCrosstabRequestDateGrouper(t *testing.T) {
	body := spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "region"}},
		Columns: []spec.CrosstabGroup{{Field: "order_date", Type: "date", Period: "quarter"}},
		Cell:    spec.CrosstabCell{Aggregate: "sum", Field: "revenue", As: "rev"},
	}
	req, err := buildCrosstabRequest("sales.pulse", body, "rev")
	if err != nil {
		t.Fatalf("buildCrosstabRequest: %v", err)
	}

	if len(req.Crosstab.Rows) != 1 || req.Crosstab.Rows[0].Type != pulsetypes.GROUP_CATEGORY {
		t.Fatalf("row grouper = %+v, want GROUP_CATEGORY", req.Crosstab.Rows)
	}
	if req.Crosstab.Rows[0].Params != nil {
		t.Errorf("category grouper should carry no Params, got %s", req.Crosstab.Rows[0].Params)
	}

	col := req.Crosstab.Columns[0]
	if col.Type != pulsetypes.GROUP_DATE {
		t.Fatalf("column grouper type = %q, want GROUP_DATE", col.Type)
	}
	var got map[string]string
	if err := json.Unmarshal(col.Params, &got); err != nil {
		t.Fatalf("date grouper Params not valid JSON: %v", err)
	}
	if got["component"] != "quarter" {
		t.Errorf("date grouper component = %q, want quarter", got["component"])
	}
}

func TestBuildCrosstabRequestDateDefaultPeriod(t *testing.T) {
	body := spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "d", Type: "date"}},
		Columns: []spec.CrosstabGroup{{Field: "region"}},
		Cell:    spec.CrosstabCell{Aggregate: "count"},
	}
	req, err := buildCrosstabRequest("c.pulse", body, "n")
	if err != nil {
		t.Fatalf("buildCrosstabRequest: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(req.Crosstab.Rows[0].Params, &got); err != nil {
		t.Fatalf("Params: %v", err)
	}
	if got["component"] != "month" {
		t.Errorf("default period component = %q, want month", got["component"])
	}
}

func TestBuildCrosstabRequestBadPeriod(t *testing.T) {
	body := spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "d", Type: "date", Period: "fortnight"}},
		Columns: []spec.CrosstabGroup{{Field: "region"}},
		Cell:    spec.CrosstabCell{Aggregate: "count"},
	}
	if _, err := buildCrosstabRequest("c.pulse", body, "n"); err == nil {
		t.Fatal("expected error for invalid date period, got nil")
	}
}

// TestBuildCrosstabRequestOverlays verifies overlay translation: the
// request switches to matrix shape, forces the margin each overlay
// needs as a denominator, and sets Ref.Margin.Axis correctly (fixed by
// kind for share_of_*, user-supplied for index_vs_margin).
func TestBuildCrosstabRequestOverlays(t *testing.T) {
	body := spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "region"}},
		Columns: []spec.CrosstabGroup{{Field: "quarter"}},
		Cell:    spec.CrosstabCell{Aggregate: "sum", Field: "rev", As: "rev"},
		Overlays: []spec.CrosstabOverlay{
			{Kind: "share_of_row", As: "rs"},
			{Kind: "index_vs_margin", Axis: "column", As: "idx"},
		},
	}
	req, err := buildCrosstabRequest("s.pulse", body, "rev")
	if err != nil {
		t.Fatalf("buildCrosstabRequest: %v", err)
	}
	if req.Crosstab.Shape != pulsetypes.CrosstabShapeMatrix {
		t.Errorf("shape = %q, want matrix (overlays force matrix)", req.Crosstab.Shape)
	}
	if !req.Crosstab.Margins.Rows || !req.Crosstab.Margins.Columns {
		t.Errorf("margins = %+v, want both row+column forced", req.Crosstab.Margins)
	}
	if len(req.Overlays) != 2 {
		t.Fatalf("overlays = %d, want 2", len(req.Overlays))
	}
	if req.Overlays[0].Kind != pulsetypes.OverlayKindShareOfRow ||
		req.Overlays[0].Ref.Margin == nil ||
		req.Overlays[0].Ref.Margin.Axis != pulsetypes.MarginAxisRow {
		t.Errorf("overlay[0] = %+v, want SHARE_OF_ROW with row margin ref", req.Overlays[0])
	}
	if req.Overlays[1].Kind != pulsetypes.OverlayKindIndexVsMargin ||
		req.Overlays[1].Ref.Margin == nil ||
		req.Overlays[1].Ref.Margin.Axis != pulsetypes.MarginAxisColumn {
		t.Errorf("overlay[1] = %+v, want INDEX_VS_MARGIN with column margin ref", req.Overlays[1])
	}
}

func TestBuildCrosstabRequestBadOverlay(t *testing.T) {
	// Unknown kind.
	bad := spec.CrosstabBody{
		Rows:     []spec.CrosstabGroup{{Field: "r"}},
		Columns:  []spec.CrosstabGroup{{Field: "c"}},
		Cell:     spec.CrosstabCell{Aggregate: "count", Field: "x"},
		Overlays: []spec.CrosstabOverlay{{Kind: "bogus"}},
	}
	if _, err := buildCrosstabRequest("s.pulse", bad, "x"); err == nil {
		t.Fatal("expected error for unknown overlay kind")
	}
	// index_vs_margin without axis.
	noAxis := spec.CrosstabBody{
		Rows:     []spec.CrosstabGroup{{Field: "r"}},
		Columns:  []spec.CrosstabGroup{{Field: "c"}},
		Cell:     spec.CrosstabCell{Aggregate: "count", Field: "x"},
		Overlays: []spec.CrosstabOverlay{{Kind: "index_vs_margin"}},
	}
	if _, err := buildCrosstabRequest("s.pulse", noAxis, "x"); err == nil {
		t.Fatal("expected error for index_vs_margin without axis")
	}
}
