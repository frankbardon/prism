package inmem

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// xtabFixture builds a small region × quarter × revenue table:
//
//	region quarter revenue
//	east   Q1      10
//	east   Q1      20
//	east   Q2       5
//	west   Q1      40
//	west   Q2      30
//	west   Q2      30
//
// sum(revenue) cells: east/Q1=30 east/Q2=5 west/Q1=40 west/Q2=60.
// row margins: east=35 west=100. col margins: Q1=70 Q2=65. grand=135.
func xtabFixture(t *testing.T) *table.Table {
	t.Helper()
	region := table.StringColumn{"east", "east", "east", "west", "west", "west"}
	quarter := table.StringColumn{"Q1", "Q1", "Q2", "Q1", "Q2", "Q2"}
	revenue := table.FloatColumn{10, 20, 5, 40, 30, 30}
	schema := &table.Schema{Fields: []table.Field{
		{Name: "region", Type: table.FieldTypeCategoricalU8},
		{Name: "quarter", Type: table.FieldTypeCategoricalU8},
		{Name: "revenue", Type: table.FieldTypeF64},
	}}
	tbl, err := table.NewTable(schema, map[string]table.Column{
		"region": region, "quarter": quarter, "revenue": revenue,
	}, 6, "xtabsrc")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	return tbl
}

// runXtab builds a CrosstabNode over the fixture schema and executes the
// in-memory crosstab, returning the long-form output table.
func runXtab(t *testing.T, in *table.Table, body spec.CrosstabBody) *table.Table {
	t.Helper()
	n, err := nodes.NewCrosstab("ct", "src", "ref", afero.NewMemMapFs(), body)
	if err != nil {
		t.Fatalf("NewCrosstab: %v", err)
	}
	out, err := executeCrosstab(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("executeCrosstab: %v", err)
	}
	return out
}

// cellMap projects (region, quarter) → value for a body-cell column.
func cellMap(t *testing.T, tbl *table.Table, valueCol string) map[string]float64 {
	t.Helper()
	r, _ := tbl.Column("region")
	q, _ := tbl.Column("quarter")
	v, ok := tbl.Column(valueCol)
	if !ok {
		t.Fatalf("missing column %q", valueCol)
	}
	m, _ := tbl.Column("_margin")
	out := map[string]float64{}
	for i := 0; i < tbl.NumRows(); i++ {
		if m != nil {
			if s, _ := m.ValueAt(i).(string); s != "" {
				continue // skip margin rows
			}
		}
		region, _ := r.ValueAt(i).(string)
		quarter, _ := q.ValueAt(i).(string)
		val, _ := v.ValueAt(i).(float64)
		out[region+"/"+quarter] = val
	}
	return out
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCrosstabCellsAndSum(t *testing.T) {
	out := runXtab(t, xtabFixture(t), spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "region"}},
		Columns: []spec.CrosstabGroup{{Field: "quarter"}},
		Cell:    spec.CrosstabCell{Aggregate: "sum", Field: "revenue", As: "s"},
	})
	if out.NumRows() != 4 {
		t.Fatalf("body rows = %d, want 4", out.NumRows())
	}
	got := cellMap(t, out, "s")
	want := map[string]float64{"east/Q1": 30, "east/Q2": 5, "west/Q1": 40, "west/Q2": 60}
	for k, w := range want {
		if !approx(got[k], w) {
			t.Errorf("cell %s = %v, want %v", k, got[k], w)
		}
	}
}

func TestCrosstabMargins(t *testing.T) {
	out := runXtab(t, xtabFixture(t), spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "region"}},
		Columns: []spec.CrosstabGroup{{Field: "quarter"}},
		Cell:    spec.CrosstabCell{Aggregate: "sum", Field: "revenue", As: "s"},
		Margins: &spec.CrosstabMargins{Rows: true, Columns: true, Grand: true},
	})
	m, ok := out.Column("_margin")
	if !ok {
		t.Fatal("missing _margin column")
	}
	r, _ := out.Column("region")
	q, _ := out.Column("quarter")
	s, _ := out.Column("s")
	rowM := map[string]float64{}
	colM := map[string]float64{}
	var grand float64
	for i := 0; i < out.NumRows(); i++ {
		tag, _ := m.ValueAt(i).(string)
		val, _ := s.ValueAt(i).(float64)
		switch tag {
		case "row":
			region, _ := r.ValueAt(i).(string)
			rowM[region] = val
		case "column":
			quarter, _ := q.ValueAt(i).(string)
			colM[quarter] = val
		case "grand":
			grand = val
		}
	}
	if !approx(rowM["east"], 35) || !approx(rowM["west"], 100) {
		t.Errorf("row margins = %v, want east=35 west=100", rowM)
	}
	if !approx(colM["Q1"], 70) || !approx(colM["Q2"], 65) {
		t.Errorf("col margins = %v, want Q1=70 Q2=65", colM)
	}
	if !approx(grand, 135) {
		t.Errorf("grand = %v, want 135", grand)
	}
}

func TestCrosstabNormalizeRow(t *testing.T) {
	out := runXtab(t, xtabFixture(t), spec.CrosstabBody{
		Rows:      []spec.CrosstabGroup{{Field: "region"}},
		Columns:   []spec.CrosstabGroup{{Field: "quarter"}},
		Cell:      spec.CrosstabCell{Aggregate: "sum", Field: "revenue", As: "s"},
		Normalize: "row",
	})
	got := cellMap(t, out, "s")
	want := map[string]float64{"east/Q1": 30.0 / 35, "east/Q2": 5.0 / 35, "west/Q1": 40.0 / 100, "west/Q2": 60.0 / 100}
	for k, w := range want {
		if !approx(got[k], w) {
			t.Errorf("normalized cell %s = %v, want %v", k, got[k], w)
		}
	}
}

func TestCrosstabOverlays(t *testing.T) {
	out := runXtab(t, xtabFixture(t), spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "region"}},
		Columns: []spec.CrosstabGroup{{Field: "quarter"}},
		Cell:    spec.CrosstabCell{Aggregate: "sum", Field: "revenue", As: "s"},
		Overlays: []spec.CrosstabOverlay{
			{Kind: "share_of_row", As: "rs"},
			{Kind: "share_of_col", As: "cs"},
			{Kind: "index_vs_margin", Axis: "row", As: "idx"},
		},
	})
	rs := cellMap(t, out, "rs")
	cs := cellMap(t, out, "cs")
	idx := cellMap(t, out, "idx")
	// share_of_row sums to 1 within each region.
	if !approx(rs["east/Q1"]+rs["east/Q2"], 1) || !approx(rs["west/Q1"]+rs["west/Q2"], 1) {
		t.Errorf("share_of_row does not sum to 1 per row: %v", rs)
	}
	// share_of_col sums to 1 within each quarter.
	if !approx(cs["east/Q1"]+cs["west/Q1"], 1) || !approx(cs["east/Q2"]+cs["west/Q2"], 1) {
		t.Errorf("share_of_col does not sum to 1 per column: %v", cs)
	}
	// index_vs_margin axis=row: cell / row_margin * 100.
	if !approx(idx["east/Q1"], 30.0/35*100) || !approx(idx["west/Q2"], 60.0/100*100) {
		t.Errorf("index_vs_margin = %v", idx)
	}
}

// TestCrosstabDateGrouper verifies the date column-grouper buckets to
// the calendar label and aggregates correctly.
func TestCrosstabDateGrouper(t *testing.T) {
	day := func(s string) int64 {
		ts, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return ts.Unix() / secondsPerDay
	}
	region := table.StringColumn{"east", "east", "west", "west"}
	d := table.DateColumn{day("2024-01-15"), day("2024-04-20"), day("2024-01-05"), day("2024-04-01")}
	revenue := table.FloatColumn{10, 20, 40, 30}
	schema := &table.Schema{Fields: []table.Field{
		{Name: "region", Type: table.FieldTypeCategoricalU8},
		{Name: "d", Type: table.FieldTypeDate},
		{Name: "revenue", Type: table.FieldTypeF64},
	}}
	in, err := table.NewTable(schema, map[string]table.Column{
		"region": region, "d": d, "revenue": revenue,
	}, 4, "datesrc")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	out := runXtab(t, in, spec.CrosstabBody{
		Rows:    []spec.CrosstabGroup{{Field: "region"}},
		Columns: []spec.CrosstabGroup{{Field: "d", Type: "date", Period: "quarter"}},
		Cell:    spec.CrosstabCell{Aggregate: "sum", Field: "revenue", As: "s"},
	})
	dc, ok := out.Column("d")
	if !ok || dc.Kind() != table.KindString {
		t.Fatalf("date grouper output column must be a string bucket key, got %v", dc.Kind())
	}
	// Expect bucket labels 2024-Q1 and 2024-Q2.
	rc, _ := out.Column("region")
	sc, _ := out.Column("s")
	got := map[string]float64{}
	for i := 0; i < out.NumRows(); i++ {
		region, _ := rc.ValueAt(i).(string)
		bucket, _ := dc.ValueAt(i).(string)
		val, _ := sc.ValueAt(i).(float64)
		got[region+"/"+bucket] = val
	}
	want := map[string]float64{"east/2024-Q1": 10, "east/2024-Q2": 20, "west/2024-Q1": 40, "west/2024-Q2": 30}
	for k, w := range want {
		if !approx(got[k], w) {
			t.Errorf("date cell %s = %v, want %v (all=%v)", k, got[k], w, got)
		}
	}
}
