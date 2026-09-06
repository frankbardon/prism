package theme

import (
	"testing"

	"github.com/frankbardon/prism/spec"
)

// TestMarkStyle_CloneMerge_Typography covers the Clone/Merge
// round-trip for the LineHeight/LetterSpacing fields on MarkStyle
// (E2-S1), mirroring TestMarkStyle_CloneMerge_Filter's coverage of
// the Filter field.
func TestMarkStyle_CloneMerge_Typography(t *testing.T) {
	lh, ls := 1.4, 0.5
	base := &MarkStyle{Fill: "#111", LineHeight: &lh, LetterSpacing: &ls}

	clone := base.Clone()
	if clone.LineHeight == nil || *clone.LineHeight != 1.4 {
		t.Fatalf("Clone: LineHeight = %v, want 1.4", clone.LineHeight)
	}
	if clone.LetterSpacing == nil || *clone.LetterSpacing != 0.5 {
		t.Fatalf("Clone: LetterSpacing = %v, want 0.5", clone.LetterSpacing)
	}
	*clone.LineHeight = 9
	*clone.LetterSpacing = 9
	if *base.LineHeight != 1.4 || *base.LetterSpacing != 0.5 {
		t.Fatalf("Clone: mutation leaked back into base (pointer aliasing)")
	}

	overrideLH, overrideLS := 2.0, 1.0
	override := &MarkStyle{LineHeight: &overrideLH, LetterSpacing: &overrideLS}
	merged := MergeMarkStyle(base, override)
	if merged.LineHeight == nil || *merged.LineHeight != 2.0 {
		t.Fatalf("MergeMarkStyle: LineHeight = %v, want 2.0 (override wins)", merged.LineHeight)
	}
	if merged.LetterSpacing == nil || *merged.LetterSpacing != 1.0 {
		t.Fatalf("MergeMarkStyle: LetterSpacing = %v, want 1.0 (override wins)", merged.LetterSpacing)
	}

	// A nil override field inherits base's value (sparse-merge idiom).
	merged2 := MergeMarkStyle(base, &MarkStyle{Fill: "#222"})
	if merged2.LineHeight == nil || *merged2.LineHeight != 1.4 {
		t.Fatalf("MergeMarkStyle: LineHeight = %v, want 1.4 to be inherited", merged2.LineHeight)
	}
	if merged2.LetterSpacing == nil || *merged2.LetterSpacing != 0.5 {
		t.Fatalf("MergeMarkStyle: LetterSpacing = %v, want 0.5 to be inherited", merged2.LetterSpacing)
	}
}

// TestAxisLegendTitleStyle_MergeTypography covers the per-block merge
// helpers (mergeAxis/mergeLegend/mergeTitle) for the new
// LineHeight/LetterSpacing fields, including the Axis/Legend
// label vs. title sub-block split.
func TestAxisLegendTitleStyle_MergeTypography(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	base := &Theme{
		Axis: &AxisStyle{
			LabelLineHeight: f(1.1), LabelLetterSpacing: f(0.1),
			TitleLineHeight: f(1.2), TitleLetterSpacing: f(0.2),
		},
		Legend: &LegendStyle{
			LabelLineHeight: f(1.3), LabelLetterSpacing: f(0.3),
			TitleLineHeight: f(1.4), TitleLetterSpacing: f(0.4),
		},
		Title: &TitleStyle{LineHeight: f(1.5), LetterSpacing: f(0.5)},
	}
	override := &Theme{
		Axis: &AxisStyle{
			LabelLineHeight: f(2.1), LabelLetterSpacing: f(1.1),
			TitleLineHeight: f(2.2), TitleLetterSpacing: f(1.2),
		},
		Legend: &LegendStyle{
			LabelLineHeight: f(2.3), LabelLetterSpacing: f(1.3),
			TitleLineHeight: f(2.4), TitleLetterSpacing: f(1.4),
		},
		Title: &TitleStyle{LineHeight: f(2.5), LetterSpacing: f(1.5)},
	}

	merged := Merge(base, override)
	if *merged.Axis.LabelLineHeight != 2.1 || *merged.Axis.LabelLetterSpacing != 1.1 ||
		*merged.Axis.TitleLineHeight != 2.2 || *merged.Axis.TitleLetterSpacing != 1.2 {
		t.Fatalf("Merge: Axis typography override did not win: %+v", merged.Axis)
	}
	if *merged.Legend.LabelLineHeight != 2.3 || *merged.Legend.LabelLetterSpacing != 1.3 ||
		*merged.Legend.TitleLineHeight != 2.4 || *merged.Legend.TitleLetterSpacing != 1.4 {
		t.Fatalf("Merge: Legend typography override did not win: %+v", merged.Legend)
	}
	if *merged.Title.LineHeight != 2.5 || *merged.Title.LetterSpacing != 1.5 {
		t.Fatalf("Merge: Title typography override did not win: %+v", merged.Title)
	}

	// Nil fields in the override inherit from base.
	merged2 := Merge(base, &Theme{
		Axis:   &AxisStyle{},
		Legend: &LegendStyle{},
		Title:  &TitleStyle{},
	})
	if *merged2.Axis.LabelLineHeight != 1.1 || *merged2.Axis.TitleLetterSpacing != 0.2 {
		t.Fatalf("Merge: Axis typography did not inherit from base: %+v", merged2.Axis)
	}
	if *merged2.Legend.LabelLineHeight != 1.3 || *merged2.Legend.TitleLetterSpacing != 0.4 {
		t.Fatalf("Merge: Legend typography did not inherit from base: %+v", merged2.Legend)
	}
	if *merged2.Title.LineHeight != 1.5 || *merged2.Title.LetterSpacing != 0.5 {
		t.Fatalf("Merge: Title typography did not inherit from base: %+v", merged2.Title)
	}
}

// TestApplyOverride_Typography exercises the spec.ThemeOverride ->
// theme.Theme copy path (theme/override.go's copyAxisStyle /
// copyLegendStyle / copyTitleStyle / copyMarkStyle) for the new
// fields, ensuring a spec-level inline theme override can set them.
func TestApplyOverride_Typography(t *testing.T) {
	lh, ls := 1.6, 0.6
	o := &spec.ThemeOverride{
		Mark:   &spec.MarkStyle{LineHeight: &lh, LetterSpacing: &ls},
		Axis:   &spec.AxisStyle{LabelLineHeight: &lh, LabelLetterSpacing: &ls},
		Legend: &spec.LegendStyle{TitleLineHeight: &lh, TitleLetterSpacing: &ls},
		Title:  &spec.TitleStyle{LineHeight: &lh, LetterSpacing: &ls},
	}
	base := &Theme{}
	got := ApplyOverride(base, o)
	if got.Title == nil || got.Title.LineHeight == nil || *got.Title.LineHeight != 1.6 {
		t.Fatalf("ApplyOverride: Title.LineHeight = %v, want 1.6", got.Title)
	}
	if got.Title.LetterSpacing == nil || *got.Title.LetterSpacing != 0.6 {
		t.Fatalf("ApplyOverride: Title.LetterSpacing = %v, want 0.6", got.Title)
	}
	if got.Axis == nil || got.Axis.LabelLineHeight == nil || *got.Axis.LabelLineHeight != 1.6 {
		t.Fatalf("ApplyOverride: Axis.LabelLineHeight = %v, want 1.6", got.Axis)
	}
	if got.Legend == nil || got.Legend.TitleLetterSpacing == nil || *got.Legend.TitleLetterSpacing != 0.6 {
		t.Fatalf("ApplyOverride: Legend.TitleLetterSpacing = %v, want 0.6", got.Legend)
	}
	if got.Mark == nil || got.Mark.LineHeight == nil || *got.Mark.LineHeight != 1.6 {
		t.Fatalf("ApplyOverride: Mark.LineHeight = %v, want 1.6", got.Mark)
	}
}
