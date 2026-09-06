package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// darkVariantSmokeSpec is a small categorical-color bar chart whose
// spec-level `theme.dark_variant` names the built-in "dark" theme.
// Per E4-S1/E4-S2/E4-S3, this is enough to trigger auto-dark end to
// end — no `--theme` flag, no custom theme registration, and no new
// CLI/serve option: the base theme defaults to "light" (unset
// `theme.name` + unset `--theme` flag) and the sparse override wires
// `dark_variant` onto it via theme.ApplyOverride at encode time. E4-S4
// is a pure verification story: this fixture proves the CLI surfaces
// (`plot`, `scene`) don't special-case away that automatic behavior.
const darkVariantSmokeSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "title": "E4-S4 CLI auto-dark smoke",
  "data": {
    "values": [
      {"category": "alpha", "value": 12},
      {"category": "beta",  "value": 27},
      {"category": "gamma", "value": 19}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "category", "type": "nominal"},
    "y": {"field": "value", "type": "quantitative"},
    "color": {"field": "category", "type": "nominal"}
  },
  "theme": {"dark_variant": "dark"}
}`

// writeDarkVariantSmokeSpec writes darkVariantSmokeSpec to a temp file
// and returns its path, for the CLI's positional spec-file argument.
func writeDarkVariantSmokeSpec(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dark_variant_smoke.json")
	if err := os.WriteFile(path, []byte(darkVariantSmokeSpec), 0o644); err != nil {
		t.Fatalf("write spec fixture: %v", err)
	}
	return path
}

// TestPrismPlotAutoDarkSVG proves `prism plot` (SVG backend) emits the
// doubled auto-dark <style> block — the base :root chrome (E4-S2), the
// @media (prefers-color-scheme: dark) chrome swap (E4-S2), and at
// least one --prism-resolved-N mark-color variable pair (E4-S3) — for
// a spec whose theme carries `dark_variant`, with no extra flag.
func TestPrismPlotAutoDarkSVG(t *testing.T) {
	fixture := writeDarkVariantSmokeSpec(t)
	out, exit := runCLI(t, "prism", "plot", fixture)
	if exit != 0 {
		t.Fatalf("plot exited %d: %s", exit, firstChars(out, 400))
	}
	body := stripLeadingWarnings(out)
	if !strings.HasPrefix(body, "<svg ") {
		t.Fatalf("output does not start with <svg: %s", firstChars(body, 200))
	}
	if !strings.Contains(body, "@media (prefers-color-scheme: dark)") {
		t.Errorf("plot SVG missing dark media-query chrome block: %s", firstChars(body, 2000))
	}
	if !strings.Contains(body, "--prism-resolved-") {
		t.Errorf("plot SVG missing --prism-resolved-N mark-color variable declarations: %s", firstChars(body, 2000))
	}
	if !strings.Contains(body, `fill="var(--prism-resolved-`) {
		t.Errorf("plot SVG bar marks do not reference a resolved-var fill: %s", firstChars(body, 2000))
	}
}

// TestPrismPlotAutoDarkHTML proves the HTML backend (which wraps
// render/svg's own emitters — see render/html) carries the same
// doubled CSS through `prism plot --format html`.
func TestPrismPlotAutoDarkHTML(t *testing.T) {
	fixture := writeDarkVariantSmokeSpec(t)
	out, exit := runCLI(t, "prism", "plot", "--format", "html", fixture)
	if exit != 0 {
		t.Fatalf("plot --format html exited %d: %s", exit, firstChars(out, 400))
	}
	body := stripLeadingWarnings(out)
	if !strings.HasPrefix(body, "<!doctype html>") {
		t.Fatalf("output does not start with <!doctype html>: %s", firstChars(body, 200))
	}
	if !strings.Contains(body, "@media (prefers-color-scheme: dark)") {
		t.Errorf("plot --format html missing dark media-query chrome block: %s", firstChars(body, 2000))
	}
	if !strings.Contains(body, "--prism-resolved-") {
		t.Errorf("plot --format html missing --prism-resolved-N mark-color variable declarations: %s", firstChars(body, 2000))
	}
}

// TestPrismSceneAutoDark proves `prism scene` — the Scene IR JSON
// entrypoint consumed by the JS renderer and the TinyGo/wasm parity
// harness — carries the same auto-dark CSS inside its embedded
// theme.css string, since encode.Encode (not the renderer) is what
// builds that string.
func TestPrismSceneAutoDark(t *testing.T) {
	fixture := writeDarkVariantSmokeSpec(t)
	out, exit := runCLI(t, "prism", "scene", fixture)
	if exit != 0 {
		t.Fatalf("scene exited %d: %s", exit, firstChars(out, 400))
	}
	body := stripLeadingWarnings(out)
	if !strings.Contains(body, "@media (prefers-color-scheme: dark)") {
		t.Errorf("scene JSON missing dark media-query chrome block: %s", firstChars(body, 2000))
	}
	if !strings.Contains(body, "--prism-resolved-") {
		t.Errorf("scene JSON missing --prism-resolved-N mark-color variable declarations: %s", firstChars(body, 2000))
	}
}
