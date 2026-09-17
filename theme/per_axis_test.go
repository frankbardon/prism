package theme

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/spec"
)

// E8-S1 — `axis_x` / `axis_y` layer over the shared `axis` block per
// property, which is also what makes vertical and horizontal grid
// lines separately themeable.

func f(v float64) *float64 { return &v }

// The headline user-facing outcome: setting only axis_x.grid_color
// recolours the vertical grid lines and leaves the horizontal ones on
// the shared value.
func TestPrismThemeAxisXGridColorScopesToVerticalGridLines(t *testing.T) {
	th := &Theme{
		Name:  "t",
		Axis:  &AxisStyle{GridColor: "#cccccc"},
		AxisX: &AxisStyle{GridColor: "#ff0000"},
	}
	css := th.CSSVariables()

	if !strings.Contains(css, ":root{--prism-grid-color:#cccccc;}") {
		t.Errorf("shared axis.grid_color did not stay on :root:\n%s", css)
	}
	if !strings.Contains(css, ".prism-axis-x{--prism-grid-color:#ff0000;}") {
		t.Errorf("axis_x.grid_color was not scoped to the x-axis group:\n%s", css)
	}
	if strings.Contains(css, ".prism-axis-y{") {
		t.Errorf("an unset axis_y still emitted a scope block:\n%s", css)
	}

	// The same statement as a resolved style: x gets the override, y
	// keeps the shared value.
	if got := th.AxisFor("x").GridColor; got != "#ff0000" {
		t.Errorf("AxisFor(x).GridColor = %q, want #ff0000", got)
	}
	if got := th.AxisFor("y").GridColor; got != "#cccccc" {
		t.Errorf("AxisFor(y).GridColor = %q, want the shared #cccccc", got)
	}
}

// A per-axis block overrides property by property, never wholesale.
func TestPrismThemeAxisForMergesPerProperty(t *testing.T) {
	th := &Theme{
		Axis: &AxisStyle{
			GridColor:  "#cccccc",
			TickColor:  "#888888",
			LabelColor: "#111111",
			TickSize:   f(5),
			GridDash:   []float64{2, 2},
		},
		AxisY: &AxisStyle{GridColor: "#0000ff"},
	}
	got := th.AxisFor("y")
	if got.GridColor != "#0000ff" {
		t.Errorf("GridColor = %q, want the axis_y override", got.GridColor)
	}
	for _, c := range []struct{ name, got, want string }{
		{"TickColor", got.TickColor, "#888888"},
		{"LabelColor", got.LabelColor, "#111111"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want the inherited %q", c.name, c.got, c.want)
		}
	}
	if got.TickSize == nil || *got.TickSize != 5 {
		t.Errorf("TickSize = %v, want the inherited 5", got.TickSize)
	}
	if len(got.GridDash) != 2 {
		t.Errorf("GridDash = %v, want the inherited [2 2]", got.GridDash)
	}

	// Merging must not write back into the source blocks.
	got.GridDash[0] = 99
	if th.Axis.GridDash[0] != 2 {
		t.Error("AxisFor aliased the shared block's GridDash")
	}
}

// x2 / y2 resolve to the same block as their base channel; an
// unrelated channel falls back to the shared block alone.
func TestPrismThemeAxisForChannelMapping(t *testing.T) {
	th := &Theme{
		Axis:  &AxisStyle{GridColor: "#cccccc"},
		AxisX: &AxisStyle{GridColor: "#ff0000"},
		AxisY: &AxisStyle{GridColor: "#0000ff"},
	}
	for _, tc := range []struct{ channel, want string }{
		{"x", "#ff0000"},
		{"x2", "#ff0000"},
		{"y", "#0000ff"},
		{"y2", "#0000ff"},
		{"color", "#cccccc"},
	} {
		if got := th.AxisFor(tc.channel).GridColor; got != tc.want {
			t.Errorf("AxisFor(%q).GridColor = %q, want %q", tc.channel, got, tc.want)
		}
	}
	if (*Theme)(nil).AxisFor("x") != nil {
		t.Error("AxisFor on a nil theme must return nil")
	}
}

// Purely additive: a theme that states neither block emits exactly the
// CSS it did before they existed.
func TestPrismThemeNoPerAxisBlocksIsByteIdentical(t *testing.T) {
	base := &Theme{Name: "t", Axis: &AxisStyle{GridColor: "#cccccc"}}
	before := base.CSSVariables()
	if strings.Contains(before, ".prism-axis-x{") || strings.Contains(before, ".prism-axis-y{") {
		t.Fatalf("a theme with no per-axis block emitted a scope selector:\n%s", before)
	}
	// An empty (but non-nil) block also contributes nothing.
	empty := base.Clone()
	empty.AxisX = &AxisStyle{}
	if got := empty.CSSVariables(); got != before {
		t.Errorf("an empty axis_x changed the CSS:\n%s", got)
	}
}

// Clone deep-copies the per-axis blocks.
func TestPrismThemeClonePerAxis(t *testing.T) {
	th := &Theme{
		AxisX: &AxisStyle{GridColor: "#ff0000", GridDash: []float64{1, 2}},
		AxisY: &AxisStyle{GridColor: "#0000ff"},
	}
	cp := th.Clone()
	cp.AxisX.GridColor = "#00ff00"
	cp.AxisX.GridDash[0] = 9
	cp.AxisY.GridColor = "#00ff00"
	if th.AxisX.GridColor != "#ff0000" || th.AxisX.GridDash[0] != 1 || th.AxisY.GridColor != "#0000ff" {
		t.Error("Clone aliased a per-axis block")
	}
}

// Merge folds each per-axis block against its own counterpart, never
// against the shared one.
func TestPrismThemeMergePerAxis(t *testing.T) {
	base := &Theme{
		Axis:  &AxisStyle{GridColor: "#cccccc", TickColor: "#888888"},
		AxisX: &AxisStyle{GridColor: "#ff0000", LabelColor: "#222222"},
	}
	out := Merge(base, &Theme{AxisX: &AxisStyle{GridColor: "#00ff00"}})
	if out.AxisX.GridColor != "#00ff00" {
		t.Errorf("AxisX.GridColor = %q, want the override", out.AxisX.GridColor)
	}
	if out.AxisX.LabelColor != "#222222" {
		t.Errorf("AxisX.LabelColor = %q, want the base value kept", out.AxisX.LabelColor)
	}
	if out.AxisX.TickColor != "" {
		t.Error("the shared axis block leaked into axis_x at merge time")
	}
	if out.Axis.GridColor != "#cccccc" {
		t.Error("axis_x overwrote the shared axis block")
	}
}

// A spec-level `theme` override reaches the per-axis blocks.
func TestPrismThemeApplyOverridePerAxis(t *testing.T) {
	base := &Theme{Name: "light", Axis: &AxisStyle{GridColor: "#cccccc"}}
	out := ApplyOverride(base, &spec.ThemeOverride{
		AxisX: &spec.AxisStyle{GridColor: "#ff0000"},
		AxisY: &spec.AxisStyle{GridDash: []float64{4, 4}},
	})
	if out.AxisX == nil || out.AxisX.GridColor != "#ff0000" {
		t.Errorf("axis_x did not survive the spec override: %+v", out.AxisX)
	}
	if out.AxisY == nil || len(out.AxisY.GridDash) != 2 {
		t.Errorf("axis_y did not survive the spec override: %+v", out.AxisY)
	}
	if out.Axis.GridColor != "#cccccc" {
		t.Error("the spec override disturbed the shared axis block")
	}
}

// Only the tokens a CSS variable cannot express reach the Scene IR.
func TestPrismThemeToSceneThemePerAxis(t *testing.T) {
	th := &Theme{
		Axis:  &AxisStyle{TickSize: f(5)},
		AxisX: &AxisStyle{TickSize: f(12), GridColor: "#ff0000"},
		AxisY: &AxisStyle{GridColor: "#0000ff"},
	}
	sc := th.ToSceneTheme()
	if sc.AxisX == nil || sc.AxisX.TickSize == nil || *sc.AxisX.TickSize != 12 {
		t.Fatalf("axis_x.tick_size did not reach the Scene IR: %+v", sc.AxisX)
	}
	if sc.AxisY != nil {
		t.Errorf("a colour-only axis_y added Scene IR bytes: %+v", sc.AxisY)
	}
	*sc.AxisX.TickSize = 99
	if *th.AxisX.TickSize != 12 {
		t.Error("ToSceneTheme aliased the source theme")
	}
}

// A per-axis filter reference is checked against theme.Filters the
// same way the shared block's is.
func TestPrismThemePerAxisFilterRefValidated(t *testing.T) {
	th := &Theme{Name: "t", AxisX: &AxisStyle{Filter: "nope"}}
	if err := th.Validate(); err == nil {
		t.Fatal("an unresolved axis_x.filter must fail validation")
	}
	th.Filters = map[string]string{"nope": "<feGaussianBlur/>"}
	if err := th.Validate(); err != nil {
		t.Fatalf("a resolved axis_x.filter must validate: %v", err)
	}
}
