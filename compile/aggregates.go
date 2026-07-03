package compile

import "sort"

// Aliases is the single source of truth for friendly aggregate alias
// membership. Every alias here is computed client-side by the in-memory
// backend (compile/inmem/group_aggregate.go) — Prism executes all
// aggregates over the materialised table.Table, so the registry is a
// name catalogue only (no backend-specific op constant is carried).
//
// Mirrors validate/rules/agg_compat.go's quantitative-op list verbatim —
// adding a new alias requires editing both (a TODO in agg_compat.go
// points here).
//
// The distribution-shape scalars (`range`, `skewness`, `kurtosis`,
// `null_count`) and cohort-analytics extensions (`wmean`, `ratio`,
// `lift`, `share`, `ci0`, `ci1`) live alongside the classic scalars
// (`count`, `sum`, `mean`, …) — all resolve to a client-side
// computation.
var Aliases = map[string]struct{}{
	"count":    {},
	"sum":      {},
	"mean":     {},
	"median":   {},
	"min":      {},
	"max":      {},
	"stdev":    {},
	"variance": {},
	"mode":     {},
	"distinct": {},

	// frequency is the SCALAR companion to mode: the modal count
	// (occurrences of the most frequent value). Universal: counts
	// occurrences of any field type.
	"frequency": {},
	"q1":        {},
	"q3":        {},

	// Distribution-shape scalars: range = max-min; skewness/kurtosis are
	// the population (Fisher-Pearson) forms — kurtosis is excess (−3).
	// null_count counts null records and so applies to any field type.
	"range":      {},
	"skewness":   {},
	"kurtosis":   {},
	"null_count": {},

	// Cohort-analytics extensions (D003). Sibling-column conventions bind
	// the operands at compute time: wmean uses <field>_weight; ratio uses
	// <field> as numerator and <field>_denom as denominator; lift uses
	// <field>_baseline; ci0/ci1 default to 95% confidence.
	"ci0":   {},
	"ci1":   {},
	"wmean": {},
	"ratio": {},
	"lift":  {},
	"share": {},
}

// IsAlias reports whether name is a known friendly aggregate alias.
func IsAlias(name string) bool {
	_, ok := Aliases[name]
	return ok
}

// AllAliases returns the alias names in sorted order. Stable order
// matters for the enumeration test and for fixup messages.
func AllAliases() []string {
	out := make([]string, 0, len(Aliases))
	for k := range Aliases {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
