package compile

import (
	"sort"

	"github.com/frankbardon/pulse/types"
)

// AggregateMapping resolves a Prism alias to its Pulse counterpart
// (Type) plus any required params. When Type == "" the alias has no
// Pulse equivalent in v0.8.4 — the in-memory backend implements
// these (`wmean`, `ratio`, `lift`, `share`, `ci0`, `ci1`). See D034.
type AggregateMapping struct {
	// Alias is the friendly spec-level name (`mean`, `sum`, …).
	Alias string
	// Type is the Pulse AggregationType constant; "" when no Pulse
	// equivalent exists in v0.8.4.
	Type types.AggregationType
	// Params is the JSON-encoded params blob Pulse expects for
	// parameterised aggregators (e.g. percentile). nil for the
	// parameterless ops.
	Params []byte
}

// IsDeferredFromPulse reports whether this alias has no Pulse
// equivalent in the pinned facade version and so must be executed
// client-side by the in-memory backend.
func (m AggregateMapping) IsDeferredFromPulse() bool { return m.Type == "" }

// AliasToPulse is the single source of truth for friendly aggregate
// alias resolution. Mirrors validate/rules/agg_compat.go's
// quantitative-op list verbatim — adding a new alias requires editing
// both (a TODO in agg_compat.go points here).
var AliasToPulse = map[string]AggregateMapping{
	"count":    {Alias: "count", Type: types.AGG_COUNT},
	"sum":      {Alias: "sum", Type: types.AGG_SUM},
	"mean":     {Alias: "mean", Type: types.AGG_AVERAGE},
	"median":   {Alias: "median", Type: types.AGG_MEDIAN},
	"min":      {Alias: "min", Type: types.AGG_MIN},
	"max":      {Alias: "max", Type: types.AGG_MAX},
	"stdev":    {Alias: "stdev", Type: types.AGG_STDDEV},
	"variance": {Alias: "variance", Type: types.AGG_VARIANCE},
	"mode":     {Alias: "mode", Type: types.AGG_MODE},
	"distinct": {Alias: "distinct", Type: types.AGG_DISTINCT_COUNT},
	"q1":       {Alias: "q1", Type: types.AGG_PERCENTILE, Params: []byte(`{"percentile":25}`)},
	"q3":       {Alias: "q3", Type: types.AGG_PERCENTILE, Params: []byte(`{"percentile":75}`)},

	// Cohort-analytics extensions (D003): first-class Prism ops with
	// no Pulse equivalent in v0.8.4. Executed by the in-memory
	// backend. Convention for sibling columns documented per alias.
	"ci0":   {Alias: "ci0"},   // 95% lower CI of mean over the group
	"ci1":   {Alias: "ci1"},   // 95% upper CI of mean over the group
	"wmean": {Alias: "wmean"}, // weighted mean; weight column = <field>_weight
	"ratio": {Alias: "ratio"}, // first(field)/first(<field>_denom)
	"lift":  {Alias: "lift"},  // mean(field)/mean(<field>_baseline)
	"share": {Alias: "share"}, // sum(field)/sum(field across whole input)
}

// AllAliases returns the alias names in sorted order. Stable order
// matters for the enumeration test and for fixup messages.
func AllAliases() []string {
	out := make([]string, 0, len(AliasToPulse))
	for k := range AliasToPulse {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// PulseBackedAliases returns the subset of aliases whose Type is
// non-empty (i.e. the parity-test eligible set). Useful for tests
// that compare Prism output to pulse.Process output.
func PulseBackedAliases() []string {
	out := make([]string, 0)
	for _, k := range AllAliases() {
		if !AliasToPulse[k].IsDeferredFromPulse() {
			out = append(out, k)
		}
	}
	return out
}
