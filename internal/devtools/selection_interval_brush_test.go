package devtools

import "testing"

// TestPrismSelectionIntervalBrush — mandatory PHASE.md gate.
// Asserts a synthetic mousedown/mousemove/mouseup over the plot
// region fires prism:select with detail.state.range = {channel:"x",
// min, max} where min < max and both lie in the x scale's domain.
func TestPrismSelectionIntervalBrush(t *testing.T) {
	root, nodePath := crossImplPreflight(t)
	out, err := runHarness(t, root, nodePath, "selection-interval-brush.mjs")
	if err != nil {
		t.Fatalf("selection-interval-brush: %v\noutput:\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("output:\n%s", out)
	}
}
