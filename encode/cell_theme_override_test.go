package encode

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/theme"
)

// TestFindCellThemeOverride pins the (row, col) addressing contract
// documented on spec.CellThemeOverride: an entry is returned only
// when both Row and Column match exactly; no match returns nil so
// callers fall back to the base theme unchanged.
func TestFindCellThemeOverride(t *testing.T) {
	overrides := []spec.CellThemeOverride{
		{Row: 0, Column: 0, Theme: spec.ThemeOverride{Background: "red"}},
		{Row: 0, Column: 1, Theme: spec.ThemeOverride{Background: "green"}},
		{Row: 1, Column: 0, Theme: spec.ThemeOverride{Background: "blue"}},
	}

	cases := []struct {
		row, col int
		want     string // expected Background, "" for no match
	}{
		{0, 0, "red"},
		{0, 1, "green"},
		{1, 0, "blue"},
		{1, 1, ""}, // no entry addresses this cell
		{2, 2, ""}, // out of range entirely
	}
	for _, c := range cases {
		got := findCellThemeOverride(overrides, c.row, c.col)
		if c.want == "" {
			if got != nil {
				t.Errorf("(%d,%d): got override %+v, want nil", c.row, c.col, got)
			}
			continue
		}
		if got == nil {
			t.Fatalf("(%d,%d): got nil, want Background=%q", c.row, c.col, c.want)
		}
		if got.Background != c.want {
			t.Errorf("(%d,%d): Background = %q, want %q", c.row, c.col, got.Background, c.want)
		}
	}
}

// TestFindCellThemeOverride_LastMatchWins covers the (unusual, but
// possible) case of two entries addressing the same cell: the later
// entry in the list wins, matching a top-to-bottom reading order.
func TestFindCellThemeOverride_LastMatchWins(t *testing.T) {
	overrides := []spec.CellThemeOverride{
		{Row: 0, Column: 0, Theme: spec.ThemeOverride{Background: "first"}},
		{Row: 0, Column: 0, Theme: spec.ThemeOverride{Background: "second"}},
	}
	got := findCellThemeOverride(overrides, 0, 0)
	if got == nil || got.Background != "second" {
		t.Fatalf("got %+v, want Background=\"second\"", got)
	}
}

// TestFindCellThemeOverride_Empty covers the zero-overrides case
// (the common one — most facet/repeat specs never set cell_overrides).
func TestFindCellThemeOverride_Empty(t *testing.T) {
	if got := findCellThemeOverride(nil, 0, 0); got != nil {
		t.Errorf("got %+v, want nil for empty overrides", got)
	}
}

// TestResolveCellTheme_NilOverridePassesThrough asserts the no-op
// path returns the exact same pointers handed in (no allocation, no
// re-merge) when a cell has no matching override — the common case,
// and the one that must keep existing goldens byte-identical.
func TestResolveCellTheme_NilOverridePassesThrough(t *testing.T) {
	base, ok := theme.Get("light")
	if !ok {
		t.Fatal("light theme not registered")
	}
	baseScene := base.ToSceneTheme()
	gotScene, gotFull := resolveCellTheme(baseScene, base, nil)
	if gotScene != baseScene {
		t.Error("resolveCellTheme with nil override returned a different *scene.Theme pointer")
	}
	if gotFull != base {
		t.Error("resolveCellTheme with nil override returned a different *theme.Theme pointer")
	}
}

// TestResolveCellTheme_AppliesOverride confirms a non-nil override
// is layered on top of the base theme via theme.ApplyOverride (the
// same merge machinery the spec-level `theme` override uses) and
// that the result differs from the base while leaving the base
// theme itself untouched.
func TestResolveCellTheme_AppliesOverride(t *testing.T) {
	base, ok := theme.Get("light")
	if !ok {
		t.Fatal("light theme not registered")
	}
	baseScene := base.ToSceneTheme()
	baseFillBefore := base.MarkDefault("bar")

	// light theme sets an explicit per-type Marks["bar"].Fill, so the
	// override must target the same per-type slot to take effect —
	// mirrors the fixture specs under examples/specs/*_cell_theme_override.json.
	override := &spec.ThemeOverride{Marks: map[string]*spec.MarkStyle{"bar": {Fill: "#e11d48"}}}
	gotScene, gotFull := resolveCellTheme(baseScene, base, override)

	if gotFull == base {
		t.Fatal("resolveCellTheme with a non-nil override must not return the base *theme.Theme unchanged")
	}
	if gotScene == baseScene {
		t.Fatal("resolveCellTheme with a non-nil override must not return the base *scene.Theme unchanged")
	}
	ms := gotFull.MarkDefault("bar")
	if ms == nil || ms.Fill != "#e11d48" {
		t.Fatalf("merged theme bar fill = %+v, want #e11d48", ms)
	}
	// Base theme must be unmutated by the merge (ApplyOverride clones).
	afterBase := base.MarkDefault("bar")
	if baseFillBefore != nil && afterBase != nil && baseFillBefore.Fill != afterBase.Fill {
		t.Error("resolveCellTheme mutated the base theme's mark default")
	}
}

// TestFindCellThemeOverride_NCellsMOverrides is a small table-driven
// simulation of a grid with N cells and M sparse overrides, checking
// that each cell resolves to the correct override (or none) per the
// acceptance criteria's "given N cells and M overrides" requirement.
func TestFindCellThemeOverride_NCellsMOverrides(t *testing.T) {
	const rows, cols = 2, 3
	overrides := []spec.CellThemeOverride{
		{Row: 0, Column: 0, Theme: spec.ThemeOverride{Background: "r0c0"}},
		{Row: 1, Column: 2, Theme: spec.ThemeOverride{Background: "r1c2"}},
	}
	want := map[[2]int]string{
		{0, 0}: "r0c0",
		{1, 2}: "r1c2",
	}
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			got := findCellThemeOverride(overrides, r, c)
			wantBG, shouldMatch := want[[2]int{r, c}]
			if shouldMatch {
				if got == nil || got.Background != wantBG {
					t.Errorf("cell (%d,%d): got %+v, want Background=%q", r, c, got, wantBG)
				}
			} else if got != nil {
				t.Errorf("cell (%d,%d): got %+v, want nil (no override addresses this cell)", r, c, got)
			}
		}
	}
}
