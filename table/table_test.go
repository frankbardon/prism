package table

import (
	"strings"
	"testing"

	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
)

func miniSchema() *encoding.Schema {
	return &encoding.Schema{
		Fields: []encoding.Field{
			{Name: "id", Type: encoding.FieldTypeU32},
			{Name: "score", Type: encoding.FieldTypeF64},
			{Name: "label", Type: encoding.FieldTypeCategoricalU8, Dictionary: encoding.NewDictionary()},
		},
	}
}

func miniColumns(n int) map[string]Column {
	id := make(IntColumn, n)
	score := make(FloatColumn, n)
	label := make(StringColumn, n)
	for i := 0; i < n; i++ {
		id[i] = int64(i)
		score[i] = float64(i) * 0.5
		label[i] = "x"
	}
	return map[string]Column{"id": id, "score": score, "label": label}
}

func TestNewTableHappyPath(t *testing.T) {
	s := miniSchema()
	cols := miniColumns(3)
	tbl, err := NewTable(s, cols, 3, "abcd1234")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	if tbl.NumRows() != 3 {
		t.Fatalf("NumRows = %d, want 3", tbl.NumRows())
	}
	if tbl.Hash() != "abcd1234" {
		t.Fatalf("Hash = %q, want abcd1234", tbl.Hash())
	}
	if names := tbl.FieldNames(); !equalSlice(names, []string{"id", "score", "label"}) {
		t.Fatalf("FieldNames = %v, want [id score label]", names)
	}
	if _, ok := tbl.Column("id"); !ok {
		t.Fatalf("Column(id) not found")
	}
	if _, ok := tbl.Column("nope"); ok {
		t.Fatalf("Column(nope) unexpectedly found")
	}
}

func TestNewTableMissingColumn(t *testing.T) {
	s := miniSchema()
	cols := miniColumns(2)
	delete(cols, "score")
	_, err := NewTable(s, cols, 2, "")
	if err == nil || !strings.Contains(err.Error(), "table: have 2 columns") {
		t.Fatalf("expected missing-column error, got %v", err)
	}
}

func TestNewTableWrongLength(t *testing.T) {
	s := miniSchema()
	cols := miniColumns(3)
	cols["score"] = FloatColumn{1.0, 2.0} // wrong length
	_, err := NewTable(s, cols, 3, "")
	if err == nil || !strings.Contains(err.Error(), `column "score" has length 2`) {
		t.Fatalf("expected wrong-length error, got %v", err)
	}
}

func TestNewTableWrongKind(t *testing.T) {
	s := miniSchema()
	cols := miniColumns(3)
	cols["score"] = IntColumn{1, 2, 3} // wrong kind for f64
	_, err := NewTable(s, cols, 3, "")
	if err == nil || !strings.Contains(err.Error(), `column "score" kind int`) {
		t.Fatalf("expected wrong-kind error, got %v", err)
	}
}

func TestNewTableRowLimit(t *testing.T) {
	t.Setenv("PRISM_TABLE_MAX_ROWS", "10")
	s := miniSchema()
	cols := miniColumns(11)
	_, err := NewTable(s, cols, 11, "")
	if err == nil {
		t.Fatalf("expected row-limit error")
	}
	ae, ok := err.(*prismerrors.AppError)
	if !ok {
		t.Fatalf("expected *AppError, got %T", err)
	}
	if ae.Code != "PRISM_RESOLVE_007" {
		t.Fatalf("got %s, want PRISM_RESOLVE_007", ae.Code)
	}
}

func TestColumnsInDeclarationOrder(t *testing.T) {
	s := miniSchema()
	cols := miniColumns(2)
	tbl, err := NewTable(s, cols, 2, "")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	got := tbl.Columns()
	if len(got) != 3 {
		t.Fatalf("Columns len = %d, want 3", len(got))
	}
	if got[0].Kind() != KindInt {
		t.Fatalf("first column kind = %s, want int", got[0].Kind())
	}
	if got[1].Kind() != KindFloat {
		t.Fatalf("second column kind = %s, want float", got[1].Kind())
	}
	if got[2].Kind() != KindString {
		t.Fatalf("third column kind = %s, want string", got[2].Kind())
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
