package theme

import (
	"strings"
	"testing"
)

// TestCSSVariables_RawCSS covers E1-S2: RawCSS must be appended
// verbatim inside the <style> block, after the generated
// :root{--prism-*} variable manifest and the fixed class selectors,
// and must not appear when unset (back-compat with every existing
// built-in theme, none of which set RawCSS).
func TestCSSVariables_RawCSS(t *testing.T) {
	th := &Theme{
		AxisColor: "#333",
		RawCSS:    ".prism-mark-bar:hover{filter:brightness(1.1);}",
	}
	css := th.CSSVariables()
	if !strings.HasPrefix(css, "<style>") || !strings.HasSuffix(css, "</style>") {
		t.Fatalf("CSSVariables: missing <style> wrapper: %s", css)
	}
	body := strings.TrimSuffix(strings.TrimPrefix(css, "<style>"), "</style>")
	if !strings.HasSuffix(body, th.RawCSS) {
		t.Fatalf("CSSVariables: RawCSS not appended last inside <style>: %s", css)
	}
	// It must land after the :root{} manifest and the fixed class
	// selectors, not before.
	rootIdx := strings.Index(css, ":root{")
	rawIdx := strings.Index(css, th.RawCSS)
	if rootIdx == -1 || rawIdx == -1 || rawIdx < rootIdx {
		t.Fatalf("CSSVariables: RawCSS did not land after the variable manifest: %s", css)
	}
}

// TestCSSVariables_NoRawCSS proves the field is fully opt-in.
func TestCSSVariables_NoRawCSS(t *testing.T) {
	th := &Theme{AxisColor: "#333"}
	css := th.CSSVariables()
	if strings.Count(css, "</style>") != 1 {
		t.Fatalf("CSSVariables: unexpected shape with no RawCSS: %s", css)
	}
	if !strings.HasSuffix(css, ".prism-deselected{opacity:var(--prism-deselected-opacity,0.3);}</style>") {
		t.Fatalf("CSSVariables: expected fixed class selectors immediately before </style> when RawCSS is unset: %s", css)
	}
}
