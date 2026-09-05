package theme

import (
	"strings"
	"testing"
)

// TestCSSVariables_DarkVariant_Unset proves the no-op path: a theme
// with DarkVariant unset emits byte-identical CSS to a theme with the
// field simply absent — no @media block, ever. This is the load-
// bearing regression guard for the "existing goldens unaffected"
// acceptance criterion (E4-S2): every built-in theme leaves
// DarkVariant unset, so their CSSVariables() output — and therefore
// every SVG/HTML golden — must not move by a single byte.
func TestCSSVariables_DarkVariant_Unset(t *testing.T) {
	th := &Theme{AxisColor: "#333", TextColor: "#111"}
	css := th.CSSVariables()
	if strings.Contains(css, "prefers-color-scheme") {
		t.Fatalf("CSSVariables: unexpected @media block with DarkVariant unset: %s", css)
	}
	if strings.Count(css, ":root{") != 1 {
		t.Fatalf("CSSVariables: expected exactly one :root{} block, got: %s", css)
	}
}

// TestCSSVariables_DarkVariant_Resolved covers the success path: a
// registered DarkVariant pairing emits a second :root rule guarded by
// @media (prefers-color-scheme: dark) carrying the dark theme's
// chrome values (axis/grid/legend/title/view/selection-state), placed
// after the base :root block and the fixed class selectors, inside
// the same <style> element.
func TestCSSVariables_DarkVariant_Resolved(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	darkCounterpart := &Theme{
		AxisColor: "#dark-axis",
		Axis:      &AxisStyle{DomainColor: "#dark-domain", TickWidth: f(2)},
		Legend:    &LegendStyle{LabelColor: "#dark-legend-label"},
		Title:     &TitleStyle{Color: "#dark-title"},
		View:      &ViewStyle{Background: "#dark-view-bg"},
		States: map[string]*StateStyle{
			"selected": {Opacity: f(1)},
		},
		// Mark defaults must NOT appear in the dark block — mark
		// colors are out of scope for E4-S2 (E4-S3's job).
		Mark: &MarkStyle{Fill: "#dark-mark-fill-should-not-appear"},
	}
	if err := Register("dv-css-test-dark", darkCounterpart); err != nil {
		t.Fatalf("Register(dark counterpart): unexpected error: %v", err)
	}
	t.Cleanup(func() { delete(registry, "dv-css-test-dark") })

	primary := &Theme{
		AxisColor:   "#light-axis",
		DarkVariant: "dv-css-test-dark",
		Axis:        &AxisStyle{DomainColor: "#light-domain"},
	}
	if err := Register("dv-css-test-primary", primary); err != nil {
		t.Fatalf("Register(primary): unexpected error: %v", err)
	}
	t.Cleanup(func() { delete(registry, "dv-css-test-primary") })

	resolved, ok := Get("dv-css-test-primary")
	if !ok {
		t.Fatalf("Get: theme not registered")
	}
	css := resolved.CSSVariables()

	// Structural shape: <style> ... :root{base} ... classes ...
	// @media(dark){ :root{dark} } ... </style>.
	if !strings.HasPrefix(css, "<style>") || !strings.HasSuffix(css, "</style>") {
		t.Fatalf("CSSVariables: missing <style> wrapper: %s", css)
	}
	mediaIdx := strings.Index(css, "@media (prefers-color-scheme: dark){:root{")
	if mediaIdx == -1 {
		t.Fatalf("CSSVariables: missing dark media query block: %s", css)
	}
	baseRootIdx := strings.Index(css, ":root{")
	if baseRootIdx == -1 || mediaIdx < baseRootIdx {
		t.Fatalf("CSSVariables: dark media block did not land after the base :root block: %s", css)
	}
	classIdx := strings.Index(css, ".prism-axis-domain")
	if classIdx == -1 || mediaIdx < classIdx {
		t.Fatalf("CSSVariables: dark media block did not land after the fixed class selectors: %s", css)
	}

	// Base :root carries the primary (light) theme's values.
	baseBlock := css[baseRootIdx:mediaIdx]
	if !strings.Contains(baseBlock, "--prism-color-axis:#light-axis;") {
		t.Errorf("base :root missing light axis color: %s", baseBlock)
	}
	if !strings.Contains(baseBlock, "--prism-axis-domain-color:#light-domain;") {
		t.Errorf("base :root missing light domain color: %s", baseBlock)
	}

	// Dark media block carries the counterpart's chrome values.
	darkBlock := css[mediaIdx:]
	for _, want := range []string{
		"--prism-color-axis:#dark-axis;",
		"--prism-axis-domain-color:#dark-domain;",
		"--prism-axis-tick-width:2px;",
		"--prism-legend-label-color:#dark-legend-label;",
		"--prism-title-color:#dark-title;",
		"--prism-view-bg:#dark-view-bg;",
		"--prism-selected-opacity:1;",
	} {
		if !strings.Contains(darkBlock, want) {
			t.Errorf("dark media block missing %q: %s", want, darkBlock)
		}
	}
	if strings.Contains(darkBlock, "dark-mark-fill-should-not-appear") {
		t.Errorf("dark media block must not carry mark defaults (out of scope for E4-S2): %s", darkBlock)
	}

	// Fixed class selectors are not duplicated inside the media
	// query — the class set is static and already references var(),
	// so only the custom-property values need to swap.
	if strings.Count(css, ".prism-axis-domain{") != 1 {
		t.Fatalf("CSSVariables: fixed class selectors must not be duplicated inside @media: %s", css)
	}
}

// TestCSSVariables_DarkVariant_UnregisteredDegradesGracefully covers a
// defensive path: a *Theme built by hand (bypassing Register/Validate)
// with a DarkVariant that names nothing in the registry must not
// panic or emit a broken/empty dark block — CSSVariables treats it
// the same as DarkVariant being unset.
func TestCSSVariables_DarkVariant_UnregisteredDegradesGracefully(t *testing.T) {
	th := &Theme{AxisColor: "#333", DarkVariant: "does-not-exist-anywhere"}
	css := th.CSSVariables()
	if strings.Contains(css, "prefers-color-scheme") {
		t.Fatalf("CSSVariables: unexpected @media block for unresolved DarkVariant: %s", css)
	}
}

// TestCSSVariables_DarkVariant_RawCSSStillLast proves RawCSS keeps
// its E1-S2 invariant (appended verbatim, last, inside <style>) even
// when a dark media block is also present.
func TestCSSVariables_DarkVariant_RawCSSStillLast(t *testing.T) {
	if err := Register("dv-css-test-dark2", &Theme{AxisColor: "#dark"}); err != nil {
		t.Fatalf("Register(dark counterpart): unexpected error: %v", err)
	}
	t.Cleanup(func() { delete(registry, "dv-css-test-dark2") })

	th := &Theme{
		AxisColor:   "#light",
		DarkVariant: "dv-css-test-dark2",
		RawCSS:      ".prism-mark-bar:hover{filter:brightness(1.1);}",
	}
	css := th.CSSVariables()
	body := strings.TrimSuffix(strings.TrimPrefix(css, "<style>"), "</style>")
	if !strings.HasSuffix(body, th.RawCSS) {
		t.Fatalf("CSSVariables: RawCSS not appended last with dark variant present: %s", css)
	}
}
