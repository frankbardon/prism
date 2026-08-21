package theme

import (
	"strings"
	"testing"
)

// TestCSSKeepsExactlyOneRootRule pins the contract the consumer
// depends on: Arc rewrites the `:root{` selector to scope tokens per
// chart, and per-organisation theming swaps the values inside that one
// rule. A second `:root{` anywhere in the block would leave half the
// tokens unscoped and half the org overrides unreachable.
func TestCSSKeepsExactlyOneRootRule(t *testing.T) {
	for _, name := range []string{"light", "dark", "print", "high_contrast", "colorblind"} {
		th := MustGet(name)
		css := th.CSSVariables()
		if n := strings.Count(css, ":root{"); n != 1 {
			t.Errorf("%s: %d `:root{` rules, want exactly 1", name, n)
		}
		if !strings.HasPrefix(css, "<style>:root{") {
			t.Errorf("%s: token block must lead the style element, got %.40s", name, css)
		}
	}
}

// TestCSSCarriesADarkCompanion asserts a light-family base ships both
// token sets, so a host that flips theme repaints through CSS alone
// rather than asking for a re-render.
func TestCSSCarriesADarkCompanion(t *testing.T) {
	css := MustGet("light").CSSVariables()
	if !strings.Contains(css, ".prism-dark") {
		t.Fatal("light base must carry a dark companion block")
	}
	// The companion must override the values a dark ground needs.
	for _, token := range []string{"--prism-color-text:", "--prism-color-grid:", "--prism-color-bg:"} {
		idx := strings.Index(css, ".prism-dark")
		if !strings.Contains(css[idx:], token) {
			t.Errorf("dark companion missing %s", token)
		}
	}
}

// TestCSSDarkCompanionIsOptInOnly is the guard for the failure that is
// easy to ship and hard to notice: an SVG inlined into a LIGHT page,
// viewed on a machine whose OS is set to dark, must not repaint itself
// dark. The OS setting says nothing about this chart's background.
func TestCSSDarkCompanionIsOptInOnly(t *testing.T) {
	css := MustGet("light").CSSVariables()
	media := strings.Index(css, "@media")
	if media < 0 {
		t.Fatal("expected a prefers-color-scheme block")
	}
	end := strings.Index(css[media:], "}}")
	if end < 0 {
		t.Fatal("malformed media block")
	}
	block := css[media : media+end]
	if !strings.Contains(block, ".prism-auto") {
		t.Error("the media query must only reach hosts that opted in with prism-auto")
	}
	if strings.Contains(block, ":root:not(") || strings.Contains(block, "{:root{") {
		t.Errorf("media query reaches un-classed charts: %s", block)
	}
}

// TestCSSDarkCompanionOnlyEmitsDifferences asserts the companion block
// carries the tokens that actually change, not a full second copy.
func TestCSSDarkCompanionOnlyEmitsDifferences(t *testing.T) {
	light := MustGet("light")
	css := light.CSSVariables()
	rootEnd := strings.Index(css, "}")
	root := css[len("<style>:root{"):rootEnd]
	idx := strings.Index(css, ".prism-dark")
	companion := css[idx : strings.Index(css[idx:], "}")+idx]

	// Font family does not change between grounds, so it must not be
	// repeated on every chart.
	if strings.Contains(companion, "--prism-font-sans:") {
		t.Error("dark companion repeats an unchanged token (--prism-font-sans)")
	}
	if !strings.Contains(root, "--prism-font-sans:") {
		t.Error("root block lost --prism-font-sans")
	}
}

// TestDarkFamilyBasesHaveNoCompanion asserts an explicitly-chosen dark
// theme is not re-flipped by a host, and print — which has no dark
// mode — carries no companion at all.
func TestDarkFamilyBasesHaveNoCompanion(t *testing.T) {
	for _, name := range []string{"dark", "print"} {
		if got := MustGet(name).CompanionDark(); got != "" {
			t.Errorf("%s: CompanionDark() = %q, want none", name, got)
		}
		if strings.Contains(MustGet(name).CSSVariables(), ".prism-dark") {
			t.Errorf("%s must not emit a companion block", name)
		}
	}
}

// TestEveryBaseDeclaresTheNewTokens guards the Update Demand: a token
// added to one base and forgotten in the others is a control that
// silently does nothing on four themes out of five.
func TestEveryBaseDeclaresTheNewTokens(t *testing.T) {
	required := []string{
		"--prism-color-text-muted:",
		"--prism-axis-zero-color:",
		"--prism-axis-zero-width:",
		"--prism-axis-band-padding:",
		"--prism-axis-band-max-width:",
		"--prism-legend-gap:",
		"--prism-legend-row-height:",
		"--prism-legend-symbol-extent:",
		"--prism-legend-symbol-corner-radius:",
	}
	for _, name := range Names() {
		css := MustGet(name).CSSVariables()
		for _, tok := range required {
			if !strings.Contains(css, tok) {
				t.Errorf("theme %s does not declare %s", name, tok)
			}
		}
	}
}

// TestCSSDarkCompanionCarriesNoGeometry is the guard for an override
// that silently loses on a dark host.
//
// The companion block wins over :root by specificity, so any GEOMETRY
// token in it overrides whatever the organisation set. An org that
// raised --prism-axis-band-padding to 0.5 got 0.5 on a light host and
// Prism's own 0.28 on a dark one, because the companion carried the
// dark base's default for a token the org had already customised.
// Geometry does not depend on the ground and must never appear here.
func TestCSSDarkCompanionCarriesNoGeometry(t *testing.T) {
	css := MustGet("light").CSSVariables()
	idx := strings.Index(css, ".prism-dark")
	if idx < 0 {
		t.Fatal("expected a companion block")
	}
	block := css[idx : idx+strings.Index(css[idx:], "}")]
	_, block, _ = strings.Cut(block, "{")
	for _, decl := range strings.Split(block, ";") {
		_, v, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		if !isColorValue(v) {
			t.Errorf("dark companion carries a non-colour declaration: %q", decl)
		}
	}
	// And it must still carry the colours that matter.
	if !strings.Contains(block, "--prism-color-text:") {
		t.Error("companion lost --prism-color-text")
	}
}

// TestCSSOrgGeometryOverrideSurvivesTheDarkCompanion is the same
// invariant stated end-to-end: a theme that customises a geometry
// token must render with that value under BOTH grounds.
func TestCSSOrgGeometryOverrideSurvivesTheDarkCompanion(t *testing.T) {
	base := MustGet("light")
	base.Axis.BandPadding = ptr(0.5)
	base.Legend.SymbolExtent = ptr(14)
	css := base.CSSVariables()

	idx := strings.Index(css, ".prism-dark")
	if idx < 0 {
		t.Fatal("expected a companion block")
	}
	block := css[idx : idx+strings.Index(css[idx:], "}")]
	for _, tok := range []string{"--prism-axis-band-padding", "--prism-legend-symbol-extent"} {
		if strings.Contains(block, tok) {
			t.Errorf("companion re-declares %s and would overwrite the organisation's value", tok)
		}
	}
	if !strings.Contains(css[:idx], "--prism-axis-band-padding:0.5;") {
		t.Error("root block lost the customised band padding")
	}
}
