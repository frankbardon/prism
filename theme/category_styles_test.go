package theme

import (
	"testing"

	"github.com/frankbardon/prism/spec"
)

// TestCategoryStyles_Clone covers deep-copy semantics for the nested
// field->value->style map: mutating the clone (at any level: the
// outer map, the inner map, or a leaf MarkStyle's pointer field) must
// never leak back into the original.
func TestCategoryStyles_Clone(t *testing.T) {
	width := 1.5
	base := &Theme{
		CategoryStyles: map[string]map[string]*MarkStyle{
			"Origin": {
				"USA":    {Fill: "#4c78a8"},
				"Europe": {Fill: "#f58518", StrokeWidth: &width},
			},
		},
	}

	clone := base.Clone()

	// Mutate the clone's outer map — must not affect base.
	clone.CategoryStyles["Status"] = map[string]*MarkStyle{"ok": {Fill: "#000"}}
	if _, ok := base.CategoryStyles["Status"]; ok {
		t.Fatalf("Theme.Clone: outer map aliasing leaked into base")
	}

	// Mutate the clone's inner map — must not affect base.
	clone.CategoryStyles["Origin"]["Japan"] = &MarkStyle{Fill: "#e45756"}
	if _, ok := base.CategoryStyles["Origin"]["Japan"]; ok {
		t.Fatalf("Theme.Clone: inner map aliasing leaked into base")
	}

	// Mutate a cloned leaf MarkStyle's fields — must not affect base.
	clone.CategoryStyles["Origin"]["USA"].Fill = "#mutated"
	if base.CategoryStyles["Origin"]["USA"].Fill != "#4c78a8" {
		t.Fatalf("Theme.Clone: leaf MarkStyle string field aliasing leaked into base")
	}
	*clone.CategoryStyles["Origin"]["Europe"].StrokeWidth = 99
	if *base.CategoryStyles["Origin"]["Europe"].StrokeWidth != 1.5 {
		t.Fatalf("Theme.Clone: leaf MarkStyle pointer field aliasing leaked into base")
	}
}

// TestCategoryStyles_Clone_Nil covers the nil round-trip: Clone of a
// Theme with no CategoryStyles must not synthesize an empty map.
func TestCategoryStyles_Clone_Nil(t *testing.T) {
	base := &Theme{}
	clone := base.Clone()
	if clone.CategoryStyles != nil {
		t.Fatalf("Theme.Clone: CategoryStyles = %+v, want nil", clone.CategoryStyles)
	}
}

// TestCategoryStyles_Merge covers Merge's fine-grained cascade: unlike
// Gradients/Patterns (wholesale per-key replacement), each (field,
// value) leaf merges through MergeMarkStyle, so an override that only
// sets Stroke on an existing entry leaves that entry's Fill intact.
func TestCategoryStyles_Merge(t *testing.T) {
	base := &Theme{
		CategoryStyles: map[string]map[string]*MarkStyle{
			"Origin": {
				"USA":    {Fill: "#4c78a8"},
				"Europe": {Fill: "#f58518"},
			},
		},
	}
	override := &Theme{
		CategoryStyles: map[string]map[string]*MarkStyle{
			"Origin": {
				// Only sets Stroke on USA — Fill must survive from base.
				"USA":   {Stroke: "#000000"},
				"Japan": {Fill: "#e45756"},
			},
			"Status": {
				"at_risk": {Fill: "#e45756"},
			},
		},
	}

	merged := Merge(base, override)

	usa := merged.CategoryStyles["Origin"]["USA"]
	if usa.Fill != "#4c78a8" {
		t.Fatalf("Merge: Origin.USA.Fill = %q, want base value #4c78a8 preserved", usa.Fill)
	}
	if usa.Stroke != "#000000" {
		t.Fatalf("Merge: Origin.USA.Stroke = %q, want override value #000000", usa.Stroke)
	}
	if merged.CategoryStyles["Origin"]["Europe"].Fill != "#f58518" {
		t.Fatalf("Merge: Origin.Europe not preserved from base: %+v", merged.CategoryStyles["Origin"]["Europe"])
	}
	if merged.CategoryStyles["Origin"]["Japan"].Fill != "#e45756" {
		t.Fatalf("Merge: Origin.Japan (override-only value) not added: %+v", merged.CategoryStyles["Origin"])
	}
	if merged.CategoryStyles["Status"]["at_risk"].Fill != "#e45756" {
		t.Fatalf("Merge: Status (override-only field) not added: %+v", merged.CategoryStyles["Status"])
	}

	// Base must be untouched by the merge.
	if base.CategoryStyles["Origin"]["USA"].Stroke != "" {
		t.Fatalf("Merge: mutated base in place; Origin.USA.Stroke = %q, want empty", base.CategoryStyles["Origin"]["USA"].Stroke)
	}
}

// TestCategoryStyles_Merge_NilSides covers both-nil and one-nil-side
// Merge behavior, mirroring the MergeMarkStyle/MergeRange precedent.
func TestCategoryStyles_Merge_NilSides(t *testing.T) {
	if got := mergeCategoryStyles(nil, nil); got != nil {
		t.Fatalf("mergeCategoryStyles(nil, nil) = %+v, want nil", got)
	}

	baseOnly := map[string]map[string]*MarkStyle{"Origin": {"USA": {Fill: "#4c78a8"}}}
	got := mergeCategoryStyles(baseOnly, nil)
	if got["Origin"]["USA"].Fill != "#4c78a8" {
		t.Fatalf("mergeCategoryStyles(base, nil) = %+v, want base preserved", got)
	}
	// Result must be a copy, not an alias of baseOnly.
	got["Origin"]["USA"].Fill = "#mutated"
	if baseOnly["Origin"]["USA"].Fill != "#4c78a8" {
		t.Fatalf("mergeCategoryStyles(base, nil): result aliases base")
	}

	overrideOnly := map[string]map[string]*MarkStyle{"Origin": {"USA": {Fill: "#f58518"}}}
	got = mergeCategoryStyles(nil, overrideOnly)
	if got["Origin"]["USA"].Fill != "#f58518" {
		t.Fatalf("mergeCategoryStyles(nil, override) = %+v, want override applied", got)
	}
}

// TestApplyOverride_CategoryStyles confirms the spec-level
// theme.category_styles override block reaches theme.Theme through
// ApplyOverride, and that it round-trips through Merge (folded onto
// an existing base entry) via the same fine-grained per-leaf cascade
// as a directly-constructed theme.Theme.
func TestApplyOverride_CategoryStyles(t *testing.T) {
	width := 2.0
	base := &Theme{
		CategoryStyles: map[string]map[string]*MarkStyle{
			"Origin": {"USA": {Fill: "#4c78a8"}},
		},
	}
	override := &spec.ThemeOverride{
		CategoryStyles: map[string]map[string]*spec.MarkStyle{
			"Origin": {
				"USA":    {StrokeWidth: &width},
				"Europe": {Fill: "#f58518"},
			},
		},
	}
	got := ApplyOverride(base, override)

	usa := got.CategoryStyles["Origin"]["USA"]
	if usa.Fill != "#4c78a8" {
		t.Fatalf("ApplyOverride: Origin.USA.Fill = %q, want base value preserved", usa.Fill)
	}
	if usa.StrokeWidth == nil || *usa.StrokeWidth != 2.0 {
		t.Fatalf("ApplyOverride: Origin.USA.StrokeWidth = %v, want 2.0", usa.StrokeWidth)
	}
	if got.CategoryStyles["Origin"]["Europe"].Fill != "#f58518" {
		t.Fatalf("ApplyOverride: Origin.Europe not translated: %+v", got.CategoryStyles["Origin"])
	}

	// Base must be untouched.
	if base.CategoryStyles["Origin"]["USA"].StrokeWidth != nil {
		t.Fatalf("ApplyOverride: mutated base in place")
	}
}

// TestLoadBytes_CategoryStyles exercises the JSON load path
// end-to-end, mirroring TestLoadBytes_GradientsPatterns_Valid.
func TestLoadBytes_CategoryStyles(t *testing.T) {
	body := []byte(`{
		"name": "brand",
		"category_styles": {
			"Origin": {
				"USA": {"fill": "#4c78a8"},
				"Europe": {"fill": "#f58518", "stroke": "#000000"}
			}
		}
	}`)
	got, err := LoadBytes(body)
	if err != nil {
		t.Fatalf("LoadBytes: unexpected error: %v", err)
	}
	if got.CategoryStyles["Origin"]["USA"].Fill != "#4c78a8" {
		t.Fatalf("CategoryStyles[Origin][USA] not populated: %+v", got.CategoryStyles)
	}
	if got.CategoryStyles["Origin"]["Europe"].Stroke != "#000000" {
		t.Fatalf("CategoryStyles[Origin][Europe].Stroke not populated: %+v", got.CategoryStyles["Origin"]["Europe"])
	}
}
