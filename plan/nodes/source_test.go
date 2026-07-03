package nodes_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// tinyInlineRows loads the frozen tiny cohort rows committed under
// testdata/inline/tiny.json. The rows were captured from the retired
// tiny.pulse fixture (via pulse.Sample) at the point Pulse left the
// ingestion path, so a SourceNode backed by these rows materialises the
// same 1000-row, three-column table the .pulse loader once produced.
func tinyInlineRows(t *testing.T) []map[string]any {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// here = .../prism/plan/nodes/source_test.go
	root := filepath.Join(filepath.Dir(here), "..", "..")
	body, err := os.ReadFile(filepath.Join(root, "testdata", "inline", "tiny.json"))
	if err != nil {
		t.Fatalf("read tiny.json: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("decode tiny.json: %v", err)
	}
	return rows
}

// tinyResolver wires a DataResolver serving the frozen tiny rows under
// the ref "tiny". Fields are declared explicitly so the materialised
// schema preserves the brand_id, score, age column order the loader
// emitted.
func tinyResolver(t *testing.T) resolve.Resolver {
	t.Helper()
	data := resolve.MapDataResolver{
		"tiny": {
			Values: tinyInlineRows(t),
			Fields: []spec.FieldSpec{
				{Name: "brand_id", Type: "categorical_u8"},
				{Name: "score", Type: "f64"},
				{Name: "age", Type: "u8"},
			},
		},
	}
	return resolve.NewWithData(nil, data)
}

func TestPrismSourceNodeExecute(t *testing.T) {
	node := nodes.New("tiny", afero.NewMemMapFs(), tinyResolver(t))

	tbl, err := node.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := tbl.NumRows(); got != 1000 {
		t.Fatalf("NumRows = %d, want 1000", got)
	}
	wantFields := []string{"brand_id", "score", "age"}
	got := tbl.FieldNames()
	if len(got) != len(wantFields) {
		t.Fatalf("FieldNames len = %d, want %d (%v)", len(got), len(wantFields), got)
	}
	for i, w := range wantFields {
		if got[i] != w {
			t.Fatalf("FieldNames[%d] = %q, want %q (full: %v)", i, got[i], w, got)
		}
	}

	// Hash stability: Execute again, confirm same hash. FromInline hashes
	// the rows in schema order so identical input yields identical hash
	// regardless of map iteration order.
	tbl2, err := node.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute 2: %v", err)
	}
	if tbl.Hash() != tbl2.Hash() {
		t.Fatalf("hash mismatch across executions: %s vs %s", tbl.Hash(), tbl2.Hash())
	}
	if tbl.Hash() == "" {
		t.Fatal("hash is empty")
	}

	// Spot check a categorical: brand_id values should all be one of the
	// declared categories.
	brandCol, ok := tbl.Column("brand_id")
	if !ok {
		t.Fatal("brand_id column missing")
	}
	allowed := map[string]bool{"alpha": true, "beta": true, "gamma": true, "delta": true}
	for i := 0; i < brandCol.Len(); i++ {
		v := brandCol.ValueAt(i).(string)
		if !allowed[v] {
			t.Fatalf("brand_id[%d] = %q, not in allowed set", i, v)
		}
	}

	// Spot check a numeric column has finite values in the synth range.
	scoreCol, ok := tbl.Column("score")
	if !ok {
		t.Fatal("score column missing")
	}
	for i := 0; i < scoreCol.Len(); i++ {
		v := scoreCol.ValueAt(i).(float64)
		if v < 0 || v > 1 {
			t.Fatalf("score[%d] = %g, out of [0,1] declared by synth spec", i, v)
		}
	}
}

func TestSourceNodeOutputSchema(t *testing.T) {
	node := nodes.New("tiny", afero.NewMemMapFs(), tinyResolver(t))

	schema, err := node.OutputSchema()
	if err != nil {
		t.Fatalf("OutputSchema: %v", err)
	}
	if len(schema.Fields) != 3 {
		t.Fatalf("Fields len = %d, want 3", len(schema.Fields))
	}
}

func TestSourceNodeFingerprintStable(t *testing.T) {
	a := nodes.New("foo", nil, nil)
	b := nodes.New("foo", nil, nil)
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatalf("fingerprint differs for identical ref: %s vs %s", a.Fingerprint(), b.Fingerprint())
	}
	c := nodes.New("bar", nil, nil)
	if a.Fingerprint() == c.Fingerprint() {
		t.Fatalf("fingerprint same for different refs: %s", a.Fingerprint())
	}
}
