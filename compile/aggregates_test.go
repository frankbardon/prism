package compile

import (
	"sort"
	"testing"
)

// expectedAliases mirrors the 23 friendly aliases the validator
// accepts (see validate/rules/agg_compat.go). Drift between the
// validator and this map breaks the alias passthrough —
// TestPrismAggOpEnumCoverage catches it.
var expectedAliases = []string{
	"ci0", "ci1", "count", "distinct", "frequency", "kurtosis", "lift",
	"max", "mean", "median", "min", "mode", "null_count", "q1", "q3",
	"range", "ratio", "share", "skewness", "stdev", "sum", "variance",
	"wmean",
}

// TestPrismAggOpEnumCoverage is the PHASE.md test gate: every Prism
// alias must have an entry in the shared aggregate registry
// (compile.Aliases). The registry is a name catalogue only — every
// alias is computed client-side by the in-memory backend.
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

	for _, alias := range got {
		if !IsAlias(alias) {
			t.Errorf("alias %q not in Aliases", alias)
		}
	}
}

// TestPrismAggOpIsAlias asserts IsAlias accepts every catalogued alias
// and rejects an unknown one.
func TestPrismAggOpIsAlias(t *testing.T) {
	for _, alias := range expectedAliases {
		if !IsAlias(alias) {
			t.Errorf("IsAlias(%q) = false; want true", alias)
		}
	}
	if IsAlias("not_an_aggregate") {
		t.Error("IsAlias(\"not_an_aggregate\") = true; want false")
	}
}
