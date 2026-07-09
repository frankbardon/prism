package devtools

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/frankbardon/prism/internal/devtools/floatcorpus"
)

// tinyGoParityFixtures is the float-diverse spec set the TinyGo↔Go
// render-parity check renders through both toolchains. Each name must
// resolve to examples/specs/<name>.json and produce committed
// testdata/cross_impl/<name>/{scene.json,go.svg} (auto-bootstrapped on
// first run from the host `prism` binary). The set intentionally spans
// mark families whose geometry stresses render.FormatFloat differently:
// straight edges (bar/rule/tick), curves (line/area/sparkline/path),
// trigonometric arcs (pie/donut/arc/funnel), bezier ribbons (sankey),
// and dense rect/box/violin layouts.
var tinyGoParityFixtures = []string{
	"bar_basic",
	"line_basic",
	"area_basic",
	"point_scatter",
	"layer_actual_vs_benchmark",
	"pie",
	"donut",
	"arc_basic",
	"sankey_user_flow",
	"histogram",
	"heatmap",
	"boxplot",
	"violin_score",
	"funnel_signup",
	"sparkline_inline",
	"path_arbitrary",
}

// TestTinyGoWasmSVGParity proves the TinyGo-built wasm renderer emits
// byte-identical SVG to the standard-Go renderer for the fixture
// corpus. This guards the E5-S2 top risk: TinyGo ships its own strconv,
// and every SVG coordinate funnels through render.FormatFloat
// (render/precision.go). If TinyGo rounded or stringified floats
// differently, the coordinate goldens would drift. The check builds a
// TinyGo js/wasm module from ./cmd/prismwasm, renders each fixture's
// scene.json under Node with TinyGo's matching wasm_exec.js, and diffs
// the SVG against the committed standard-Go go.svg.
//
// Opt-in (mirrors PRISM_CROSS_IMPL) because it needs node + tinygo:
//   - PRISM_CROSS_IMPL_TINYGO must be "1"
//   - node + tinygo must be on PATH
//
// Run: PRISM_CROSS_IMPL_TINYGO=1 go test ./internal/devtools/ -run TinyGo
func TestTinyGoWasmSVGParity(t *testing.T) {
	root := requireTinyGoParityEnv(t)
	prismBin := hostPrismBinary(t, root)

	wasmPath, execPath := buildTinyGoWasm(t, root, "./cmd/prismwasm")
	runner := filepath.Join(root, "internal", "devtools", "cross-impl-runner", "probe-runner.mjs")

	for _, fixture := range tinyGoParityFixtures {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			dir := filepath.Join(root, "testdata", "cross_impl", fixture)
			scenePath := filepath.Join(dir, "scene.json")
			goSVGPath := filepath.Join(dir, "go.svg")
			specPath := filepath.Join(root, "examples", "specs", fixture+".json")

			// Self-bootstrap the Go-side inputs if a fixture is new.
			if _, err := os.Stat(scenePath); err != nil {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", dir, err)
				}
				if err := regenerateGoInputs(root, prismBin, specPath, scenePath, goSVGPath); err != nil {
					t.Fatalf("auto-regen: %v", err)
				}
			}

			goSVG, err := os.ReadFile(goSVGPath)
			if err != nil {
				t.Fatalf("read go.svg: %v", err)
			}

			tinySVG := runNodeProbe(t, root, runner, "render", wasmPath, execPath, scenePath)

			if !bytes.Equal(goSVG, tinySVG) {
				diffPath := filepath.Join(dir, "tinygo.diff.txt")
				_ = os.WriteFile(diffPath, []byte(
					"go bytes: "+itoa(len(goSVG))+"\ntinygo bytes: "+itoa(len(tinySVG))+
						"\n\n--- go ---\n"+string(goSVG)+"\n\n--- tinygo ---\n"+string(tinySVG)+"\n",
				), 0o644)
				t.Errorf("TinyGo↔Go SVG drift for %s (go=%d bytes, tinygo=%d bytes)\n%s",
					fixture, len(goSVG), len(tinySVG), diffContext(goSVG, tinySVG, 200))
			}
		})
	}
}

// TestTinyGoFloatFormatParity confirms render.FormatFloat
// (render/precision.go's 3-decimal quantization) produces identical
// output on the host, in a standard-Go wasm module, and in a TinyGo
// wasm module for a corpus that stresses half-way rounding, trailing-
// zero trimming, negative zero, magnitude extremes, and the non-finite
// guards. This is the targeted companion to the whole-SVG render diff:
// it pins the exact funnel every coordinate flows through, over edge
// values a fixture render may never hit.
//
// Same opt-in gate as TestTinyGoWasmSVGParity.
func TestTinyGoFloatFormatParity(t *testing.T) {
	root := requireTinyGoParityEnv(t)
	runner := filepath.Join(root, "internal", "devtools", "cross-impl-runner", "probe-runner.mjs")

	// Host reference: the same code path, compiled natively.
	want := floatcorpus.Format()

	goWasm, goExec := buildGoWasm(t, root, "./internal/devtools/floatprobe")
	tinyWasm, tinyExec := buildTinyGoWasm(t, root, "./internal/devtools/floatprobe")

	goOut := string(runNodeProbe(t, root, runner, "global", goWasm, goExec, "prismFloatProbe"))
	if goOut != want {
		t.Errorf("standard-Go wasm float format diverged from host\nhost:\n%s\nwasm:\n%s", want, goOut)
	}

	tinyOut := string(runNodeProbe(t, root, runner, "global", tinyWasm, tinyExec, "prismFloatProbe"))
	if tinyOut != want {
		t.Errorf("TinyGo wasm float format diverged from host\nhost:\n%s\ntinygo:\n%s", want, tinyOut)
	}

	if goOut != tinyOut {
		t.Errorf("standard-Go wasm and TinyGo wasm float format disagree\ngo:\n%s\ntinygo:\n%s", goOut, tinyOut)
	}
}

// --- helpers ---

// requireTinyGoParityEnv enforces the opt-in gate + toolchain presence
// and returns the repo root. Skips (never fails) when the harness can't
// run, matching the PRISM_CROSS_IMPL contract.
func requireTinyGoParityEnv(t *testing.T) string {
	t.Helper()
	if os.Getenv("PRISM_CROSS_IMPL_TINYGO") != "1" {
		t.Skip("set PRISM_CROSS_IMPL_TINYGO=1 (needs node + tinygo) to enable")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node binary not on PATH")
	}
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("tinygo binary not on PATH")
	}
	return repoRoot(t)
}

// hostPrismBinary ensures bin/prism exists, building it if absent, and
// returns its path. The host binary is the source of truth the TinyGo
// wasm SVG is diffed against.
func hostPrismBinary(t *testing.T, root string) string {
	t.Helper()
	prismBin := filepath.Join(root, "bin", "prism")
	if _, err := os.Stat(prismBin); err == nil {
		return prismBin
	}
	cmd := exec.Command("go", "build", "-o", "bin/prism", "./cmd/prism")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build bin/prism: %v\n%s", err, out)
	}
	return prismBin
}

// buildTinyGoWasm compiles pkg to a TinyGo js/wasm module in a fresh
// temp dir and copies TinyGo's paired wasm_exec.js alongside it. The
// -stack-size flag mirrors the Makefile's build-wasm-tinygo target (the
// JSON-Schema validator's recursion overflows the default stack).
func buildTinyGoWasm(t *testing.T, root, pkg string) (wasmPath, execPath string) {
	t.Helper()
	dir := t.TempDir()
	wasmPath = filepath.Join(dir, "probe.wasm")

	build := exec.Command("tinygo", "build", "-target=wasm", "-stack-size=8MB", "-o", wasmPath, pkg)
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("tinygo build %s: %v\n%s", pkg, err, out)
	}

	tinyRoot := strings.TrimSpace(runOut(t, root, "tinygo", "env", "TINYGOROOT"))
	src := filepath.Join(tinyRoot, "targets", "wasm_exec.js")
	execPath = filepath.Join(dir, "wasm_exec.js")
	copyFile(t, src, execPath)
	return wasmPath, execPath
}

// buildGoWasm compiles pkg to a standard-Go js/wasm module in a fresh
// temp dir and copies the Go toolchain's paired wasm_exec.js.
func buildGoWasm(t *testing.T, root, pkg string) (wasmPath, execPath string) {
	t.Helper()
	dir := t.TempDir()
	wasmPath = filepath.Join(dir, "probe.wasm")

	build := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", wasmPath, pkg)
	build.Dir = root
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build (js/wasm) %s: %v\n%s", pkg, err, out)
	}

	goRoot := strings.TrimSpace(runOut(t, root, "go", "env", "GOROOT"))
	src := filepath.Join(goRoot, "lib", "wasm", "wasm_exec.js")
	if _, err := os.Stat(src); err != nil {
		src = filepath.Join(goRoot, "misc", "wasm", "wasm_exec.js")
	}
	execPath = filepath.Join(dir, "wasm_exec.js")
	copyFile(t, src, execPath)
	return wasmPath, execPath
}

// runNodeProbe invokes probe-runner.mjs and returns its stdout bytes.
func runNodeProbe(t *testing.T, root, runner, mode, wasmPath, execPath, arg string) []byte {
	t.Helper()
	cmd := exec.Command("node", runner, mode, wasmPath, execPath, arg)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("node probe-runner %s: %v\nstderr:\n%s", mode, err, stderr.String())
	}
	return stdout.Bytes()
}

func runOut(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return string(out)
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

// itoa avoids pulling strconv into the test's top-level imports just for
// a couple of diagnostic sizes.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
