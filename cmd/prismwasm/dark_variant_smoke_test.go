package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// darkVariantWasmSmokeSpec mirrors the fixture used by
// cmd/prism/dark_variant_smoke_test.go and the Twirp coverage in
// cmd/prism/twirp_roundtrip_test.go: a small categorical-color bar
// chart whose spec-level `theme.dark_variant` names the built-in
// "dark" theme. No `theme` name is set and no optsJSON theme is
// passed to prism.execute below, so the base theme resolves to the
// "light" default (encode.resolveThemeFull) with the dark_variant
// sparse-override layered on top — the same "automatic, no new flag"
// path exercised on every other surface.
const darkVariantWasmSmokeSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "title": "E4-S4 wasm auto-dark smoke",
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

// TestPrismWasmAutoDarkPipelineSmoke is E4-S4's web-component/WASM
// coverage. It drives the exact globalThis.prism.validate → plan →
// execute → render chain the browser's <prism-chart> web component
// uses (probe-runner.mjs's "pipeline" mode — the same harness E4-S1's
// renderHTML regression closer and the TinyGo↔host parity suite use),
// over a spec whose theme carries `dark_variant`, and asserts the
// resulting SVG string carries the doubled auto-dark <style> block:
// the @media (prefers-color-scheme: dark) chrome swap (E4-S2) and at
// least one --prism-resolved-N mark-color variable pair (E4-S3).
//
// This proves prism.execute's theme resolution (which is just
// encode.Encode under the hood — see doExecute/executePipeline in
// main.go) is not bypassed or special-cased by the wasm bridge: no
// optsJSON theme argument is passed to prism.execute here at all, so
// the wasm entry has to fall through to the spec-embedded override on
// its own, exactly like the CLI and Twirp surfaces.
func TestPrismWasmAutoDarkPipelineSmoke(t *testing.T) {
	tinygo, err := exec.LookPath("tinygo")
	if err != nil {
		t.Skip("tinygo not on PATH; skipping wasm auto-dark smoke test (install via `brew tap tinygo-org/tools && brew install tinygo`)")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH; skipping wasm auto-dark smoke test")
	}

	root := repoRoot(t)
	specPath := filepath.Join(t.TempDir(), "dark_variant_wasm_smoke.json")
	if err := os.WriteFile(specPath, []byte(darkVariantWasmSmokeSpec), 0o644); err != nil {
		t.Fatalf("write spec fixture: %v", err)
	}

	wasmPath, execPath := buildTinyGoWasm(t, root, tinygo)
	runner := filepath.Join(root, "internal", "devtools", "cross-impl-runner", "probe-runner.mjs")

	out := runNode(t, root, runner, "pipeline", wasmPath, execPath, specPath)
	svg := strings.TrimSpace(string(out))

	if svg == "" {
		t.Fatal("pipeline (render) produced empty output")
	}
	if strings.HasPrefix(svg, `{"ok":false`) {
		t.Fatalf("pipeline returned an error envelope: %s", svg)
	}
	if !strings.Contains(svg, "<svg") {
		t.Errorf("pipeline output is not an SVG document; got prefix %q", head(svg, 80))
	}
	if !strings.Contains(svg, "@media (prefers-color-scheme: dark)") {
		t.Errorf("wasm pipeline SVG missing dark media-query chrome block:\n%s", head(svg, 2000))
	}
	if !strings.Contains(svg, "--prism-resolved-") {
		t.Errorf("wasm pipeline SVG missing --prism-resolved-N mark-color variable declarations:\n%s", head(svg, 2000))
	}
}
