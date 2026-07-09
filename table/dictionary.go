package table

import "fmt"

// Dictionary maps string categories to sequential uint32 IDs, mirroring
// the surface Prism consults on Pulse's encoding.Dictionary (Add / Resolve
// / Values / IDFor / Count). It is intentionally minimal: categorical
// columns are decoded to their string values at materialisation time, so
// Prism needs only forward interning (Add) and reverse lookup (Resolve).
//
// IDs are assigned densely from 0 in insertion order. The zero value is
// not usable; construct with NewDictionary.
type Dictionary struct {
	values []string
	ids    map[string]uint32
}

// NewDictionary returns an empty, ready-to-use Dictionary.
func NewDictionary() *Dictionary {
	return &Dictionary{ids: map[string]uint32{}}
}

// Add interns s and returns its ID. Re-adding an existing value returns
// the original ID. The error return mirrors Pulse's signature (overflow at
// the categorical cap); Prism's in-memory path never overflows, so it is
// always nil here.
func (d *Dictionary) Add(s string) (uint32, error) {
	if d.ids == nil {
		d.ids = map[string]uint32{}
	}
	if id, ok := d.ids[s]; ok {
		return id, nil
	}
	id := uint32(len(d.values))
	d.values = append(d.values, s)
	d.ids[s] = id
	return id, nil
}

// Resolve returns the string for id, or "" when id is out of range.
func (d *Dictionary) Resolve(id uint32) string {
	if int(id) >= len(d.values) {
		return ""
	}
	return d.values[id]
}

// IDFor returns the ID for s and whether it is present.
func (d *Dictionary) IDFor(s string) (uint32, bool) {
	if d.ids == nil {
		return 0, false
	}
	id, ok := d.ids[s]
	return id, ok
}

// Values returns the interned values in ID order. The returned slice is a
// copy; callers may retain it freely.
func (d *Dictionary) Values() []string {
	out := make([]string, len(d.values))
	copy(out, d.values)
	return out
}

// Count returns the number of interned values.
func (d *Dictionary) Count() int { return len(d.values) }

// String renders the dictionary size for error context.
func (d *Dictionary) String() string {
	return fmt.Sprintf("Dictionary(%d entries)", len(d.values))
}
