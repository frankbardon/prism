package compile_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/resolve"
	specpkg "github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// The compile e2e / parity tests once read testdata/cohorts/tiny.pulse
// through the Pulse loader and cross-checked their output against live
// pulse.Process calls. Epic E4 removed both the loader and the Pulse
// dependency from the ingestion path, so these tests are FROZEN: the
// tiny cohort rows are committed under testdata/inline/tiny.json (a
// pulse.Sample capture) and every expected aggregate is the value Pulse
// produced for that data at capture time, hardcoded as a literal. The
// tests now prove "Prism's pure-Go in-memory path == the value Pulse
// produced today" — parity preserved by construction, Pulse-free.

// tinyRef is the ref the tiny cohort is served under by tinyResolver.
const tinyRef = "tiny"

// loadInlineRows reads a committed row fixture from testdata/inline/.
func loadInlineRows(t *testing.T, name string) []map[string]any {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(here), "..")
	body, err := os.ReadFile(filepath.Join(root, "testdata", "inline", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return rows
}

// tinyFields declares the frozen tiny cohort schema so the materialised
// column order + types match what the .pulse loader emitted.
func tinyFields() []specpkg.FieldSpec {
	return []specpkg.FieldSpec{
		{Name: "brand_id", Type: "categorical_u8"},
		{Name: "score", Type: "f64"},
		{Name: "age", Type: "u8"},
	}
}

// tinyResolver serves the frozen tiny rows under tinyRef via the
// Pulse-free inline DataResolver seam, so a SourceNode bound to
// data.source="tiny" materialises them.
func tinyResolver(t *testing.T) resolve.Resolver {
	t.Helper()
	data := resolve.MapDataResolver{
		tinyRef: {Values: loadInlineRows(t, "tiny.json"), Fields: tinyFields()},
	}
	return resolve.NewWithData(nil, data)
}

// finalTable locates the tip node's output table inside an ExecResult.
func finalTable(dag *plan.DAG, res *plan.ExecResult) *table.Table {
	for _, id := range dag.Sinks() {
		if t, ok := res.Tables[id]; ok {
			return t
		}
	}
	return nil
}

// tableToBrandValueMap projects (brand_id, value) into a map.
func tableToBrandValueMap(t *testing.T, tbl *table.Table, valueCol string) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	brand, ok := tbl.Column("brand_id")
	if !ok {
		t.Fatal("brand_id column missing from prism table")
	}
	val, ok := tbl.Column(valueCol)
	if !ok {
		t.Fatalf("%s column missing", valueCol)
	}
	for i := 0; i < tbl.NumRows(); i++ {
		key, _ := brand.ValueAt(i).(string)
		v, _ := val.ValueAt(i).(float64)
		out[key] = v
	}
	return out
}
