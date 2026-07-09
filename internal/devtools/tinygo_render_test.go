package devtools

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// tinyGoRenderCase is one end-to-end pipeline fixture: a spec fed
// through the TinyGo wasm's exported validate → plan → execute → render
// surface under Node, plus the SVG geometry the render must contain.
type tinyGoRenderCase struct {
	// name is the fixture stem under testdata/tinygo_render/<name>.json.
	name string
	// sidecar, when non-empty, names a sibling JSON file carrying a
	// {"datasets": {...}, "resolver": {...}} pair the runner forwards as
	// the datasets registry and installs via prism.setDataResolver —
	// exercising the JS-supplied DataResolver seam (a `data: {ref}` or a
	// `datasets`-registered `data: {name}` reference).
	sidecar string
	// wantTags are SVG element openings the rendered output must all
	// contain (proof the marks actually made it into the geometry, not
	// just an empty chrome frame).
	wantTags []string
}

// tinyGoRenderCases spans the two transform families this story must
// prove render end-to-end through the TinyGo binary:
//   - structured filter + calculate (E2, pure-Go predicate/derived-column)
//   - the previously-Pulse crosstab and regression transforms (E3, now
//     pure-Go)
//
// plus a resolver-backed spec that enters its rows through the
// JS-supplied DataResolver rather than inline data.values.
var tinyGoRenderCases = []tinyGoRenderCase{
	{name: "filter_calculate", wantTags: []string{"<rect"}},
	{name: "crosstab", sidecar: "crosstab.sidecar.json", wantTags: []string{"<rect"}},
	{name: "regression", sidecar: "regression.sidecar.json", wantTags: []string{"<circle", "<polyline"}},
	{name: "resolver", sidecar: "resolver.sidecar.json", wantTags: []string{"<rect"}},
}

// TestTinyGoWasmRenderPipeline proves the TinyGo-built wasm renders a
// chart end-to-end — not merely compiles. It drives the full exported
// pipeline (validate → plan → execute over inline data.values / a
// JS-supplied DataResolver → render → SVG bytes) under Node with
// TinyGo's paired wasm_exec.js, and asserts each fixture produces a
// non-empty, well-formed <svg> carrying the expected mark geometry.
//
// This is the E5 success-criterion closer: "TinyGo wasm renders a chart
// end-to-end." Byte-level float parity is covered separately by
// TestTinyGoWasmSVGParity; here the assertions are structural so the
// smoke stays robust to legitimate goldens churn.
//
// Opt-in (mirrors PRISM_CROSS_IMPL): needs node + tinygo on PATH and
// PRISM_CROSS_IMPL_TINYGO=1. Skips cleanly when the harness can't run.
//
// Run: PRISM_CROSS_IMPL_TINYGO=1 go test ./internal/devtools/ -run TinyGoWasmRenderPipeline
func TestTinyGoWasmRenderPipeline(t *testing.T) {
	root := requireTinyGoParityEnv(t)

	wasmPath, execPath := buildTinyGoWasm(t, root, "./cmd/prismwasm")
	runner := filepath.Join(root, "internal", "devtools", "cross-impl-runner", "probe-runner.mjs")
	fixtureDir := filepath.Join(root, "internal", "devtools", "testdata", "tinygo_render")

	for _, tc := range tinyGoRenderCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			specPath := filepath.Join(fixtureDir, tc.name+".json")
			args := []string{"pipeline", wasmPath, execPath, specPath}
			if tc.sidecar != "" {
				args = append(args, filepath.Join(fixtureDir, tc.sidecar))
			}

			svg := string(runNodePipeline(t, root, runner, args...))

			if strings.TrimSpace(svg) == "" {
				t.Fatalf("%s: pipeline produced empty output", tc.name)
			}
			if !strings.HasPrefix(strings.TrimSpace(svg), "<svg") {
				t.Fatalf("%s: output is not an <svg> document; got prefix %q", tc.name, head(svg, 60))
			}
			if !strings.Contains(svg, "</svg>") {
				t.Fatalf("%s: output has no closing </svg> tag", tc.name)
			}
			if !strings.Contains(svg, "viewBox=") {
				t.Errorf("%s: rendered svg is missing a viewBox", tc.name)
			}
			for _, tag := range tc.wantTags {
				if !strings.Contains(svg, tag) {
					t.Errorf("%s: rendered svg missing expected mark geometry %q\n%s",
						tc.name, tag, head(svg, 400))
				}
			}
		})
	}
}

// TestTinyGoWasmErrorEnvelope is the E6-S2 regression closer: it proves
// an ERROR path through the TinyGo-built wasm now returns a structured
// PRISM_* envelope instead of trapping.
//
// Before E6-S2, rendering any fixup reached text/template's Execute,
// which calls reflect.Value.MethodByName — unimplemented in TinyGo — so
// every error path crashed the module with "RuntimeError: unreachable"
// and no envelope ever reached JS. The fixture spec (a bar mark carrying
// a theta channel) fails validation with PRISM_SPEC_003, whose fixups
// interpolate {{.Mark}} / {{.Allowed}} / {{.Channel}} — exactly the
// path that used to trap. We assert a well-formed {"ok":false,...}
// envelope comes back with the code and rendered fixups, and that no
// RuntimeError leaked to stderr.
//
// Opt-in like the other TinyGo tests: needs node + tinygo + PRISM_CROSS_IMPL_TINYGO=1.
//
// Run: PRISM_CROSS_IMPL_TINYGO=1 go test ./internal/devtools/ -run TinyGoWasmErrorEnvelope
func TestTinyGoWasmErrorEnvelope(t *testing.T) {
	root := requireTinyGoParityEnv(t)

	wasmPath, execPath := buildTinyGoWasm(t, root, "./cmd/prismwasm")
	runner := filepath.Join(root, "internal", "devtools", "cross-impl-runner", "probe-runner.mjs")
	specPath := filepath.Join(root, "internal", "devtools", "testdata", "tinygo_render", "validate_error.json")

	out := string(runNodePipeline(t, root, runner, "error", wasmPath, execPath, specPath))

	if strings.Contains(out, "RuntimeError") || strings.Contains(out, "unreachable") {
		t.Fatalf("error path trapped the TinyGo wasm instead of returning an envelope:\n%s", out)
	}
	for _, want := range []string{`"ok":false`, "PRISM_SPEC_003", "fixups"} {
		if !strings.Contains(out, want) {
			t.Errorf("error envelope missing %q\nenvelope:\n%s", want, out)
		}
	}
	// The rendered fixup must carry the interpolated placeholders (proof
	// the non-reflective interpolator ran, not a raw template body).
	if !strings.Contains(out, "channel supported by bar") {
		t.Errorf("fixup placeholders were not interpolated\nenvelope:\n%s", out)
	}
}

// runNodePipeline invokes probe-runner.mjs with an arbitrary argument
// tail (the pipeline mode takes an optional resolver-data path beyond
// the fixed <mode> <wasm> <exec> <spec> quad) and returns its stdout.
func runNodePipeline(t *testing.T, root, runner string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("node", append([]string{runner}, args...)...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("node probe-runner pipeline: %v\nstderr:\n%s", err, stderr.String())
	}
	return stdout.Bytes()
}

// head returns the first n bytes of s for diagnostics.
func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
