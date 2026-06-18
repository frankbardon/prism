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
