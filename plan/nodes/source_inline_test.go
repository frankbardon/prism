package nodes_test

import (
	"context"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/resolve"
)

// TestSourceNodeInlineResolver exercises the Pulse-free SourceNode path
// (E4-S1a): a ref backed by a DataResolver materialises into a native
// Table via the resolver's InlineResolver seam — no .pulse bytes.
func TestSourceNodeInlineResolver(t *testing.T) {
	data := resolve.MapDataResolver{
		"cohort": {
			Values: []map[string]any{
				{"region": "north", "score": 10.0},
				{"region": "south", "score": 20.0},
			},
		},
	}
	r := resolve.NewWithData(nil, data)
	node := nodes.New("cohort", afero.NewMemMapFs(), r)

	tbl, err := node.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := tbl.NumRows(); got != 2 {
		t.Fatalf("NumRows = %d, want 2", got)
	}
	got := tbl.FieldNames()
	if len(got) != 2 {
		t.Fatalf("FieldNames = %v, want 2 fields", got)
	}

	schema, err := node.Schema(nil)
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(schema.Fields) != 2 {
		t.Fatalf("schema fields = %d, want 2", len(schema.Fields))
	}

	count, err := node.RowCount(context.Background())
	if err != nil {
		t.Fatalf("RowCount: %v", err)
	}
	if count != 2 {
		t.Fatalf("RowCount = %d, want 2", count)
	}
}

// TestSourceNodeUnresolvedRef confirms a ref with no DataResolver wired
// surfaces PRISM_RESOLVE_REF_UNRESOLVED rather than opening a .pulse.
func TestSourceNodeUnresolvedRef(t *testing.T) {
	node := nodes.New("missing.pulse", afero.NewMemMapFs(), resolve.New(nil))
	if _, err := node.Execute(context.Background(), nil); err == nil {
		t.Fatal("Execute over unresolvable ref: want error, got nil")
	}
}
