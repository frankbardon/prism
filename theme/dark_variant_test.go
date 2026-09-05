package theme

import (
	"testing"

	"github.com/frankbardon/prism/spec"
)

// TestRegister_AcceptsResolvedDarkVariant covers the success path: a
// theme paired to an already-registered counterpart registers
// cleanly and the pairing round-trips through Get.
func TestRegister_AcceptsResolvedDarkVariant(t *testing.T) {
	if err := Register("dv-test-counterpart", &Theme{}); err != nil {
		t.Fatalf("Register(counterpart): unexpected error: %v", err)
	}
	t.Cleanup(func() { delete(registry, "dv-test-counterpart") })

	good := &Theme{DarkVariant: "dv-test-counterpart"}
	if err := Register("dv-test-primary", good); err != nil {
		t.Fatalf("Register(primary): unexpected error: %v", err)
	}
	t.Cleanup(func() { delete(registry, "dv-test-primary") })

	got, ok := Get("dv-test-primary")
	if !ok {
		t.Fatalf("Get: theme not registered")
	}
	if got.DarkVariant != "dv-test-counterpart" {
		t.Fatalf("DarkVariant = %q, want dv-test-counterpart", got.DarkVariant)
	}
}

// TestRegister_RejectsUnknownDarkVariant covers the fail-loudly path:
// pairing to a theme name that is not (yet) registered rejects at
// Register with PRISM_THEME_DARK_VARIANT_UNKNOWN, and the registry is
// left unmutated.
func TestRegister_RejectsUnknownDarkVariant(t *testing.T) {
	bad := &Theme{DarkVariant: "does-not-exist"}
	err := Register("dv-test-invalid", bad)
	if err == nil {
		t.Fatalf("Register: expected error for unresolved dark_variant reference, got nil")
	}
	requireCode(t, err, "PRISM_THEME_DARK_VARIANT_UNKNOWN")
	if _, ok := Get("dv-test-invalid"); ok {
		t.Fatalf("Register: registry mutated despite validation failure")
	}
}

// TestLoadBytes_DarkVariantReference_Unresolved mirrors the Register
// coverage for the other fail-loudly entry point named in the story:
// theme JSON loaded via LoadBytes.
func TestLoadBytes_DarkVariantReference_Unresolved(t *testing.T) {
	body := []byte(`{
		"name": "brand",
		"dark_variant": "ghost_theme"
	}`)

	_, err := LoadBytes(body)
	if err == nil {
		t.Fatalf("LoadBytes: expected error for unresolved dark_variant reference, got nil")
	}
	requireCode(t, err, "PRISM_THEME_DARK_VARIANT_UNKNOWN")
}

// TestLoadBytes_DarkVariantReference_Valid covers the LoadBytes
// success path, including the base-merge flow.
func TestLoadBytes_DarkVariantReference_Valid(t *testing.T) {
	body := []byte(`{
		"base": "light",
		"dark_variant": "dark"
	}`)

	got, err := LoadBytes(body)
	if err != nil {
		t.Fatalf("LoadBytes: unexpected error: %v", err)
	}
	if got.DarkVariant != "dark" {
		t.Fatalf("DarkVariant = %q, want dark", got.DarkVariant)
	}
}

// TestTheme_CloneMerge_DarkVariant covers the Clone/Merge round-trip
// for the new top-level DarkVariant field.
func TestTheme_CloneMerge_DarkVariant(t *testing.T) {
	base := &Theme{DarkVariant: "dark"}
	clone := base.Clone()
	if clone.DarkVariant != "dark" {
		t.Fatalf("Clone: DarkVariant = %q, want dark", clone.DarkVariant)
	}
	clone.DarkVariant = "mutated"
	if base.DarkVariant != "dark" {
		t.Fatalf("Clone: mutation leaked back into base (aliasing)")
	}

	override := &Theme{DarkVariant: "high_contrast"}
	merged := Merge(base, override)
	if merged.DarkVariant != "high_contrast" {
		t.Fatalf("Merge: DarkVariant = %q, want high_contrast (override wins)", merged.DarkVariant)
	}

	// Empty override.DarkVariant inherits base's value.
	merged2 := Merge(base, &Theme{})
	if merged2.DarkVariant != "dark" {
		t.Fatalf("Merge: DarkVariant = %q, want dark to be inherited", merged2.DarkVariant)
	}
}

// TestApplyOverride_DarkVariant covers the spec-level ThemeOverride
// thread-through (mirrors how Filters/RawCSS were threaded through
// theme/override.go in E1-S1/E3-S1).
func TestApplyOverride_DarkVariant(t *testing.T) {
	base := &Theme{}
	merged := ApplyOverride(base, &spec.ThemeOverride{DarkVariant: "dark"})
	if merged.DarkVariant != "dark" {
		t.Fatalf("ApplyOverride: DarkVariant = %q, want dark", merged.DarkVariant)
	}

	// A nil/empty override leaves the base's DarkVariant untouched.
	baseWithVariant := &Theme{DarkVariant: "print"}
	unchanged := ApplyOverride(baseWithVariant, &spec.ThemeOverride{})
	if unchanged.DarkVariant != "print" {
		t.Fatalf("ApplyOverride: DarkVariant = %q, want print to be inherited", unchanged.DarkVariant)
	}
}
