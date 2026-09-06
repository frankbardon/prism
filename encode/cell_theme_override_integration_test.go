package encode_test

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// cellMarkFillHex walks a cell's Scene down to its first mark's
// resolved Style.Fill and returns its hex, or "" when the mark
// carries no baked fill color (e.g. a FillVar / FillRef path, or no
// mark at all).
func cellMarkFillHex(layers []scene.SceneLayer) string {
	for _, layer := range layers {
		for _, m := range layer.Marks {
			if m.Style.Fill != nil {
				return m.Style.Fill.Hex()
			}
		}
	}
	return ""
}

// cellMarkStrokeHex mirrors cellMarkFillHex for Style.Stroke.
func cellMarkStrokeHex(layers []scene.SceneLayer) string {
	for _, layer := range layers {
		for _, m := range layer.Marks {
			if m.Style.Stroke != nil {
				return m.Style.Stroke.Hex()
			}
		}
	}
	return ""
}

// TestPrismEncodeFacetCellThemeOverride pins E5-S2's core behaviour:
// facet_cell_theme_override.json overrides bar fill on cell (0,0) and
// (0,1) and leaves (0,2) on the base theme. Each cell's child scene
// must reflect its own resolved theme.
func TestPrismEncodeFacetCellThemeOverride(t *testing.T) {
	_, _, _, doc := runFacetSpec(t, "facet_cell_theme_override.json")
	if len(doc.Grid.Cells) != 3 {
		t.Fatalf("cells = %d, want 3", len(doc.Grid.Cells))
	}
	want := map[int]string{
		0: "#e11d48", // region NA, col 0 — overridden red
		1: "#16a34a", // region EU, col 1 — overridden green
		2: "#4c78a8", // region APAC, col 2 — no override, base light-theme bar fill
	}
	seen := map[int]bool{}
	for _, cell := range doc.Grid.Cells {
		got := cellMarkFillHex(cell.Scene.Layers)
		if want[cell.Col] != got {
			t.Errorf("cell col %d: fill = %q, want %q", cell.Col, got, want[cell.Col])
		}
		seen[cell.Col] = true
	}
	for col := range want {
		if !seen[col] {
			t.Errorf("expected a cell at col %d, got none", col)
		}
	}
}

// TestPrismEncodeFacetCellThemeOverride_ScaleResolutionUnaffected
// asserts per-cell theme overrides don't couple into cross-layer
// scale resolution (E5-S2 AC #2): facet_cell_theme_override.json
// sets no `resolve` block, so the D057 default (shared y) must still
// hold even though two of its three cells carry a theme override —
// a shared y axis, and no per-cell y axes.
func TestPrismEncodeFacetCellThemeOverride_ScaleResolutionUnaffected(t *testing.T) {
	_, _, _, doc := runFacetSpec(t, "facet_cell_theme_override.json")
	if doc.Grid.Shared.Y == nil {
		t.Error("Grid.Shared.Y is nil; per-cell theme overrides must not disable default shared-y resolve (D057)")
	}
	for i, cell := range doc.Grid.Cells {
		for _, ax := range cell.Scene.Axes {
			if ax.Channel == scene.ChannelY {
				t.Errorf("cell %d still carries a y-axis under shared resolve; a theme override must not force independent resolution", i)
			}
		}
	}
}

// TestPrismEncodeRepeatCellThemeOverride mirrors the facet case for
// repeat: repeat_cell_theme_override.json overrides line stroke on
// cell (0,0) and (0,1), leaving (0,2) on the base theme.
func TestPrismEncodeRepeatCellThemeOverride(t *testing.T) {
	_, _, _, doc := runFacetSpec(t, "repeat_cell_theme_override.json")
	if len(doc.Grid.Cells) != 3 {
		t.Fatalf("cells = %d, want 3", len(doc.Grid.Cells))
	}
	want := map[int]string{
		0: "#e11d48", // score — overridden red
		1: "#16a34a", // latency_ms — overridden green
		2: "#4c78a8", // uptime — no override, base light-theme line stroke
	}
	seen := map[int]bool{}
	for _, cell := range doc.Grid.Cells {
		got := cellMarkStrokeHex(cell.Scene.Layers)
		if want[cell.Col] != got {
			t.Errorf("cell col %d: stroke = %q, want %q", cell.Col, got, want[cell.Col])
		}
		seen[cell.Col] = true
	}
	for col := range want {
		if !seen[col] {
			t.Errorf("expected a cell at col %d, got none", col)
		}
	}
}

// TestPrismEncodeRepeatCellThemeOverride_ScaleResolutionUnaffected
// mirrors the facet-side AC #2 check for repeat: no `resolve` block
// is set, so the D057 default (independent x/y) must still hold —
// each cell keeps its own y-axis — even though two of its three
// cells carry a theme override.
func TestPrismEncodeRepeatCellThemeOverride_ScaleResolutionUnaffected(t *testing.T) {
	_, _, _, doc := runFacetSpec(t, "repeat_cell_theme_override.json")
	if doc.Grid.Shared.Y != nil {
		t.Error("Grid.Shared.Y must stay nil under repeat's default independent-y resolve, regardless of per-cell theme overrides")
	}
	for _, cell := range doc.Grid.Cells {
		hasY := false
		for _, ax := range cell.Scene.Axes {
			if ax.Channel == scene.ChannelY {
				hasY = true
			}
		}
		if !hasY {
			t.Errorf("cell col %d missing its own y-axis under independent resolve", cell.Col)
		}
	}
}
