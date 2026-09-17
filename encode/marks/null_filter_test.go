package marks

import (
	"errors"
	"sort"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/table"
)

func TestSkipNullRowsNoNulls(t *testing.T) {
	schema := &table.Schema{Fields: []table.Field{
		{Name: "region", Type: table.FieldTypeCategoricalU8},
		{Name: "score", Type: table.FieldTypeF64},
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
	schema := &table.Schema{Fields: []table.Field{
		{Name: "region", Type: table.FieldTypeCategoricalU8, Nullable: true},
		{Name: "score", Type: table.FieldTypeF64, Nullable: true},
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

// nullTestTable builds a 3-row table whose "score" column is null at
// the supplied row indices. "region" never carries a null so the
// tests can tell a scale-bound drop from an unbound one.
func nullTestTable(t *testing.T, nullRows ...int) *table.Table {
	t.Helper()
	schema := &table.Schema{Fields: []table.Field{
		{Name: "region", Type: table.FieldTypeCategoricalU8},
		{Name: "score", Type: table.FieldTypeF64, Nullable: true},
	}}
	nulls := table.NewNullBitmap(3)
	for _, i := range nullRows {
		nulls.Set(i)
	}
	cols := map[string]table.Column{
		"region": table.StringColumn{"west", "east", "north"},
		"score":  table.NullableColumn{Inner: table.FloatColumn{0.5, 0.6, 0.7}, Nulls: nulls},
	}
	tbl, err := table.NewTable(schema, cols, 3, "xxh64:0000000000000002")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	return tbl
}

func TestPrismDropNullRowsPassesCleanTableThrough(t *testing.T) {
	tbl := nullTestTable(t)
	got, warn, err := DropNullRows(tbl, "layer-0",
		NullChannel{Channel: "x", Field: "region"},
		NullChannel{Channel: "y", Field: "score"})
	if err != nil {
		t.Fatalf("DropNullRows: %v", err)
	}
	if warn != nil {
		t.Errorf("warning = %+v, want nil", warn)
	}
	if got != tbl {
		t.Error("table was rebuilt; a null-free table must pass through untouched")
	}
}

func TestPrismDropNullRowsFiltersAndWarns(t *testing.T) {
	tbl := nullTestTable(t, 1)
	got, warn, err := DropNullRows(tbl, "layer-2",
		NullChannel{Channel: "x", Field: "region"},
		NullChannel{Channel: "y", Field: "score"})
	if err != nil {
		t.Fatalf("DropNullRows: %v", err)
	}
	if got.NumRows() != 2 {
		t.Fatalf("NumRows = %d, want 2", got.NumRows())
	}
	// Row 1 leaves every column, not just the one that was null.
	region, _ := got.Column("region")
	if region.ValueAt(0) != "west" || region.ValueAt(1) != "north" {
		t.Errorf("region survivors = %v/%v, want west/north", region.ValueAt(0), region.ValueAt(1))
	}
	score, _ := got.Column("score")
	if score.IsNull(0) || score.IsNull(1) {
		t.Errorf("survivors still flagged null: %v/%v", score.IsNull(0), score.IsNull(1))
	}
	if score.ValueAt(1) != 0.7 {
		t.Errorf("score[1] = %v, want 0.7", score.ValueAt(1))
	}
	if warn == nil {
		t.Fatal("expected a warning")
	}
	if warn.Code != scene.WarnNullDropped {
		t.Errorf("code = %q, want %q", warn.Code, scene.WarnNullDropped)
	}
	if warn.Layer != "layer-2" {
		t.Errorf("layer = %q, want layer-2", warn.Layer)
	}
	if warn.Details["count"] != 1 {
		t.Errorf("count = %v, want 1", warn.Details["count"])
	}
	chans, _ := warn.Details["channels"].([]string)
	if len(chans) != 1 || chans[0] != "y" {
		t.Errorf("channels = %v, want [y]", warn.Details["channels"])
	}
}

func TestPrismDropNullRowsIgnoresUnboundColumns(t *testing.T) {
	tbl := nullTestTable(t, 0, 1, 2)
	// "score" is null in every row but is not named as a channel, so
	// nothing drops — the non-scale-bound null case (tooltip, text…).
	got, warn, err := DropNullRows(tbl, "", NullChannel{Channel: "x", Field: "region"})
	if err != nil {
		t.Fatalf("DropNullRows: %v", err)
	}
	if got.NumRows() != 3 || warn != nil {
		t.Errorf("rows=%d warn=%+v, want 3 rows and no warning", got.NumRows(), warn)
	}
}

func TestPrismDropNullRowsAllNullIsAnError(t *testing.T) {
	tbl := nullTestTable(t, 0, 1, 2)
	_, _, err := DropNullRows(tbl, "", NullChannel{Channel: "y", Field: "score"})
	if err == nil {
		t.Fatal("expected an error when every row drops")
	}
	var app *prismerrors.AppError
	if !errors.As(err, &app) {
		t.Fatalf("error %v is not an *AppError", err)
	}
	if app.Code != "PRISM_ENCODE_NULL_ALL_ROWS" {
		t.Errorf("code = %q, want PRISM_ENCODE_NULL_ALL_ROWS", app.Code)
	}
}
