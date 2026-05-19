package plan

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/frankbardon/prism/table"
)

// TableCache is the interface every Prism table cache satisfies. P03
// ships the interface only; the LRU implementation lands in P07.
// ExecOpts.Cache stays nil in P03.
type TableCache interface {
	Get(key string) (*table.Table, bool)
	Put(key string, t *table.Table)
}

// CacheKey is the deterministic cache key for one node execution.
// Computed as sha256 of (NodeID, Fingerprint, each input table's
// Hash()), hex-encoded. Two nodes with the same fingerprint but
// different IDs produce different keys because the ID is part of the
// digest — so the cache cannot accidentally share a result between
// two visually-equivalent nodes that the optimizer has chosen to
// keep distinct.
func CacheKey(n Node, ins []*table.Table) string {
	h := sha256.New()
	h.Write([]byte(n.ID()))
	h.Write([]byte{0})
	h.Write([]byte(n.Fingerprint()))
	h.Write([]byte{0})
	for _, t := range ins {
		if t == nil {
			h.Write([]byte("<nil>"))
		} else {
			h.Write([]byte(t.Hash()))
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
