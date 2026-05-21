package marks

import (
	"sort"
	"testing"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/table"
)

func TestSkipNullRowsNoNulls(t *testing.T) {
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "region", Type: encoding.FieldTypeCategoricalU8},
		{Name: "score", Type: encoding.FieldTypeF64},
	}}
	cols := map[string]table.Column{
		"region": table.StringColumn{"west", "east", "north"},
		"score":  table.FloatColumn{0.5, 0.6, 0.7},
	}
	tbl, err := table.NewTable(schema, cols, 3, "xxh64:0000000000000000")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	kept, dropped, offending := SkipNullRows(tbl, "region", "score")
	if dropped != 0 || len(offending) != 0 {
		t.Errorf("expected zero drops, got dropped=%d offending=%v", dropped, offending)
	}
	if len(kept) != 3 {
		t.Errorf("kept = %d, want 3", len(kept))
	}
}

func TestSkipNullRowsDropsAndReports(t *testing.T) {
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "region", Type: encoding.FieldTypeCategoricalU8, Nullable: true},
		{Name: "score", Type: encoding.FieldTypeF64, Nullable: true},
	}}
	regionNulls := table.NewNullBitmap(3)
	regionNulls.Set(1)
	scoreNulls := table.NewNullBitmap(3)
	scoreNulls.Set(2)
	cols := map[string]table.Column{
		"region": table.NullableColumn{Inner: table.StringColumn{"west", "", "north"}, Nulls: regionNulls},
		"score":  table.NullableColumn{Inner: table.FloatColumn{0.5, 0.6, 0}, Nulls: scoreNulls},
	}
	tbl, err := table.NewTable(schema, cols, 3, "xxh64:0000000000000001")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	kept, dropped, offending := SkipNullRows(tbl, "region", "score")
	if dropped != 2 {
		t.Errorf("dropped = %d, want 2", dropped)
	}
	if len(kept) != 1 || kept[0] != 0 {
		t.Errorf("kept = %v, want [0]", kept)
	}
	sort.Strings(offending)
	if len(offending) != 2 || offending[0] != "region" || offending[1] != "score" {
		t.Errorf("offending = %v, want [region score]", offending)
	}
}
