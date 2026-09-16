package spec

// Ordering (E5-S4).
//
// `encoding.order` is Vega-Lite's overloaded ordering channel. Prism
// resolves all three of its senses through ONE mechanism: the row
// order of the table the encoder consumes.
//
//   - Stack order — which segment sits at the bottom of a stack.
//     plan/nodes/stack.go ranks segments by first appearance of the
//     stack-by tuple, so reordering the rows reorders the stack.
//   - Draw order — the sequence per-row marks (point, bar, rule, …)
//     are emitted in, and therefore which one paints on top.
//   - Point sequence — the order points connect along a line or area
//     path, since encode/marks/group.go's groupRows hands each group
//     its row indices in table order.
//
// The reordering itself is a synthetic SortNode injected by
// plan/build/build.go, placed after the synthetic encoding aggregate
// and before the StackNode. Both stages read ResolveOrder on the same
// spec, so plan and encode can never disagree about whether the
// author took control of row order — the same contract spec/stack.go
// establishes for stacking.
//
// When `order` is unbound ResolveOrder returns nil, no node is
// injected, and table order is preserved exactly. That is the
// Vega-Lite rule and it is what keeps every pre-E5-S4 chart
// byte-identical.

// Sort direction vocabulary shared by `encoding.order` entries and
// the `sort` transform's per-field `order` key.
//
// "ascending" / "descending" are the canonical wire spellings (they
// are what the JSON Schema documents and what Vega-Lite uses); "asc"
// / "desc" are accepted aliases. Both spellings are listed in the
// schema enums so the schema and the decoder agree — a mismatch there
// is exactly what made "descending" a silent no-op in the `sort` and
// `window` transforms before E5-S4.
const (
	SortAscending       = "ascending"
	SortDescending      = "descending"
	SortAscendingShort  = "asc"
	SortDescendingShort = "desc"
)

// SortDirectionDescending reports whether a wire sort direction means
// "reverse the comparison". Empty (the default) and every ascending
// spelling report false.
func SortDirectionDescending(v string) bool {
	return v == SortDescending || v == SortDescendingShort
}

// SortDirectionValid reports whether v is a recognised sort
// direction. Empty is valid and means ascending.
func SortDirectionValid(v string) bool {
	switch v {
	case "", SortAscending, SortDescending, SortAscendingShort, SortDescendingShort:
		return true
	}
	return false
}

// OrderKey is one resolved ordering key: the table column to compare
// and the direction to compare it in.
type OrderKey struct {
	Field      string
	Descending bool
}

// OrderEntries flattens encoding.order — which decodes as either a
// single entry or an array — into a flat slice, mirroring
// DetailEntries.
func OrderEntries(enc *Encoding) []OrderChannelEntry {
	if enc == nil || enc.Order == nil {
		return nil
	}
	out := make([]OrderChannelEntry, 0, len(enc.Order.Multi)+1)
	if enc.Order.Single != nil {
		out = append(out, *enc.Order.Single)
	}
	out = append(out, enc.Order.Multi...)
	return out
}

// ResolveOrder returns the ordering keys encoding.order binds, in
// spec order, or nil when order is unbound (or bound only to entries
// with no field, which validate rejects with PRISM_SPEC_055).
//
// Returning nil is the signal every caller keys on: no synthetic sort
// node, no encoder behaviour change, table order preserved exactly.
func ResolveOrder(enc *Encoding) []OrderKey {
	entries := OrderEntries(enc)
	if len(entries) == 0 {
		return nil
	}
	out := make([]OrderKey, 0, len(entries))
	for _, e := range entries {
		if e.Field == "" {
			continue
		}
		out = append(out, OrderKey{
			Field:      e.Field,
			Descending: SortDirectionDescending(e.Sort),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
