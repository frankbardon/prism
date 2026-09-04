package html_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/custommark"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/render"
	prismhtml "github.com/frankbardon/prism/render/html"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// customFixtureSpec builds a minimal custom-mark spec referencing
// rendererName. Mirrors render/svg/custom_test.go's helper of the
// same name.
func customFixtureSpec(rendererName string) []byte {
	return []byte(fmt.Sprintf(`{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"x": 1}, {"x": 2}]},
  "mark": {"type": "custom", "renderer": %q},
  "encoding": {}
}`, rendererName))
}

// scriptHTMLRenderer implements only HTMLCustomRenderer and returns a
// fragment containing a <script> tag — the fixture for E2-S3
// acceptance criterion 1: the script tag must round-trip through the
// HTML backend intact and unmodified (no stripping, no escaping).
type scriptHTMLRenderer struct{}

func (scriptHTMLRenderer) RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return `<strong>hello custom</strong><script>window.__prismCustomMarkRan = true;</script>`, nil
}

// fixedSVGOnlyRenderer implements only SVGCustomRenderer — the
// fixture for E2-S3 acceptance criterion 2: the HTML backend must
// embed its output as an inline <svg> fragment (via the same
// sub-mark-embedding mechanism the table mark's sparkline columns
// use), since no HTMLCustomRenderer is available to call directly.
type fixedSVGOnlyRenderer struct{}

func (fixedSVGOnlyRenderer) RenderSVG(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return fmt.Sprintf(`<circle cx="%.3f" cy="%.3f" r="10" fill="tomato"/>`, box.W/2, box.H/2), nil
}

// renderCustomFixture runs the full spec -> build -> execute -> encode
// -> render pipeline against an inline spec body, mirroring render/
// svg/custom_test.go's helper of the same name — a custom mark's
// output depends on a Go-level registered renderer, so these fixtures
// don't belong in the public examples corpus.
func renderCustomFixture(t *testing.T, specJSON []byte) ([]byte, error) {
	t.Helper()
	s, err := spec.DecodeBytes(specJSON)
	if err != nil {
		return nil, err
	}
	dag, tipID, err := build.Build(s, build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		return nil, err
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
	if err != nil {
		return nil, err
	}
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{Width: 800, Height: 600})
	if err != nil {
		return nil, err
	}
	return prismhtml.New().Render(doc, render.RenderOpts{Format: "html", Width: 800, Height: 600})
}

// TestPrismHTMLCustomMarkScriptTagRoundTrips is E2-S3 acceptance
// criterion 1: a custom-mark spec whose RenderHTML includes a
// <script> tag must round-trip through the HTML backend with the
// script tag intact and unmodified — string-checked against the
// exact fragment content, not a stripped/escaped variant.
func TestPrismHTMLCustomMarkScriptTagRoundTrips(t *testing.T) {
	const name = "test-html-script"
	custommark.ResetForTest(t)
	if err := custommark.Register(name, scriptHTMLRenderer{}); err != nil {
		t.Fatalf("custommark.Register: %v", err)
	}

	got, err := renderCustomFixture(t, customFixtureSpec(name))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(got)

	const wantScript = `<script>window.__prismCustomMarkRan = true;</script>`
	if !strings.Contains(out, wantScript) {
		t.Fatalf("expected exact, unmodified <script> tag %q in output, got:\n%s", wantScript, out)
	}
	if strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("script tag appears escaped in output:\n%s", out)
	}
	if !strings.Contains(out, "<strong>hello custom</strong>") {
		t.Errorf("expected the rest of the RenderHTML fragment verbatim too, got:\n%s", out)
	}
	if !strings.Contains(out, `data-prism-renderer="test-html-script"`) {
		t.Errorf("expected data-prism-renderer attribute on wrapping div, got:\n%s", out)
	}
}

// TestPrismHTMLCustomMarkSVGOnlyEmbedsInlineSVG is E2-S3 acceptance
// criterion 2: a renderer implementing only SVGCustomRenderer must
// have its output embedded as a well-formed inline <svg> fragment
// under the HTML backend.
func TestPrismHTMLCustomMarkSVGOnlyEmbedsInlineSVG(t *testing.T) {
	const name = "test-svg-only"
	custommark.ResetForTest(t)
	if err := custommark.Register(name, fixedSVGOnlyRenderer{}); err != nil {
		t.Fatalf("custommark.Register: %v", err)
	}

	got, err := renderCustomFixture(t, customFixtureSpec(name))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(got)

	frag := extractSVGFragment(t, out)
	if err := assertWellFormedXML(frag); err != nil {
		t.Fatalf("inline <svg> fragment is not well-formed XML: %v\n%s", err, frag)
	}
	if !strings.Contains(frag, `fill="tomato"`) {
		t.Errorf("expected the SVGCustomRenderer's spliced <circle> inside the inline <svg>, got:\n%s", frag)
	}
	if strings.Contains(out, "foreignObject") {
		t.Errorf("HTML backend should not need a <foreignObject> wrapper (that's the SVG backend's fallback), got:\n%s", out)
	}
}

// TestPrismHTMLCustomMarkMissingRendererErrors asserts an unregistered
// renderer name produces the same PRISM_RENDER_CUSTOM_MARK_NOT_FOUND
// *errors.AppError the SVG backend raises (E2-S2), not a panic or a
// bare Go error.
func TestPrismHTMLCustomMarkMissingRendererErrors(t *testing.T) {
	_, err := renderCustomFixture(t, customFixtureSpec("does-not-exist"))
	if err == nil {
		t.Fatalf("expected an error for an unregistered custom mark renderer, got nil")
	}
	if !strings.Contains(err.Error(), "PRISM_RENDER_CUSTOM_MARK_NOT_FOUND") {
		t.Errorf("expected PRISM_RENDER_CUSTOM_MARK_NOT_FOUND, got: %v", err)
	}
}

// TestPrismHTMLCustomMarkGoldensStable golden-tests both custom-mark
// paths under the HTML backend. Set UPDATE_GOLDENS=1 to regenerate,
// mirroring goldens_test.go's convention.
func TestPrismHTMLCustomMarkGoldensStable(t *testing.T) {
	update := os.Getenv("UPDATE_GOLDENS") == "1"
	cases := []struct {
		name     string
		renderer string
		impl     custommark.CustomRenderer
	}{
		{name: "custom_html_script", renderer: "golden-html-script", impl: scriptHTMLRenderer{}},
		{name: "custom_svg_only", renderer: "golden-svg-only", impl: fixedSVGOnlyRenderer{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			custommark.ResetForTest(t)
			if err := custommark.Register(tc.renderer, tc.impl); err != nil {
				t.Fatalf("custommark.Register: %v", err)
			}

			got, err := renderCustomFixture(t, customFixtureSpec(tc.renderer))
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			goldenPath := filepath.Join(repoRoot(t), "render", "html", "testdata", "htmls", tc.name+".html")
			if update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden %s: %v", goldenPath, err)
				}
				t.Logf("wrote golden %s (%d bytes)", goldenPath, len(got))
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (%s): %v.\nRun `UPDATE_GOLDENS=1 go test ./render/html/...` to create.", goldenPath, err)
			}
			if !bytes.Equal(want, got) {
				t.Errorf("HTML does not match golden %s.\n--- golden ---\n%s\n--- got ---\n%s",
					goldenPath, truncate(want, 1200), truncate(got, 1200))
			}
		})
	}
}

// extractSVGFragment returns the single top-level <svg>...</svg>
// fragment found in s, failing the test if there isn't exactly one.
func extractSVGFragment(t *testing.T, s string) string {
	t.Helper()
	frags := extractSVGFragments(s)
	if len(frags) != 1 {
		t.Fatalf("got %d inline <svg> fragments, want 1:\n%s", len(frags), truncate([]byte(s), 1200))
	}
	return frags[0]
}
