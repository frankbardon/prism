package devtools

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// curatedFixtures is the P12 launch set for cross-impl parity. Each
// fixture must exist as testdata/specs/<name>.json. The harness
// derives testdata/cross_impl/<name>/scene.json (from `prism scene`)
// + testdata/cross_impl/<name>/go.svg (from `prism plot`) on first
// run with PRISM_CROSS_IMPL_REGEN=1, then re-runs to byte-diff the
// JS output against go.svg.
//
// Adding fixtures: append the spec name here, regenerate, commit.
var curatedFixtures = []string{
	"bar_basic",
	"line_basic",
	"layer_actual_vs_benchmark",
	"pie",
	"sankey_user_flow",
}

// TestCrossImplSVGParity diffs Go-rendered SVG against JS-rendered
// SVG byte-for-byte for each curated fixture (D076). Skips when:
//   - PRISM_CROSS_IMPL != "1" (CI without node deps stays green)
//   - `node` is not on PATH
//   - happy-dom is not installed in the runner's node_modules/
//
// Set PRISM_CROSS_IMPL_REGEN=1 to refresh scene.json + go.svg under
// testdata/cross_impl/<fixture>/ before diffing. Use this after a
// Scene IR change.
func TestCrossImplSVGParity(t *testing.T) {
	if os.Getenv("PRISM_CROSS_IMPL") != "1" {
		t.Skip("set PRISM_CROSS_IMPL=1 + run `npm install` inside internal/devtools/cross-impl-runner/ to enable")
	}
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node binary not on PATH")
	}
	root := repoRoot(t)

	// Ensure bin/prism exists; build if missing.
	prismBin := filepath.Join(root, "bin", "prism")
	if _, err := os.Stat(prismBin); err != nil {
		buildCmd := exec.Command("go", "build", "-o", "bin/prism", "./cmd/prism")
		buildCmd.Dir = root
		if buildOut, buildErr := buildCmd.CombinedOutput(); buildErr != nil {
			t.Fatalf("go build: %v\n%s", buildErr, buildOut)
		}
	}

	// Check the runner is wired up (package.json + node_modules).
	runnerDir := filepath.Join(root, "internal", "devtools", "cross-impl-runner")
	if _, err := os.Stat(filepath.Join(runnerDir, "node_modules", "happy-dom")); err != nil {
		t.Skipf("happy-dom not installed under %s/node_modules/ — run `npm install` per README", runnerDir)
	}

	regen := os.Getenv("PRISM_CROSS_IMPL_REGEN") == "1"

	for _, fixture := range curatedFixtures {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			dir := filepath.Join(root, "testdata", "cross_impl", fixture)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", dir, err)
			}
			scenePath := filepath.Join(dir, "scene.json")
			goSVGPath := filepath.Join(dir, "go.svg")
			jsSVGPath := filepath.Join(dir, "js.svg")
			specPath := filepath.Join(root, "testdata", "specs", fixture+".json")

			if regen {
				if err := regenerateGoInputs(root, prismBin, specPath, scenePath, goSVGPath); err != nil {
					t.Fatalf("regen: %v", err)
				}
			}

			// Inputs must exist before running the JS harness.
			if _, err := os.Stat(scenePath); err != nil {
				if regen {
					t.Fatalf("regen failed to produce %s: %v", scenePath, err)
				}
				// Auto-regen on first run to make the test self-bootstrapping.
				if err := regenerateGoInputs(root, prismBin, specPath, scenePath, goSVGPath); err != nil {
					t.Fatalf("auto-regen: %v", err)
				}
			}
			if _, err := os.Stat(goSVGPath); err != nil {
				if err := regenerateGoInputs(root, prismBin, specPath, scenePath, goSVGPath); err != nil {
					t.Fatalf("auto-regen go.svg: %v", err)
				}
			}

			// Run the Node harness — emits js.svg.
			cmd := exec.Command(nodePath, "internal/devtools/cross-impl-runner/main.mjs", fixture)
			cmd.Dir = root
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("node harness for %s: %v\nstderr:\n%s", fixture, err, stderr.String())
			}

			goSVG, err := os.ReadFile(goSVGPath)
			if err != nil {
				t.Fatalf("read go.svg: %v", err)
			}
			jsSVG, err := os.ReadFile(jsSVGPath)
			if err != nil {
				t.Fatalf("read js.svg: %v", err)
			}
			// Normalise both sides before diffing — see D076 +
			// normaliseSVG comment.
			goN := normaliseSVG(goSVG)
			jsN := normaliseSVG(jsSVG)
			if !bytes.Equal(goN, jsN) {
				t.Errorf("cross-impl SVG drift for %s (go=%d bytes raw / %d normalised, js=%d bytes raw / %d normalised)\nfirst-diff context (normalised):\n%s",
					fixture, len(goSVG), len(goN), len(jsSVG), len(jsN), diffContext(goN, jsN, 200))
				// Always write a side-by-side debug artifact when
				// drift is detected — speeds up triage.
				diffPath := filepath.Join(dir, "diff.txt")
				_ = os.WriteFile(diffPath, []byte(fmt.Sprintf(
					"go bytes (raw): %d\njs bytes (raw): %d\ngo bytes (normalised): %d\njs bytes (normalised): %d\n\n--- go (normalised) ---\n%s\n\n--- js (normalised) ---\n%s\n",
					len(goSVG), len(jsSVG), len(goN), len(jsN), goN, jsN,
				)), 0o644)
			}
		})
	}
}

// regenerateGoInputs shells out to `prism scene` + `prism plot` to
// refresh the Go-side cross-impl inputs. Called either on
// PRISM_CROSS_IMPL_REGEN=1 or auto-bootstrap when the inputs are
// missing for a curated fixture.
func regenerateGoInputs(root, prismBin, specPath, scenePath, goSVGPath string) error {
	if _, err := os.Stat(specPath); err != nil {
		return fmt.Errorf("missing spec %s: %w", specPath, err)
	}
	// scene.json
	sceneCmd := exec.Command(prismBin, "scene", "--out", scenePath, specPath)
	sceneCmd.Dir = root
	if out, err := sceneCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("prism scene: %w\n%s", err, out)
	}
	// go.svg
	plotCmd := exec.Command(prismBin, "plot", "--out", goSVGPath, specPath)
	plotCmd.Dir = root
	if out, err := plotCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("prism plot: %w\n%s", err, out)
	}
	return nil
}

// diffContext returns the first 2N bytes around the first byte
// where Go and JS SVG differ. Used to make the failure message
// useful without dumping the entire SVG.
func diffContext(a, b []byte, n int) string {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			start := i - n
			if start < 0 {
				start = 0
			}
			endA := i + n
			if endA > len(a) {
				endA = len(a)
			}
			endB := i + n
			if endB > len(b) {
				endB = len(b)
			}
			return fmt.Sprintf("first diff at byte %d:\n  go: %q\n  js: %q", i, a[start:endA], b[start:endB])
		}
	}
	if len(a) != len(b) {
		return fmt.Sprintf("equal prefix; lengths differ: go=%d js=%d", len(a), len(b))
	}
	return "(no diff?)"
}

// errNotInstalled is the sentinel for the skip path when the runner
// is missing its node deps. Kept as a named var so callers can
// errors.Is check it instead of string-matching.
var errNotInstalled = errors.New("happy-dom not installed")

// normaliseSVG canonicalises an SVG byte stream so two implementations
// that differ only in serialiser cosmetics still compare equal. Per
// D076, parity is on semantic content + structure + attribute values
// — not on whitespace placement or self-closing-tag style.
//
// Normalisations applied:
//  1. Strip inter-tag whitespace (`>\s+<` → `><`). Go's writer
//     emits "\n  " between siblings for human-readable goldens;
//     happy-dom's outerHTML strips that.
//  2. Strip leading whitespace at the start of file.
//  3. Strip trailing whitespace at the end of file.
//  4. Collapse `<foo ... ></foo>` to `<foo ... />` for void-style
//     elements (rect, line, circle, image, path, polyline, ellipse,
//     polygon, use, stop). Go uses the SelfClose path; happy-dom's
//     outerHTML always uses the explicit close form.
//  5. Style + text + title element bodies are preserved verbatim
//     (whitespace inside <style>/<text>/<title> is content).
//
// A future refinement could parse both into an XML DOM and compare
// trees; for the launch fixture set, regex-based normalisation is
// adequate and faster to debug.
func normaliseSVG(src []byte) []byte {
	s := string(src)
	// 1+2+3: strip inter-tag whitespace + leading/trailing space.
	// `>\s+<` is unambiguous: the only place SVG can have `>`
	// followed by whitespace followed by `<` is between sibling
	// tags. Quoted attribute values can't contain raw `<`/`>` per
	// the XML spec, and SVG text content (inside <text>/<title>/
	// <style>) is plain text without `<` characters. Style sheets
	// likewise contain no `<`.
	out := stripInterTagWhitespace(s)
	out = strings.TrimSpace(out)

	// 4: collapse `<tag ...></tag>` to `<tag .../>` for known void
	// SVG elements. Go writer's SelfClose path emits the former;
	// happy-dom outerHTML emits the latter.
	for _, tag := range []string{"rect", "line", "circle", "image", "polyline", "path", "polygon", "ellipse", "use", "stop"} {
		// Match `<tag ATTRS></tag>` where ATTRS contains no '>' or '<'
		// — i.e. a single-line, child-less element. Quoted-attr
		// values can't contain raw '<'/'>' (XML escaping rule), so
		// this is safe.
		re := regexp.MustCompile(`<` + tag + `(\s[^<>]*?)></` + tag + `>`)
		out = re.ReplaceAllString(out, `<`+tag+`$1/>`)
		// Also handle the no-attribute form.
		re2 := regexp.MustCompile(`<` + tag + `></` + tag + `>`)
		out = re2.ReplaceAllString(out, `<`+tag+`/>`)
	}

	return []byte(out)
}

// stripInterTagWhitespace collapses runs of `>` + whitespace + `<`
// to `><`. Preserves whitespace inside attribute values (no `<` or
// `>` can appear inside a quoted attribute per the XML spec) and
// inside text/title/style bodies (those bodies contain no `<`).
var interTagWS = regexp.MustCompile(`>\s+<`)

func stripInterTagWhitespace(s string) string {
	return interTagWS.ReplaceAllString(s, "><")
}
