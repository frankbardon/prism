package compile

import (
	"sort"
	"testing"

	"github.com/frankbardon/pulse/types"
)

// expectedAliases mirrors the 22 friendly aliases the validator
// accepts (see validate/rules/agg_compat.go). Drift between the
// validator and this map breaks the alias passthrough —
// TestPrismAggOpEnumCoverage catches it.
var expectedAliases = []string{
	"ci0", "ci1", "count", "distinct", "kurtosis", "lift", "max", "mean",
	"median", "min", "mode", "null_count", "q1", "q3", "range", "ratio",
	"share", "skewness", "stdev", "sum", "variance", "wmean",
}

// TestPrismAggOpEnumCoverage is the PHASE.md test gate: every Prism
// alias must have an entry in AliasToPulse. Entries either resolve to
// a Pulse AggregationType or carry the deferred-from-Pulse marker
// (Type == "").
func TestPrismAggOpEnumCoverage(t *testing.T) {
	got := AllAliases()
	if len(got) != len(expectedAliases) {
		t.Fatalf("AllAliases len = %d, want %d (got=%v)", len(got), len(expectedAliases), got)
	}

	want := make([]string, len(expectedAliases))
	copy(want, expectedAliases)
	sort.Strings(want)
	for i, a := range got {
		if a != want[i] {
			t.Fatalf("AllAliases[%d] = %q, want %q (full=%v)", i, a, want[i], got)
		}
	}

	deferred := map[string]bool{
		"lift": true, "share": true,
	}
	for _, alias := range got {
		m, ok := AliasToPulse[alias]
		if !ok {
			t.Errorf("alias %q not in AliasToPulse", alias)
			continue
		}
		if m.Alias != alias {
			t.Errorf("alias %q: mapping.Alias = %q (drift)", alias, m.Alias)
		}
		if deferred[alias] {
			if !m.IsDeferredFromPulse() {
				t.Errorf("alias %q: expected deferred-from-Pulse, got Pulse Type %q", alias, m.Type)
			}
		} else {
			if m.IsDeferredFromPulse() {
				t.Errorf("alias %q: expected a Pulse Type, got deferred marker", alias)
			}
		}
	}
}

// TestPrismAggOpPulseTypesValid asserts every non-deferred mapping
// resolves to a real types.AggregationType the Pulse package
// recognises. Catches typos in AliasToPulse at compile/test time
// instead of at execute time.
func TestPrismAggOpPulseTypesValid(t *testing.T) {
	valid := map[types.AggregationType]bool{}
	for _, at := range types.AllAggregationTypes() {
		valid[at] = true
	}
	for alias, m := range AliasToPulse {
		if m.IsDeferredFromPulse() {
			continue
		}
		if !valid[m.Type] {
			t.Errorf("alias %q resolves to %q which is not in types.AllAggregationTypes()", alias, m.Type)
		}
	}
}

// TestPrismPulseBackedAliasesSubset asserts PulseBackedAliases() is
// the complement of the deferred set inside the full alias enum.
func TestPrismPulseBackedAliasesSubset(t *testing.T) {
	backed := PulseBackedAliases()
	for _, alias := range backed {
		m := AliasToPulse[alias]
		if m.IsDeferredFromPulse() {
			t.Errorf("PulseBackedAliases includes deferred alias %q", alias)
		}
	}
	if len(backed)+2 != len(AllAliases()) {
		t.Errorf("PulseBackedAliases len = %d; expected 20 (22 total - 2 deferred)", len(backed))
	}
}
