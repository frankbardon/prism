package devtools

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestTinyGoWasmCustomMarkJSFallback is the E2-S5 acceptance closer:
// it proves a `custom` mark resolves purely through the WASM entry's
// JS-side registration path (prism.registerCustomMark), with NO
// matching Go-side custommark.Register call anywhere in the process.
//
// It drives probe-runner.mjs's "custommark" mode, which:
//  1. loads the TinyGo-built wasm module,
//  2. calls globalThis.prism.registerCustomMark("js-fallback-mark", fn)
//     with a JS-only fn (never touching custommark.Register),
//  3. runs validate → execute → render over
//     testdata/tinygo_render/custommark_js_fallback.json, whose
//     mark.renderer references that exact name.
//
// The Go-side custommark registry resolving the SAME spec shape (a
// custom mark referencing a Go-registered renderer) is already
// covered end-to-end by render/svg/custom_test.go and
// render/html/custom_test.go on every `go test` run — this test is
// the WASM/JS half of that parity story, proving the two runtimes
// resolve the same spec shape through their respective registries.
//
// Opt-in like the other TinyGo tests: needs node + tinygo on PATH and
// PRISM_CROSS_IMPL_TINYGO=1.
//
// Run: PRISM_CROSS_IMPL_TINYGO=1 go test ./internal/devtools/ -run TinyGoWasmCustomMarkJSFallback
func TestTinyGoWasmCustomMarkJSFallback(t *testing.T) {
	root := requireTinyGoParityEnv(t)

	wasmPath, execPath := buildTinyGoWasm(t, root, "./cmd/prismwasm")
	runner := filepath.Join(root, "internal", "devtools", "cross-impl-runner", "probe-runner.mjs")
	specPath := filepath.Join(root, "internal", "devtools", "testdata", "tinygo_render", "custommark_js_fallback.json")

	const rendererName = "js-fallback-mark"

	svg := string(runNodePipeline(t, root, runner, "custommark", wasmPath, execPath, specPath, rendererName))

	if strings.TrimSpace(svg) == "" {
		t.Fatalf("custommark probe produced empty output")
	}
	if !strings.HasPrefix(strings.TrimSpace(svg), "<svg") {
		t.Fatalf("output is not an <svg> document; got prefix %q", head(svg, 60))
	}
	if !strings.Contains(svg, "</svg>") {
		t.Fatalf("output has no closing </svg> tag")
	}

	// Proof the JS callback actually ran (not some Go-side fallback
	// coincidentally matching the name) and that the fragment landed
	// verbatim through the HTMLCustomRenderer foreignObject path
	// render/svg/custom.go already uses for a Go-registered
	// HTMLCustomRenderer.
	for _, want := range []string{
		`data-prism-renderer="js-fallback-mark"`,
		"<foreignObject",
		`class="js-custom-mark"`,
		"js rendered",
		`data-rows="2"`, // the fixture spec has 2 data.values rows
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("rendered svg missing expected fragment %q\n%s", want, head(svg, 800))
		}
	}
}
