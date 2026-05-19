package plan

import (
	"sync"
	"sync/atomic"

	"github.com/frankbardon/prism/internal/limits"
	"github.com/frankbardon/prism/table"
)

// LRU is a thread-safe bounded TableCache backed by a doubly-linked
// list + map. Eviction is least-recently-used: each Get/Put moves the
// touched entry to the front; capacity overflow drops the tail.
//
// Capacity defaults to limits.DefaultTableCacheSize when caller passes
// a non-positive value to NewLRU. Hits/Misses counters power the
// executor-level cache-hit test (see TestPrismTableCacheHit in
// plan/cache_test.go).
type LRU struct {
	capacity int
	mu       sync.Mutex
	entries  map[string]*lruEntry
	head     *lruEntry // most recently used
	tail     *lruEntry // least recently used
	hits     int64
	misses   int64
}

// lruEntry is one slot in the doubly-linked list. Stored inline in the
// map so move-to-front is O(1).
type lruEntry struct {
	key  string
	val  *table.Table
	prev *lruEntry
	next *lruEntry
}

// NewLRU constructs a bounded LRU. capacity <= 0 falls back to
// limits.DefaultTableCacheSize (consulted via env override).
func NewLRU(capacity int) *LRU {
	if capacity <= 0 {
		capacity = limits.MustTableCacheSize()
	}
	if capacity <= 0 {
		capacity = limits.DefaultTableCacheSize
	}
	return &LRU{
		capacity: capacity,
		entries:  make(map[string]*lruEntry, capacity),
	}
}

// Get returns the cached table for key, moving the entry to the front
// of the recency list. Hit / miss counters increment per call.
func (c *LRU) Get(key string) (*table.Table, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		atomic.AddInt64(&c.misses, 1)
		return nil, false
	}
	c.moveToFront(e)
	atomic.AddInt64(&c.hits, 1)
	return e.val, true
}

// Put inserts (or updates) key → t at the front of the recency list.
// Overflow evicts the tail entry; updates do not change capacity.
func (c *LRU) Put(key string, t *table.Table) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok {
		e.val = t
		c.moveToFront(e)
		return
	}
	e := &lruEntry{key: key, val: t}
	c.entries[key] = e
	c.pushFront(e)
	if len(c.entries) > c.capacity {
		c.evictTail()
	}
}

// Len returns the current number of entries. Test-only.
func (c *LRU) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Capacity returns the configured capacity. Test-only.
func (c *LRU) Capacity() int { return c.capacity }

// Hits returns the cumulative cache-hit count since construction.
// Atomic read so callers can sample from any goroutine.
func (c *LRU) Hits() int64 { return atomic.LoadInt64(&c.hits) }

// Misses returns the cumulative cache-miss count since construction.
func (c *LRU) Misses() int64 { return atomic.LoadInt64(&c.misses) }

// moveToFront unlinks e from its current position and re-links it at
// the head. No-op when e is already the head.
func (c *LRU) moveToFront(e *lruEntry) {
	if e == c.head {
		return
	}
	// Detach.
	if e.prev != nil {
		e.prev.next = e.next
	}
	if e.next != nil {
		e.next.prev = e.prev
	}
	if e == c.tail {
		c.tail = e.prev
	}
	// Insert at head.
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
}

// pushFront inserts a new entry at the head of the recency list.
func (c *LRU) pushFront(e *lruEntry) {
	e.prev = nil
	e.next = c.head
	if c.head != nil {
		c.head.prev = e
	}
	c.head = e
	if c.tail == nil {
		c.tail = e
	}
}

// evictTail drops the least-recently-used entry. Caller holds mu.
func (c *LRU) evictTail() {
	if c.tail == nil {
		return
	}
	e := c.tail
	c.tail = e.prev
	if c.tail != nil {
		c.tail.next = nil
	} else {
		c.head = nil
	}
	delete(c.entries, e.key)
}
