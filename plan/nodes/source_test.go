package nodes_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/resolve"
)

// fixturePath returns the absolute path to the tiny .pulse cohort
// committed under testdata/. The lookup walks up from this test file's
// directory so `go test ./...` works regardless of cwd.
func fixturePath(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// here = .../prism/plan/nodes/source_test.go
	root := filepath.Join(filepath.Dir(here), "..", "..")
	return filepath.Join(root, "testdata", "cohorts", "tiny.pulse")
}

func TestPrismSourceNodeExecute(t *testing.T) {
	path := fixturePath(t)
	fs := afero.NewOsFs()
	r := resolve.New(nil)
	node := nodes.New(path, fs, r)

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

	// Hash stability: Execute again, confirm same hash. SourceNode
	// hashes the underlying bytes so identical input must yield
	// identical hash regardless of map iteration order.
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

	// Spot check a categorical: brand_id values should all be one of
	// the four declared categories.
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

	// Spot check a numeric column has finite values.
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
	path := fixturePath(t)
	fs := afero.NewOsFs()
	r := resolve.New(nil)
	node := nodes.New(path, fs, r)

	schema, err := node.OutputSchema()
	if err != nil {
		t.Fatalf("OutputSchema: %v", err)
	}
	if len(schema.Fields) != 3 {
		t.Fatalf("Fields len = %d, want 3", len(schema.Fields))
	}
}

func TestSourceNodeFingerprintStable(t *testing.T) {
	a := nodes.New("foo.pulse", nil, nil)
	b := nodes.New("foo.pulse", nil, nil)
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatalf("fingerprint differs for identical ref: %s vs %s", a.Fingerprint(), b.Fingerprint())
	}
	c := nodes.New("bar.pulse", nil, nil)
	if a.Fingerprint() == c.Fingerprint() {
		t.Fatalf("fingerprint same for different refs: %s", a.Fingerprint())
	}
}
