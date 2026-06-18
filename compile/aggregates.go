package compile

import (
	"sort"

	"github.com/frankbardon/pulse/types"
)

// AggregateMapping resolves a Prism alias to its Pulse counterpart
// (Type) plus any required params. When Type == "" the alias has no
// Pulse equivalent in the pinned facade version — the in-memory
// backend implements these (`lift`, `share` as of v0.10.0). See D034.
type AggregateMapping struct {
	// Alias is the friendly spec-level name (`mean`, `sum`, …).
	Alias string
	// Type is the Pulse AggregationType constant; "" when no Pulse
	// equivalent exists in the pinned facade version.
	Type types.AggregationType
	// Params is the JSON-encoded params blob Pulse expects when the
	// alias resolves to a parameterised aggregator and the params are
	// statically derivable from the alias (e.g. `q1` always means
	// percentile=25). Aliases whose params depend on the per-call
	// AggOp (e.g. wmean's weight_field, ratio's numerator_field /
	// denominator_field) leave this nil — callers synthesize Params
	// from the AggOp at request-build time, using the documented
	// sibling-column conventions.
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
//
// Pulse v0.10.0 promoted `wmean`, `ratio`, `ci0`, `ci1` from
// inmem-only to first-class AGG_* constants. `lift` and `share`
// remain deferred until Pulse upstreams them.
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

	// frequency is the SCALAR companion to mode: AGG_FREQUENCY's
	// Finalize() return is the modal count (occurrences of the most
	// frequent value), so it rides the F64 pipeline like every other
	// scalar aggregate. The per-value cardinality map lives only on
	// Pulse's MetaAggregator Components()/Rich() surface, which Prism's
	// row-shaped Response.Data path does not consume — so no map column
	// kind is required. Universal: counts occurrences of any field type.
	"frequency": {Alias: "frequency", Type: types.AGG_FREQUENCY},
	"q1":        {Alias: "q1", Type: types.AGG_PERCENTILE, Params: []byte(`{"percentile":25}`)},
	"q3":        {Alias: "q3", Type: types.AGG_PERCENTILE, Params: []byte(`{"percentile":75}`)},

	// Distribution-shape scalars promoted to first-class Pulse AGG_*
	// ops by v0.22. range = max-min; skewness/kurtosis are the
	// population (Fisher-Pearson) forms — kurtosis is excess (−3).
	// null_count counts null records and so applies to any field type.
	// AGG_ZSCORE is intentionally NOT aliased: its per-group scalar is
	// the mean standardized score, which is 0 by definition (it is a
	// per-row ATTR concept, not a group aggregate).
	"range":      {Alias: "range", Type: types.AGG_RANGE},
	"skewness":   {Alias: "skewness", Type: types.AGG_SKEWNESS},
	"kurtosis":   {Alias: "kurtosis", Type: types.AGG_KURTOSIS},
	"null_count": {Alias: "null_count", Type: types.AGG_NULL_COUNT},

	// Cohort-analytics extensions (D003) promoted in pulse v0.10.0.
	// Sibling-column conventions still bind the Pulse Params at
	// request-build time: wmean uses <field>_weight; ratio uses
	// <field> as numerator and <field>_denom as denominator;
	// ci0/ci1 default to 95% confidence via the "normal" method.
	"ci0":   {Alias: "ci0", Type: types.AGG_CI_LOWER},
	"ci1":   {Alias: "ci1", Type: types.AGG_CI_UPPER},
	"wmean": {Alias: "wmean", Type: types.AGG_WEIGHTED_MEAN},
	"ratio": {Alias: "ratio", Type: types.AGG_RATIO},

	// Cohort-analytics extensions not yet upstreamed by Pulse.
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
