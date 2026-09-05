package devtools

import "testing"

// TestPrismTableInteractivity drives table-interactivity.mjs, which
// exercises static/vendor/prism/prism-table.mjs (E1-S5) against the
// exact data-prism-* attribute contract render/html/renderer.go
// emits: header-click sort by underlying field value (including a
// non-textual, array-valued column standing in for a sparkline
// sub-mark), client-side pagination re-slicing, and row-click
// selection dispatching a selection.Event-shaped `prism:select`.
// No WASM required — a table mark never renders through prism.wasm's
// SVG-only bridge, so the harness builds fixture markup by hand.
func TestPrismTableInteractivity(t *testing.T) {
	root, nodePath := crossImplPreflight(t)
	out, err := runHarness(t, root, nodePath, "table-interactivity.mjs")
	if err != nil {
		t.Fatalf("table-interactivity: %v\noutput:\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("output:\n%s", out)
	}
}
