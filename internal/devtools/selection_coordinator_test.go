package devtools

import "testing"

// TestPrismSelectionCoordinator asserts <prism-coordinator> per D082:
// click on an overview chart's bar re-dispatches the prism:select
// event on every sibling that declares the same selection ID, with
// the __prism_coordinated__ marker on the detail. No event-storm loop.
func TestPrismSelectionCoordinator(t *testing.T) {
	root, nodePath := crossImplPreflight(t)
	out, err := runHarness(t, root, nodePath, "selection-coordinator.mjs")
	if err != nil {
		t.Fatalf("selection-coordinator: %v\noutput:\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("output:\n%s", out)
	}
}
