package encode_test

import (
	"context"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// tableFixtureJSON is a table-mark spec whose transform chain filters
// to region=="west" and sorts descending by revenue, and whose
// "trend" column carries a per-row numeric array bound to a
// "sparkline" sub-mark. Exercises E1-S3's acceptance criteria: the
// standard filter/sort transform pipeline feeding a table mark, and a
// sub-mark column producing nested Scene IR per row.
const tableFixtureJSON = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "values": [
      {"name": "Acme", "region": "west", "revenue": 120, "trend": [10, 12, 9, 14, 20]},
      {"name": "Globex", "region": "east", "revenue": 80, "trend": [5, 6, 4, 7, 9]},
      {"name": "Initech", "region": "west", "revenue": 200, "trend": [1, 2, 3, 2, 1]},
      {"name": "Umbrella", "region": "north", "revenue": 40, "trend": [8, 7, 6, 5, 4]}
    ]
  },
  "transform": [
    {"filter": {"op": "eq", "field": "region", "value": "west"}},
    {"sort": [{"field": "revenue", "order": "desc"}]}
  ],
  "mark": {"type": "table"},
  "encoding": {
    "columns": [
      {"field": "name", "type": "nominal", "title": "Account"},
      {"field": "revenue", "type": "quantitative"},
      {"field": "trend", "type": "quantitative", "mark": "sparkline"}
    ]
  }
}`

// runTableFixture decodes, builds, executes, and encodes
// tableFixtureJSON, returning the resulting SceneDoc.
func runTableFixture(t *testing.T, body string) *scene.SceneDoc {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	dag, tipID, err := build.Build(s, build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("execute: %d node errors: %v", len(res.Errors), res.Errors)
	}
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return doc
}

// TestPrismTableTransformPipeline proves a table mark's data resolves
// through the standard transform pipeline: filter + sort apply
// upstream exactly as for any other mark, and the resulting rows land
// in scene.Table in the filtered/sorted order.
func TestPrismTableTransformPipeline(t *testing.T) {
	doc := runTableFixture(t, tableFixtureJSON)
	if len(doc.Grid.Cells) != 1 {
		t.Fatalf("want 1 cell, got %d", len(doc.Grid.Cells))
	}
	tbl := doc.Grid.Cells[0].Scene.Table
	if tbl == nil {
		t.Fatalf("Scene.Table is nil")
	}
	if len(tbl.Rows) != 2 {
		t.Fatalf("want 2 rows after filter(region=west), got %d", len(tbl.Rows))
	}
	if got := tbl.Rows[0].Values["name"]; got != "Initech" {
		t.Errorf("row 0 name = %v, want Initech (revenue 200, sorted desc)", got)
	}
	if got := tbl.Rows[1].Values["name"]; got != "Acme" {
		t.Errorf("row 1 name = %v, want Acme (revenue 120, sorted desc)", got)
	}
	if got := tbl.Rows[0].Values["revenue"]; got != 200.0 {
		t.Errorf("row 0 revenue = %v, want 200", got)
	}
}

// TestPrismTableColumnsResolved checks the column defs carry the
// resolved field, header (falling back to field when no title), and
// sub-mark name.
func TestPrismTableColumnsResolved(t *testing.T) {
	doc := runTableFixture(t, tableFixtureJSON)
	cols := doc.Grid.Cells[0].Scene.Table.Columns
	if len(cols) != 3 {
		t.Fatalf("want 3 columns, got %d", len(cols))
	}
	if cols[0].Field != "name" || cols[0].Header != "Account" {
		t.Errorf("col 0 = %+v, want field=name header=Account", cols[0])
	}
	if cols[1].Field != "revenue" || cols[1].Header != "revenue" {
		t.Errorf("col 1 = %+v, want field=revenue header=revenue (title fallback)", cols[1])
	}
	if cols[2].Field != "trend" || cols[2].Mark != "sparkline" {
		t.Errorf("col 2 = %+v, want field=trend mark=sparkline", cols[2])
	}
}

// TestPrismTableSparkMarkCellNested proves a sub-mark column produces
// a nested Scene IR subtree per row/cell, encoded the same way a
// standalone sparkline would be.
func TestPrismTableSparkMarkCellNested(t *testing.T) {
	doc := runTableFixture(t, tableFixtureJSON)
	rows := doc.Grid.Cells[0].Scene.Table.Rows
	for _, row := range rows {
		cell := row.Cells["trend"]
		if cell == nil {
			t.Fatalf("row %d: no nested cell for sparkline column %q", row.ID, "trend")
		}
		if len(cell.Marks) == 0 {
			t.Fatalf("row %d: sparkline cell has no marks", row.ID)
		}
		mark := cell.Marks[0]
		if mark.Type != scene.MarkLine || mark.Line == nil {
			t.Fatalf("row %d: sparkline cell mark = %+v, want a populated MarkLine", row.ID, mark)
		}
		if len(mark.Line.Points) != 5 {
			t.Errorf("row %d: sparkline points = %d, want 5", row.ID, len(mark.Line.Points))
		}
	}
	// Non-sub-mark columns carry no nested cell.
	if rows[0].Cells["name"] != nil {
		t.Errorf("row 0: text column %q unexpectedly has a nested cell", "name")
	}
}

// tableAggregateFixtureJSON has no explicit transform[] — the
// "revenue" column's aggregate:"sum" must trigger
// plan/build.injectEncodingAggregate's synthetic GroupAggregateNode
// exactly like a top-level channel's aggregate would, grouping by
// every non-aggregated column ("region").
const tableAggregateFixtureJSON = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "values": [
      {"name": "Acme", "region": "west", "revenue": 120},
      {"name": "Globex", "region": "east", "revenue": 80},
      {"name": "Initech", "region": "west", "revenue": 200},
      {"name": "Umbrella", "region": "north", "revenue": 40}
    ]
  },
  "mark": {"type": "table"},
  "encoding": {
    "columns": [
      {"field": "region", "type": "nominal"},
      {"field": "revenue", "type": "quantitative", "aggregate": "sum"}
    ]
  }
}`

// TestPrismTableColumnAggregate proves a table column's aggregate
// flows through the same injectEncodingAggregate path as any other
// channel's aggregate — no bespoke table-only aggregation logic.
func TestPrismTableColumnAggregate(t *testing.T) {
	doc := runTableFixture(t, tableAggregateFixtureJSON)
	tbl := doc.Grid.Cells[0].Scene.Table
	if tbl == nil {
		t.Fatalf("Scene.Table is nil")
	}
	if len(tbl.Rows) != 3 {
		t.Fatalf("want 3 grouped rows (west/east/north), got %d", len(tbl.Rows))
	}
	sums := map[string]float64{}
	for _, row := range tbl.Rows {
		region, _ := row.Values["region"].(string)
		revenue, _ := row.Values["revenue"].(float64)
		sums[region] = revenue
	}
	want := map[string]float64{"west": 320, "east": 80, "north": 40}
	for region, wantSum := range want {
		if got := sums[region]; got != wantSum {
			t.Errorf("region %q summed revenue = %v, want %v", region, got, wantSum)
		}
	}
}

// TestPrismTableSVGUnsupported confirms the pre-existing checkMarkSupport
// guard (E1-S1) still rejects a table-mark SceneDoc via the svg
// backend now that Scene.Layers is actually populated by real
// table-mark encode output (previously only hand-built in that
// story's own unit test).
func TestPrismTableSVGUnsupported(t *testing.T) {
	doc := runTableFixture(t, tableFixtureJSON)
	layer := doc.Grid.Cells[0].Scene.Layers[0]
	if layer.Mark != scene.MarkTable {
		t.Fatalf("layer.Mark = %q, want %q", layer.Mark, scene.MarkTable)
	}
	if len(layer.Marks) != 0 {
		t.Errorf("layer.Marks = %d, want 0 (table has no positioned geometry)", len(layer.Marks))
	}
}
