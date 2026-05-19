package plan_test

import (
	"context"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

func mkCacheTable(t *testing.T, hash string) *table.Table {
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

func TestPrismLRUEvictsLeastRecentlyUsed(t *testing.T) {
	c := plan.NewLRU(3)
	for i, h := range []string{"a", "b", "c"} {
		c.Put("k"+string(rune('0'+i)), mkCacheTable(t, "xxh64:"+h+"0000000000000000"))
	}
	// Fill one past capacity → oldest ("k0") must evict.
	c.Put("k3", mkCacheTable(t, "xxh64:d0000000000000000"))
	if c.Len() != 3 {
		t.Fatalf("Len=%d; want 3", c.Len())
	}
	if _, ok := c.Get("k0"); ok {
		t.Errorf("k0 should have evicted")
	}
	for _, k := range []string{"k1", "k2", "k3"} {
		if _, ok := c.Get(k); !ok {
			t.Errorf("%s should still be cached", k)
		}
	}
}

func TestPrismLRUMoveToFrontOnGet(t *testing.T) {
	c := plan.NewLRU(3)
	c.Put("k0", mkCacheTable(t, "xxh64:a0"))
	c.Put("k1", mkCacheTable(t, "xxh64:b0"))
	c.Put("k2", mkCacheTable(t, "xxh64:c0"))
	// Touch k0 so it becomes the freshest.
	if _, ok := c.Get("k0"); !ok {
		t.Fatal("k0 missing")
	}
	// Insert one more → k1 should evict (was oldest after the touch).
	c.Put("k3", mkCacheTable(t, "xxh64:d0"))
	if _, ok := c.Get("k1"); ok {
		t.Errorf("k1 should have evicted after k0 touch")
	}
	if _, ok := c.Get("k0"); !ok {
		t.Errorf("k0 should still be cached")
	}
}

func TestPrismLRUCapacityFallback(t *testing.T) {
	c := plan.NewLRU(0)
	if c.Capacity() <= 0 {
		t.Fatalf("Capacity=%d; want >0", c.Capacity())
	}
	c2 := plan.NewLRU(-5)
	if c2.Capacity() <= 0 {
		t.Fatalf("Capacity(-5)=%d; want >0", c2.Capacity())
	}
}

func TestPrismLRUHitsAndMisses(t *testing.T) {
	c := plan.NewLRU(2)
	c.Put("a", mkCacheTable(t, "xxh64:a0"))
	_, _ = c.Get("a")    // hit
	_, _ = c.Get("a")    // hit
	_, _ = c.Get("none") // miss
	if h, m := c.Hits(), c.Misses(); h != 2 || m != 1 {
		t.Fatalf("hits=%d misses=%d; want 2/1", h, m)
	}
}

func TestPrismLRUConcurrent(t *testing.T) {
	c := plan.NewLRU(64)
	keys := make([]string, 200)
	for i := range keys {
		keys[i] = "k" + intToStr(i)
	}
	t0 := mkCacheTable(t, "xxh64:0000000000000000")

	var wg sync.WaitGroup
	const goroutines = 16
	const ops = 5000
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(seed uint64) {
			defer wg.Done()
			r := rand.New(rand.NewPCG(seed, seed*7+1))
			for i := 0; i < ops; i++ {
				k := keys[r.IntN(len(keys))]
				if r.IntN(2) == 0 {
					c.Put(k, t0)
				} else {
					_, _ = c.Get(k)
				}
			}
		}(uint64(g + 1))
	}
	wg.Wait()
	if l := c.Len(); l < 0 || l > c.Capacity() {
		t.Fatalf("Len=%d out of bounds [0,%d]", l, c.Capacity())
	}
}

// TestPrismTableCacheHit runs the same DAG twice with a shared LRU and
// asserts every node hits the cache on the second pass.
func TestPrismTableCacheHit(t *testing.T) {
	cache := plan.NewLRU(16)

	build := func() *plan.DAG {
		b := plan.NewBuilder()
		// Three sibling sources + one sink.
		ids := []plan.NodeID{}
		for _, h := range []string{"alpha", "beta", "gamma"} {
			id := plan.NodeID("src-" + h)
			ids = append(ids, id)
			_ = b.AddNode(&execNode{id: id, tbl: mkTbl(t, "xxh64:"+h+"00000000000")})
			_ = b.MarkRoot(id)
		}
		_ = b.AddNode(&execNode{
			id: "sink", inputs: ids, tbl: mkTbl(t, "xxh64:sinkhash0000000"),
		})
		_ = b.MarkSink("sink")
		d, err := b.Build()
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		return d
	}

	// Cold run: every node is a miss.
	r1, err := plan.Execute(context.Background(), build(), plan.ExecOpts{
		Workers: 1, Cache: cache,
	})
	if err != nil {
		t.Fatalf("Execute(cold): %v", err)
	}
	coldMisses := cache.Misses()
	if coldMisses < 4 {
		t.Fatalf("cold misses=%d; want ≥4 (3 sources + sink)", coldMisses)
	}
	if cache.Hits() != 0 {
		t.Fatalf("cold hits=%d; want 0", cache.Hits())
	}
	if len(r1.Tables) != 4 {
		t.Fatalf("cold Tables=%d; want 4", len(r1.Tables))
	}

	// Warm run: rebuild an identical DAG (identical ids + fingerprints
	// + input hashes) and execute. Expect every node to hit.
	r2, err := plan.Execute(context.Background(), build(), plan.ExecOpts{
		Workers: 1, Cache: cache,
	})
	if err != nil {
		t.Fatalf("Execute(warm): %v", err)
	}
	if got := cache.Hits(); got != int64(len(r2.Tables)) {
		t.Errorf("warm hits=%d; want %d (one per node)", got, len(r2.Tables))
	}
	for id, t1 := range r1.Tables {
		t2, ok := r2.Tables[id]
		if !ok {
			t.Errorf("node %s missing in warm run", id)
			continue
		}
		if t1.Hash() != t2.Hash() {
			t.Errorf("node %s hash drift: %q vs %q", id, t1.Hash(), t2.Hash())
		}
	}
}
