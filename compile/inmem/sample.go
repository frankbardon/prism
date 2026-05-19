package inmem

import (
	"context"
	"math/rand"
	"sort"
	"time"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeSample draws a uniform-without-replacement subsample of size
// N(). When n.Seed is set, the RNG is deterministic; otherwise it's
// seeded from time.Now().UnixNano(). The output rows preserve the
// input's original order (we sort the picked indices ascending) so
// downstream ops see a stable sequence even though the picks are
// random.
func executeSample(_ context.Context, n *nodes.SampleNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}

	rows := in.NumRows()
	target := n.N()
	if target <= 0 {
		target = 0
	}
	if target > rows {
		target = rows
	}

	seed := time.Now().UnixNano()
	if s := n.Seed(); s != nil {
		seed = *s
	}
	r := rand.New(rand.NewSource(seed))

	picked := r.Perm(rows)[:target]
	sort.Ints(picked)

	cols := make(map[string]table.Column, len(in.FieldNames()))
	for _, name := range in.FieldNames() {
		col, _ := in.Column(name)
		cols[name] = pickRowsByIndex(col, picked)
	}
	hash := hashChain(in.Hash(), n.Fingerprint())
	return table.NewTable(in.Schema(), cols, target, hash)
}

