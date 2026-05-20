package devtools

import "testing"

// TestPrismSelectionPointDispatch — mandatory PHASE.md gate.
// Asserts a click on a mark <rect data-prism-datum-row="0"> fires a
// prism:select CustomEvent with detail
// {id: "highlight", state: {points: [{layer_id:"layer-0", row_id:0}], range:null}}.
func TestPrismSelectionPointDispatch(t *testing.T) {
	root, nodePath := crossImplPreflight(t)
	out, err := runHarness(t, root, nodePath, "selection-point-dispatch.mjs")
	if err != nil {
		t.Fatalf("selection-point-dispatch: %v\noutput:\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("output:\n%s", out)
	}
}
