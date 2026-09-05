package theme

import (
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
)

// TestLoadBytes_FilterReference_Valid covers the success path: a
// theme JSON document that declares a named filter body and
// references it from a style block loads cleanly, and the resolved
// theme carries both the filter registry and the reference through.
func TestLoadBytes_FilterReference_Valid(t *testing.T) {
	body := []byte(`{
		"name": "brand",
		"filters": {"soft_shadow": "<feDropShadow dx=\"0\" dy=\"2\" stdDeviation=\"2\"/>"},
		"raw_css": ".prism-mark-bar:hover { filter: brightness(1.1); }",
		"mark": {"fill": "#4c78a8", "filter": "soft_shadow"},
		"axis": {"domain_color": "#333", "filter": "soft_shadow"}
	}`)

	got, err := LoadBytes(body)
	if err != nil {
		t.Fatalf("LoadBytes: unexpected error: %v", err)
	}
	if got.Filters["soft_shadow"] == "" {
		t.Fatalf("Filters[soft_shadow] not populated")
	}
	if got.RawCSS == "" {
		t.Fatalf("RawCSS not populated")
	}
	if got.Mark == nil || got.Mark.Filter != "soft_shadow" {
		t.Fatalf("Mark.Filter = %+v, want soft_shadow", got.Mark)
	}
	if got.Axis == nil || got.Axis.Filter != "soft_shadow" {
		t.Fatalf("Axis.Filter = %+v, want soft_shadow", got.Axis)
	}
}

// TestLoadBytes_FilterReference_Unresolved covers the fail-loudly
// path: a style block's filter reference that does not name a key in
// the theme's filters map must reject at load with
// PRISM_THEME_FILTER_UNKNOWN rather than silently ignoring the
// reference (RangeSlot.Resolve's silent-fallback pattern does NOT
// apply here — see theme/validate.go).
func TestLoadBytes_FilterReference_Unresolved(t *testing.T) {
	body := []byte(`{
		"name": "brand",
		"mark": {"fill": "#4c78a8", "filter": "does_not_exist"}
	}`)

	_, err := LoadBytes(body)
	if err == nil {
		t.Fatalf("LoadBytes: expected error for unresolved filter reference, got nil")
	}
	requireCode(t, err, "PRISM_THEME_FILTER_UNKNOWN")
}

// TestLoadBytes_FilterReference_UnresolvedWithBase exercises the
// base-merge path: an override built on top of a registered base
// theme must still validate the merged result.
func TestLoadBytes_FilterReference_UnresolvedWithBase(t *testing.T) {
	body := []byte(`{
		"base": "light",
		"legend": {"filter": "missing_filter"}
	}`)

	_, err := LoadBytes(body)
	if err == nil {
		t.Fatalf("LoadBytes: expected error for unresolved filter reference on merged theme, got nil")
	}
	requireCode(t, err, "PRISM_THEME_FILTER_UNKNOWN")
}

// TestRegister_RejectsUnresolvedFilter mirrors the LoadBytes coverage
// for the other fail-loudly entry point named in the story:
// registration.
func TestRegister_RejectsUnresolvedFilter(t *testing.T) {
	bad := &Theme{
		Title: &TitleStyle{Filter: "ghost"},
	}
	err := Register("filter-test-invalid", bad)
	if err == nil {
		t.Fatalf("Register: expected error for unresolved filter reference, got nil")
	}
	requireCode(t, err, "PRISM_THEME_FILTER_UNKNOWN")
	if _, ok := Get("filter-test-invalid"); ok {
		t.Fatalf("Register: registry mutated despite validation failure")
	}
}

// TestRegister_AcceptsResolvedFilter is the Register-path success
// twin of TestLoadBytes_FilterReference_Valid.
func TestRegister_AcceptsResolvedFilter(t *testing.T) {
	good := &Theme{
		Filters: map[string]string{"glow": "<feGaussianBlur stdDeviation=\"1\"/>"},
		View:    &ViewStyle{Filter: "glow"},
	}
	if err := Register("filter-test-valid", good); err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}
	t.Cleanup(func() { delete(registry, "filter-test-valid") })

	got, ok := Get("filter-test-valid")
	if !ok {
		t.Fatalf("Get: theme not registered")
	}
	if got.View == nil || got.View.Filter != "glow" {
		t.Fatalf("View.Filter = %+v, want glow", got.View)
	}
}

// TestMarkStyle_CloneMerge_Filter covers the Clone/Merge round-trip
// for the new Filter field on MarkStyle.
func TestMarkStyle_CloneMerge_Filter(t *testing.T) {
	base := &MarkStyle{Fill: "#111", Filter: "base_filter"}
	clone := base.Clone()
	if clone.Filter != "base_filter" {
		t.Fatalf("Clone: Filter = %q, want base_filter", clone.Filter)
	}
	clone.Filter = "mutated"
	if base.Filter != "base_filter" {
		t.Fatalf("Clone: mutation leaked back into base (aliasing)")
	}

	override := &MarkStyle{Filter: "override_filter"}
	merged := MergeMarkStyle(base, override)
	if merged.Filter != "override_filter" {
		t.Fatalf("MergeMarkStyle: Filter = %q, want override_filter (override wins)", merged.Filter)
	}

	// Empty override.Filter inherits base's value (sparse-merge idiom).
	merged2 := MergeMarkStyle(base, &MarkStyle{Fill: "#222"})
	if merged2.Filter != "base_filter" {
		t.Fatalf("MergeMarkStyle: Filter = %q, want base_filter to be inherited", merged2.Filter)
	}
}

// TestTheme_CloneMerge_FiltersAndRawCSS covers the top-level
// Filters map + RawCSS field through Theme.Clone and theme.Merge.
func TestTheme_CloneMerge_FiltersAndRawCSS(t *testing.T) {
	base := &Theme{
		Filters: map[string]string{"a": "<feGaussianBlur/>"},
		RawCSS:  ".base {}",
	}
	clone := base.Clone()
	clone.Filters["a"] = "mutated"
	if base.Filters["a"] != "<feGaussianBlur/>" {
		t.Fatalf("Theme.Clone: Filters map aliasing leaked into base")
	}

	override := &Theme{
		Filters: map[string]string{"b": "<feDropShadow/>"},
		RawCSS:  ".override {}",
	}
	merged := Merge(base, override)
	if merged.Filters["a"] == "" || merged.Filters["b"] == "" {
		t.Fatalf("Merge: Filters map not merged key-by-key, got %+v", merged.Filters)
	}
	if merged.RawCSS != ".override {}" {
		t.Fatalf("Merge: RawCSS = %q, want override to win", merged.RawCSS)
	}

	// Empty override.RawCSS inherits base's value.
	merged2 := Merge(base, &Theme{})
	if merged2.RawCSS != ".base {}" {
		t.Fatalf("Merge: RawCSS = %q, want base to be inherited", merged2.RawCSS)
	}
}

// TestAxisLegendTitleViewStyle_MergeFilter covers the per-block merge
// helpers (mergeAxis/mergeLegend/mergeTitle/mergeView) for the new
// Filter field.
func TestAxisLegendTitleViewStyle_MergeFilter(t *testing.T) {
	base := &Theme{
		Axis:   &AxisStyle{Filter: "a1"},
		Legend: &LegendStyle{Filter: "l1"},
		Title:  &TitleStyle{Filter: "t1"},
		View:   &ViewStyle{Filter: "v1"},
	}
	override := &Theme{
		Axis:   &AxisStyle{Filter: "a2"},
		Legend: &LegendStyle{Filter: "l2"},
		Title:  &TitleStyle{Filter: "t2"},
		View:   &ViewStyle{Filter: "v2"},
	}
	merged := Merge(base, override)
	if merged.Axis.Filter != "a2" || merged.Legend.Filter != "l2" ||
		merged.Title.Filter != "t2" || merged.View.Filter != "v2" {
		t.Fatalf("Merge: per-block Filter override did not win: %+v %+v %+v %+v",
			merged.Axis, merged.Legend, merged.Title, merged.View)
	}

	// Empty override inherits base.
	merged2 := Merge(base, &Theme{
		Axis:   &AxisStyle{},
		Legend: &LegendStyle{},
		Title:  &TitleStyle{},
		View:   &ViewStyle{},
	})
	if merged2.Axis.Filter != "a1" || merged2.Legend.Filter != "l1" ||
		merged2.Title.Filter != "t1" || merged2.View.Filter != "v1" {
		t.Fatalf("Merge: per-block Filter did not inherit from base: %+v %+v %+v %+v",
			merged2.Axis, merged2.Legend, merged2.Title, merged2.View)
	}
}

// requireCode asserts err is a *prismerrors.AppError carrying code.
func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	appErr, ok := err.(*prismerrors.AppError)
	if !ok {
		t.Fatalf("error is %T, want *errors.AppError", err)
	}
	if appErr.Code != code {
		t.Fatalf("error code = %q, want %q", appErr.Code, code)
	}
}
