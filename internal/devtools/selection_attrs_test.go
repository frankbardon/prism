package devtools

import "testing"

// TestPrismSelectionHitTestAttrs renders a scene via prism.mjs and
// asserts every per-row mark carries data-prism-datum-row + its
// layer group carries data-prism-layer (D077). Mirrors the Go-side
// gates in render/svg/marks_datum_test.go for JS parity.
func TestPrismSelectionHitTestAttrs(t *testing.T) {
	root, nodePath := crossImplPreflight(t)
	out, err := runHarness(t, root, nodePath, "selection-hit-test-attrs.mjs")
	if err != nil {
		t.Fatalf("selection-hit-test-attrs: %v\noutput:\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("output:\n%s", out)
	}
}
