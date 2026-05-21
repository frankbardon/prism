package devtools

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// crossImplPreflight enforces the same skip conditions as
// TestCrossImplSVGParity. Returns the absolute path to `node` on
// success.
func crossImplPreflight(t *testing.T) (root, nodePath string) {
	t.Helper()
	if os.Getenv("PRISM_CROSS_IMPL") != "1" {
		t.Skip("set PRISM_CROSS_IMPL=1 + run `npm install` inside internal/devtools/cross-impl-runner/ to enable")
	}
	var err error
	nodePath, err = exec.LookPath("node")
	if err != nil {
		t.Skip("node binary not on PATH")
	}
	root = repoRoot(t)
	runnerDir := filepath.Join(root, "internal", "devtools", "cross-impl-runner")
	if _, err := os.Stat(filepath.Join(runnerDir, "node_modules", "happy-dom")); err != nil {
		t.Skipf("happy-dom not installed under %s/node_modules/ — run `npm install` per README", runnerDir)
	}
	return root, nodePath
}

// runHarness invokes the named .mjs entry under cross-impl-runner/
// and returns combined output + error. Used by both lifecycle and
// dedupe tests.
func runHarness(t *testing.T, root, nodePath, entry string) (string, error) {
	t.Helper()
	cmd := exec.Command(nodePath, filepath.Join("internal", "devtools", "cross-impl-runner", entry))
	cmd.Dir = root
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// TestPrismWebComponentLifecycle exercises the <prism-chart> custom
// element connect → attribute change → disconnect → reconnect cycle
// via happy-dom. The .mjs harness exits 0 on pass, non-zero on
// fail. Mandatory gate per PHASE.md.
func TestPrismWebComponentLifecycle(t *testing.T) {
	root, nodePath := crossImplPreflight(t)
	out, err := runHarness(t, root, nodePath, "web-component-lifecycle.mjs")
	if err != nil {
		t.Fatalf("web-component-lifecycle: %v\noutput:\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("output:\n%s", out)
	}
}

// TestPrismDatasetRegistryDedupe asserts PrismResolver.fetch
// memoises by URL — multiple consumers requesting the same src
// share one round-trip per D074. Mandatory gate per PHASE.md.
func TestPrismDatasetRegistryDedupe(t *testing.T) {
	root, nodePath := crossImplPreflight(t)
	out, err := runHarness(t, root, nodePath, "dataset-registry-dedupe.mjs")
	if err != nil {
		t.Fatalf("dataset-registry-dedupe: %v\noutput:\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("output:\n%s", out)
	}
}

// TestPrismAnimatorTween exercises the client-side tween engine
// under happy-dom: partition enter/update/exit by data-prism-mark-key,
// drive a deterministic rAF clock through 0..1, verify numeric attrs
// arrive at their target values, OKLab color interpolation lands in
// the expected perceptual band, and structurallyCompatible accepts
// matching scene shapes. No WASM required.
func TestPrismAnimatorTween(t *testing.T) {
	root, nodePath := crossImplPreflight(t)
	out, err := runHarness(t, root, nodePath, "animator-tween.mjs")
	if err != nil {
		t.Fatalf("animator-tween: %v\noutput:\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("output:\n%s", out)
	}
}

// TestPrismAnimatorWarnFallback asserts SceneHandle.update emits a
// `prism:warn` CustomEvent with PRISM_WARN_ANIM_FALLBACK when the
// new scene declares an animation block but the previous scene is
// structurally incompatible — and stays silent on the legitimate
// no-warn branches (matching shape, no block, reduced-motion,
// first render).
func TestPrismAnimatorWarnFallback(t *testing.T) {
	root, nodePath := crossImplPreflight(t)
	out, err := runHarness(t, root, nodePath, "animator-warn-fallback.mjs")
	if err != nil {
		t.Fatalf("animator-warn-fallback: %v\noutput:\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("output:\n%s", out)
	}
}

// TestPrismConditionsBrush asserts setSelection wires through to the
// new applyConditions helper: a selection-driven `fill` condition on
// two marks flips to WhenValue while the brush is active and reverts
// to Otherwise when the state clears. See tier1-01 PR4.
func TestPrismConditionsBrush(t *testing.T) {
	root, nodePath := crossImplPreflight(t)
	out, err := runHarness(t, root, nodePath, "conditions-brush.mjs")
	if err != nil {
		t.Fatalf("conditions-brush: %v\noutput:\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("output:\n%s", out)
	}
}
