package spec

import (
	"encoding/json"
	"testing"
)

// TestTableMarkDefRoundTrip asserts a table mark_def carrying
// page_size decodes without loss and re-encodes to an equivalent
// shape. page_size is optional — its absence means "apply the
// TablePageSizeDefault at encode time" (E1-S3), not zero.
func TestTableMarkDefRoundTrip(t *testing.T) {
	const in = `{"type": "table", "page_size": 50}`

	var m Mark
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Def == nil {
		t.Fatalf("expected mark def, got shorthand %q", m.Shorthand)
	}
	def := m.Def
	if def.Type != "table" {
		t.Errorf("type = %q, want table", def.Type)
	}
	if def.PageSize == nil || *def.PageSize != 50 {
		t.Errorf("page_size = %v, want 50", def.PageSize)
	}

	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var rt Mark
	if err := json.Unmarshal(out, &rt); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if rt.Def == nil || rt.Def.PageSize == nil || *rt.Def.PageSize != 50 {
		t.Fatalf("round-trip page_size lost or wrong: %+v", rt.Def)
	}
}

// TestTableMarkDefPageSizeUnset asserts an unset page_size decodes as
// nil (distinct from an explicit zero), matching the tri-state
// pointer convention used throughout MarkDef.
func TestTableMarkDefPageSizeUnset(t *testing.T) {
	var m Mark
	if err := json.Unmarshal([]byte(`{"type": "table"}`), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Def == nil {
		t.Fatalf("expected mark def, got shorthand %q", m.Shorthand)
	}
	if m.Def.PageSize != nil {
		t.Errorf("page_size = %v, want nil (unset)", m.Def.PageSize)
	}
}

// TestTableColumnsRoundTrip asserts encoding.columns[] decodes each
// column's standard channel fields plus the optional sub-mark, and
// re-encodes without loss.
func TestTableColumnsRoundTrip(t *testing.T) {
	const in = `{
		"columns": [
			{"field": "name", "type": "nominal", "title": "Name"},
			{"field": "trend", "type": "quantitative", "aggregate": "mean", "mark": "sparkline"}
		]
	}`

	var enc Encoding
	if err := json.Unmarshal([]byte(in), &enc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(enc.Columns) != 2 {
		t.Fatalf("columns len = %d, want 2", len(enc.Columns))
	}
	c0, c1 := enc.Columns[0], enc.Columns[1]
	if c0.Field != "name" || c0.Type != "nominal" || c0.Title != "Name" {
		t.Errorf("columns[0] = %+v, want field=name type=nominal title=Name", c0)
	}
	if c0.Mark != "" {
		t.Errorf("columns[0].Mark = %q, want empty (formatted text default)", c0.Mark)
	}
	if c1.Field != "trend" || c1.Type != "quantitative" || c1.Aggregate != "mean" || c1.Mark != "sparkline" {
		t.Errorf("columns[1] = %+v, want field=trend type=quantitative aggregate=mean mark=sparkline", c1)
	}

	out, err := json.Marshal(enc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var rt Encoding
	if err := json.Unmarshal(out, &rt); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if len(rt.Columns) != 2 || rt.Columns[1].Mark != "sparkline" {
		t.Fatalf("round-trip columns lost or wrong: %+v", rt.Columns)
	}
}
