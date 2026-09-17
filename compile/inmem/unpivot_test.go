package inmem

import (
	"context"
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/limits"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// wideTable builds the canonical wide fixture: one categorical id
// column plus two numeric measure columns.
//
//	region | q1 | q2
//	-------+----+----
//	east   |  1 |  2
//	west   |  3 |  4
func wideTable(t *testing.T) *table.Table {
	t.Helper()
	schema := &table.Schema{Fields: []table.Field{
		{Name: "region", Type: table.FieldTypeCategoricalU8},
		{Name: "q1", Type: table.FieldTypeF64},
		{Name: "q2", Type: table.FieldTypeF64},
	}}
	cols := map[string]table.Column{
		"region": table.StringColumn{"east", "west"},
		"q1":     table.FloatColumn{1, 3},
		"q2":     table.FloatColumn{2, 4},
	}
	tbl, err := table.NewTable(schema, cols, 2, "widesrc")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	return tbl
}

// unpivotRows drains the output into comparable per-row tuples.
func unpivotRows(t *testing.T, out *table.Table, keyName, valName string, carried ...string) [][]any {
	t.Helper()
	rows := make([][]any, out.NumRows())
	names := append(append([]string{}, carried...), keyName, valName)
	for i := 0; i < out.NumRows(); i++ {
		tuple := make([]any, 0, len(names))
		for _, n := range names {
			col, ok := out.Column(n)
			if !ok {
				t.Fatalf("missing column %q", n)
			}
			tuple = append(tuple, col.ValueAt(i))
		}
		rows[i] = tuple
	}
	return rows
}

func TestExecuteUnpivotWideToLong(t *testing.T) {
	in := wideTable(t)
	n := nodes.NewUnpivot("up:1", "src", []string{"q1", "q2"}, nil)
	out, err := executeUnpivot(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("executeUnpivot: %v", err)
	}
	if got, want := out.NumRows(), 2*2; got != want {
		t.Fatalf("NumRows = %d, want %d (input_rows × unpivot_fields)", got, want)
	}
	// Row-major: both measures of input row 0, then of input row 1.
	want := [][]any{
		{"east", "q1", 1.0},
		{"east", "q2", 2.0},
		{"west", "q1", 3.0},
		{"west", "q2", 4.0},
	}
	got := unpivotRows(t, out, "key", "value", "region")
	for i := range want {
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Errorf("row %d col %d = %v, want %v", i, j, got[i][j], want[i][j])
			}
		}
	}
}

// The executed table must carry exactly the schema the plan declared —
// same names, same order, same types. A divergence otherwise surfaces
// far from its cause as a column-count mismatch.
func TestExecuteUnpivotSchemaMatchesNodeContract(t *testing.T) {
	in := wideTable(t)
	n := nodes.NewUnpivot("up:1", "src", []string{"q1", "q2"}, nil)
	want, err := n.Schema([]*table.Schema{in.Schema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	out, err := executeUnpivot(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("executeUnpivot: %v", err)
	}
	got := out.Schema()
	if len(got.Fields) != len(want.Fields) {
		t.Fatalf("output has %d fields, node schema declares %d", len(got.Fields), len(want.Fields))
	}
	for i := range want.Fields {
		if got.Fields[i].Name != want.Fields[i].Name || got.Fields[i].Type != want.Fields[i].Type {
			t.Errorf("field %d = %s/%s, want %s/%s", i,
				got.Fields[i].Name, got.Fields[i].Type,
				want.Fields[i].Name, want.Fields[i].Type)
		}
	}
	// Field order is also the declared column order on the table.
	wantOrder := []string{"region", "key", "value"}
	gotOrder := out.FieldNames()
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("column order %v, want %v", gotOrder, wantOrder)
		}
	}
}

func TestExecuteUnpivotCustomAsNames(t *testing.T) {
	in := wideTable(t)
	n := nodes.NewUnpivot("up:1", "src", []string{"q1", "q2"}, []string{"quarter", "amount"})
	out, err := executeUnpivot(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("executeUnpivot: %v", err)
	}
	if _, ok := out.Column("quarter"); !ok {
		t.Errorf("missing key column %q", "quarter")
	}
	if _, ok := out.Column("amount"); !ok {
		t.Errorf("missing value column %q", "amount")
	}
	if _, ok := out.Column("key"); ok {
		t.Errorf("default key column emitted alongside a custom as[0]")
	}
	col, _ := out.Column("quarter")
	if got := col.ValueAt(0); got != "q1" {
		t.Errorf("quarter[0] = %v, want q1", got)
	}
	col, _ = out.Column("amount")
	if got := col.ValueAt(3); got != 4.0 {
		t.Errorf("amount[3] = %v, want 4", got)
	}
}

// A null measure cell yields a null output value: the row survives and
// the value is never coerced to zero.
func TestExecuteUnpivotNullMeasureCell(t *testing.T) {
	schema := &table.Schema{Fields: []table.Field{
		{Name: "region", Type: table.FieldTypeCategoricalU8},
		{Name: "q1", Type: table.FieldTypeF64, Nullable: true},
		{Name: "q2", Type: table.FieldTypeF64},
	}}
	nulls := table.NewNullBitmap(2)
	nulls.Set(1) // west/q1 is null
	cols := map[string]table.Column{
		"region": table.StringColumn{"east", "west"},
		"q1":     table.NullableColumn{Inner: table.FloatColumn{1, 0}, Nulls: nulls},
		"q2":     table.FloatColumn{2, 4},
	}
	in, err := table.NewTable(schema, cols, 2, "nullsrc")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}

	n := nodes.NewUnpivot("up:1", "src", []string{"q1", "q2"}, nil)
	out, err := executeUnpivot(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("executeUnpivot: %v", err)
	}
	if got, want := out.NumRows(), 4; got != want {
		t.Fatalf("NumRows = %d, want %d (the null row is kept)", got, want)
	}
	val, _ := out.Column("value")
	// Row-major: index 2 is west/q1.
	if !val.IsNull(2) {
		t.Errorf("value[2] is not flagged null (got %v)", val.ValueAt(2))
	}
	if val.ValueAt(2) != nil {
		t.Errorf("value[2] = %v, want nil", val.ValueAt(2))
	}
	if got := val.NullCount(); got != 1 {
		t.Errorf("NullCount = %d, want 1", got)
	}
	for _, i := range []int{0, 1, 3} {
		if val.IsNull(i) {
			t.Errorf("value[%d] unexpectedly null", i)
		}
	}
	if !out.Schema().Field("value").Nullable {
		t.Errorf("value field not marked Nullable despite a null cell")
	}
	// The key column still names the source column of the null cell.
	key, _ := out.Column("key")
	if got := key.ValueAt(2); got != "q1" {
		t.Errorf("key[2] = %v, want q1", got)
	}
}

func TestExecuteUnpivotSingleField(t *testing.T) {
	in := wideTable(t)
	n := nodes.NewUnpivot("up:1", "src", []string{"q2"}, nil)
	out, err := executeUnpivot(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("executeUnpivot: %v", err)
	}
	if got, want := out.NumRows(), 2; got != want {
		t.Fatalf("NumRows = %d, want %d", got, want)
	}
	// q1 is not unpivoted, so it is carried through unchanged.
	q1, ok := out.Column("q1")
	if !ok {
		t.Fatalf("carried column q1 dropped")
	}
	if q1.ValueAt(0) != 1.0 || q1.ValueAt(1) != 3.0 {
		t.Errorf("q1 = [%v %v], want [1 3]", q1.ValueAt(0), q1.ValueAt(1))
	}
	val, _ := out.Column("value")
	if val.ValueAt(0) != 2.0 || val.ValueAt(1) != 4.0 {
		t.Errorf("value = [%v %v], want [2 4]", val.ValueAt(0), val.ValueAt(1))
	}
}

// Unpivoting every numeric column leaves only the carried categorical
// plus the key / value pair.
func TestExecuteUnpivotAllMeasuresUnpivoted(t *testing.T) {
	in := wideTable(t)
	n := nodes.NewUnpivot("up:1", "src", []string{"q1", "q2"}, nil)
	out, err := executeUnpivot(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("executeUnpivot: %v", err)
	}
	if got := out.FieldNames(); len(got) != 3 {
		t.Fatalf("FieldNames = %v, want region/key/value", got)
	}
	for _, gone := range []string{"q1", "q2"} {
		if _, ok := out.Column(gone); ok {
			t.Errorf("unpivoted column %q survived in the output", gone)
		}
	}
	// The carried column repeats once per measure.
	region, _ := out.Column("region")
	want := []string{"east", "east", "west", "west"}
	for i, w := range want {
		if got := region.ValueAt(i); got != w {
			t.Errorf("region[%d] = %v, want %v", i, got, w)
		}
	}
}

// Every column unpivoted (no carried columns at all) still yields the
// key / value pair.
func TestExecuteUnpivotEveryColumnUnpivoted(t *testing.T) {
	schema := &table.Schema{Fields: []table.Field{
		{Name: "a", Type: table.FieldTypeF64},
		{Name: "b", Type: table.FieldTypeF64},
	}}
	cols := map[string]table.Column{
		"a": table.FloatColumn{1, 3},
		"b": table.FloatColumn{2, 4},
	}
	in, err := table.NewTable(schema, cols, 2, "allsrc")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	n := nodes.NewUnpivot("up:1", "src", []string{"a", "b"}, nil)
	out, err := executeUnpivot(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("executeUnpivot: %v", err)
	}
	if got := out.FieldNames(); len(got) != 2 || got[0] != "key" || got[1] != "value" {
		t.Fatalf("FieldNames = %v, want [key value]", got)
	}
	if got, want := out.NumRows(), 4; got != want {
		t.Fatalf("NumRows = %d, want %d", got, want)
	}
}

// The reshape multiplies the row count, so the product is checked
// before anything is allocated.
func TestExecuteUnpivotRowCapRefusesProduct(t *testing.T) {
	t.Setenv(limits.EnvTableMaxRows, "3")
	in := wideTable(t) // 2 rows × 2 measures = 4 > 3
	n := nodes.NewUnpivot("up:1", "src", []string{"q1", "q2"}, nil)
	_, err := executeUnpivot(context.Background(), n, []*table.Table{in})
	if err == nil {
		t.Fatalf("executeUnpivot succeeded, want PRISM_RESOLVE_007")
	}
	appErr, ok := err.(*prismerrors.AppError)
	if !ok {
		t.Fatalf("error type %T, want *errors.AppError", err)
	}
	if appErr.Code != "PRISM_RESOLVE_007" {
		t.Errorf("code = %s, want PRISM_RESOLVE_007", appErr.Code)
	}
	if got := appErr.Context["Actual"]; got != 4 {
		t.Errorf("Actual = %v, want 4 (the product, not the input row count)", got)
	}
}

// A categorical source column cannot fill an f64 value column; it is
// refused with a coded error rather than yielding zeros or NaN.
func TestExecuteUnpivotNonNumericMeasureRefused(t *testing.T) {
	in := wideTable(t)
	n := nodes.NewUnpivot("up:1", "src", []string{"region"}, nil)
	_, err := executeUnpivot(context.Background(), n, []*table.Table{in})
	if err == nil {
		t.Fatalf("executeUnpivot succeeded, want PRISM_COMPILE_002")
	}
	appErr, ok := err.(*prismerrors.AppError)
	if !ok {
		t.Fatalf("error type %T, want *errors.AppError", err)
	}
	if appErr.Code != "PRISM_COMPILE_002" {
		t.Errorf("code = %s, want PRISM_COMPILE_002", appErr.Code)
	}
}

func TestExecuteUnpivotMissingFieldRefused(t *testing.T) {
	in := wideTable(t)
	n := nodes.NewUnpivot("up:1", "src", []string{"q3"}, nil)
	_, err := executeUnpivot(context.Background(), n, []*table.Table{in})
	if err == nil {
		t.Fatalf("executeUnpivot succeeded, want PRISM_PLAN_003")
	}
	appErr, ok := err.(*prismerrors.AppError)
	if !ok {
		t.Fatalf("error type %T, want *errors.AppError", err)
	}
	if appErr.Code != "PRISM_PLAN_003" {
		t.Errorf("code = %s, want PRISM_PLAN_003", appErr.Code)
	}
}

// A key / value name that collides with a carried column is named
// rather than surfacing as an opaque column-count mismatch.
func TestExecuteUnpivotOutputNameCollisionRefused(t *testing.T) {
	in := wideTable(t)
	n := nodes.NewUnpivot("up:1", "src", []string{"q1", "q2"}, []string{"region", "value"})
	_, err := executeUnpivot(context.Background(), n, []*table.Table{in})
	if err == nil {
		t.Fatalf("executeUnpivot succeeded, want PRISM_COMPILE_002")
	}
	appErr, ok := err.(*prismerrors.AppError)
	if !ok {
		t.Fatalf("error type %T, want *errors.AppError", err)
	}
	if appErr.Code != "PRISM_COMPILE_002" {
		t.Errorf("code = %s, want PRISM_COMPILE_002", appErr.Code)
	}
}

// Dispatch: the backend routes an UnpivotNode to the executor, and the
// node's own Execute routes through the wired backend.
func TestPrismInMemBackendDispatchUnpivot(t *testing.T) {
	in := wideTable(t)
	n := nodes.NewUnpivot("up:1", "src", []string{"q1", "q2"}, nil)
	b := New()
	out, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if got, want := out.NumRows(), 4; got != want {
		t.Fatalf("NumRows = %d, want %d", got, want)
	}

	n.SetBackend(b)
	viaNode, err := n.Execute(context.Background(), []*table.Table{in})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got, want := viaNode.NumRows(), 4; got != want {
		t.Fatalf("Execute NumRows = %d, want %d", got, want)
	}
}
