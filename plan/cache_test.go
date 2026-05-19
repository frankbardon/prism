package plan_test

import (
	"testing"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// hashedNode embeds fakeNode + a custom Fingerprint so cache-key tests
// can vary fingerprint independently of id.
type hashedNode struct {
	*fakeNode
	fp string
}

func (h *hashedNode) Fingerprint() string { return h.fp }

func mkHashedNode(id, fp string) *hashedNode {
	return &hashedNode{fakeNode: mkNode(id), fp: fp}
}

func mkTable(t *testing.T, hash string) *table.Table {
	t.Helper()
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "v", Type: encoding.FieldTypeF64},
	}}
	cols := map[string]table.Column{"v": table.FloatColumn{1, 2, 3}}
	tbl, err := table.NewTable(schema, cols, 3, hash)
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	return tbl
}

func TestPrismCacheKeyDeterministic(t *testing.T) {
	n := mkHashedNode("filter:1", "fp-a")
	in := mkTable(t, "xxh64:0000000000000001")

	k1 := plan.CacheKey(n, []*table.Table{in})
	k2 := plan.CacheKey(n, []*table.Table{in})
	if k1 != k2 {
		t.Fatalf("CacheKey not deterministic: %s vs %s", k1, k2)
	}
}

func TestPrismCacheKeyInputSensitive(t *testing.T) {
	n := mkHashedNode("filter:1", "fp-a")
	a := mkTable(t, "xxh64:aaaaaaaaaaaaaaaa")
	b := mkTable(t, "xxh64:bbbbbbbbbbbbbbbb")
	if plan.CacheKey(n, []*table.Table{a}) == plan.CacheKey(n, []*table.Table{b}) {
		t.Fatal("CacheKey collided for different inputs")
	}
}

func TestPrismCacheKeyIDPartOfDigest(t *testing.T) {
	a := mkHashedNode("filter:1", "same")
	b := mkHashedNode("filter:2", "same")
	in := mkTable(t, "xxh64:0000000000000001")
	if plan.CacheKey(a, []*table.Table{in}) == plan.CacheKey(b, []*table.Table{in}) {
		t.Fatal("CacheKey collided across different ids with same fingerprint")
	}
}

func TestPrismCacheKeyNilInputs(t *testing.T) {
	n := mkHashedNode("src:1", "fp")
	if k := plan.CacheKey(n, nil); k == "" {
		t.Fatal("empty cache key for nil inputs")
	}
}
