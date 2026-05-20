package devtools

import "testing"

// TestPrismSelectionURLState asserts the URL-hash + localStorage
// round-trip lands per D079: chart seeds initial state from hash,
// mutation rewrites hash, oversized payloads overflow to
// localStorage with hash = "#prism-sel:overflow". Mandatory gate
// per PHASE.md.
func TestPrismSelectionURLState(t *testing.T) {
	root, nodePath := crossImplPreflight(t)
	out, err := runHarness(t, root, nodePath, "selection-url-state.mjs")
	if err != nil {
		t.Fatalf("selection-url-state: %v\noutput:\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("output:\n%s", out)
	}
}
