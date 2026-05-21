package table

// NullBitmap is a packed bit set tracking which positions in a column
// carry an explicit null marker. Bits are LSB-first within each word.
// A nil *NullBitmap is treated as "no nulls" — callers should compare
// against nil before touching the methods.
//
// The bitmap is grown lazily by Set; explicit pre-allocation via
// NewNullBitmap is preferred when the upper bound is known so the
// hash-join writer doesn't churn allocations as it appends nulls one
// row at a time.
type NullBitmap struct {
	bits  []uint64
	count int
	// n is the high-water mark seen by Set; callers can read it via
	// Capacity to bound iteration when the column's length isn't
	// otherwise accessible.
	n int
}

// NewNullBitmap returns a bitmap pre-sized for n positions. n may be
// zero; the slice grows on demand.
func NewNullBitmap(n int) *NullBitmap {
	words := (n + 63) / 64
	return &NullBitmap{bits: make([]uint64, words), n: n}
}

// Set marks position i as null. Out-of-range positions extend the
// backing slice. Calling Set twice on the same position is a no-op
// for Count.
func (b *NullBitmap) Set(i int) {
	if b == nil || i < 0 {
		return
	}
	word := i / 64
	bit := uint(i % 64)
	if word >= len(b.bits) {
		grow := make([]uint64, word+1)
		copy(grow, b.bits)
		b.bits = grow
	}
	mask := uint64(1) << bit
	if b.bits[word]&mask == 0 {
		b.bits[word] |= mask
		b.count++
	}
	if i >= b.n {
		b.n = i + 1
	}
}

// IsNull reports whether position i carries a null marker.
func (b *NullBitmap) IsNull(i int) bool {
	if b == nil || i < 0 {
		return false
	}
	word := i / 64
	if word >= len(b.bits) {
		return false
	}
	bit := uint(i % 64)
	return b.bits[word]&(uint64(1)<<bit) != 0
}

// Count returns the number of bits set in the bitmap.
func (b *NullBitmap) Count() int {
	if b == nil {
		return 0
	}
	return b.count
}

// Capacity returns the high-water mark of positions ever set or the
// pre-allocated size — whichever is larger.
func (b *NullBitmap) Capacity() int {
	if b == nil {
		return 0
	}
	return b.n
}
