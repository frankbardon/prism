package encode

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/theme"
)

// TestPrismMarkDefStrokeDashReachesStyle pins the E7-S4 wiring: a
// spec mark_def.stroke_dash lands on scene.Style.StrokeDash, which
// render/svg turns into stroke-dasharray. Nine committed fixtures
// asked for a dashed benchmark line and published a solid one until
// this existed.
func TestPrismMarkDefStrokeDashReachesStyle(t *testing.T) {
	style := scene.Style{}
	applyMarkDef(&spec.MarkDef{Type: "line", StrokeDash: []float64{4, 2}}, &style)

	if got := style.StrokeDash; len(got) != 2 || got[0] != 4 || got[1] != 2 {
		t.Fatalf("StrokeDash = %v, want [4 2]", got)
	}
}

// TestPrismStrokeDashSpecBeatsTheme covers the per-field precedence
// rule: applyThemeMarkStyle runs first, applyMarkDef shadows it, and
// a mark def that leaves stroke_dash unset keeps the theme's pattern.
// theme.MarkStyle and spec.MarkDef both spell the field StrokeDash —
// this is the test that catches the two being crossed.
func TestPrismStrokeDashSpecBeatsTheme(t *testing.T) {
	style := scene.Style{}
	applyThemeMarkStyle(&style, &theme.MarkStyle{StrokeDash: []float64{1, 1}}, &theme.Theme{}, nil, nil)
	if got := style.StrokeDash; len(got) != 2 || got[0] != 1 {
		t.Fatalf("theme StrokeDash not applied: %v", got)
	}

	// Mark def silent on stroke_dash: the theme token survives.
	applyMarkDef(&spec.MarkDef{Type: "line"}, &style)
	if got := style.StrokeDash; len(got) != 2 || got[0] != 1 {
		t.Fatalf("theme StrokeDash lost to a silent mark def: %v", got)
	}

	// Mark def sets it: the whole pattern is replaced, not merged.
	applyMarkDef(&spec.MarkDef{Type: "line", StrokeDash: []float64{6, 3, 2}}, &style)
	if got := style.StrokeDash; len(got) != 3 || got[0] != 6 || got[2] != 2 {
		t.Fatalf("StrokeDash = %v, want the spec's [6 3 2]", got)
	}
}

// TestPrismStrokeDashIsCopiedNotAliased guards the Scene IR against a
// later mutation of the spec slice reaching an already-encoded style.
func TestPrismStrokeDashIsCopiedNotAliased(t *testing.T) {
	dash := []float64{4, 2}
	style := scene.Style{}
	applyMarkDef(&spec.MarkDef{Type: "line", StrokeDash: dash}, &style)
	dash[0] = 99
	if style.StrokeDash[0] != 4 {
		t.Fatalf("StrokeDash aliases the spec slice: %v", style.StrokeDash)
	}
}
