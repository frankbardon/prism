package svg_test

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
	"github.com/frankbardon/prism/render/svg"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// customFixtureSpec builds a minimal custom-mark spec referencing
// rendererName. Two rows so a test renderer can assert on Rows if it
// wants to; the fixtures here draw fixed geometry instead so the
// golden output stays independent of row content.
func customFixtureSpec(rendererName string) []byte {
	return []byte(fmt.Sprintf(`{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"x": 1}, {"x": 2}]},
  "mark": {"type": "custom", "renderer": %q},
  "encoding": {}
}`, rendererName))
}

// renderCustomFixture runs the full spec -> build -> execute -> encode
// -> render pipeline (mirroring renderFixture in goldens_test.go, but
// against an inline spec body rather than the examples/specs corpus —
// a custom mark's output depends on a Go-level registered renderer,
// so these fixtures don't belong in the public examples corpus).
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
	return svg.New().Render(doc, render.RenderOpts{Format: "svg", Width: 800, Height: 600})
}

// fixedSVGRenderer implements only SVGCustomRenderer, drawing a fixed
// circle sized off the box it's handed. Used to verify the SVG
// backend's preferred direct-splice path (E2-S2 AC1): the fragment it
// returns must appear byte-identical (unwrapped) in the rendered SVG.
type fixedSVGRenderer struct{}

func (fixedSVGRenderer) RenderSVG(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return fmt.Sprintf(`<circle cx="%.3f" cy="%.3f" r="10" fill="tomato"/>`, box.W/2, box.H/2), nil
}

// fixedHTMLRenderer implements only HTMLCustomRenderer — the mirror
// case, verifying the <foreignObject> fallback (E2-S2 AC2).
type fixedHTMLRenderer struct{}

func (fixedHTMLRenderer) RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return "<strong>hello custom</strong>", nil
}

// TestPrismSVGCustomMarkGoldens covers E2-S2's two golden-fixture
// acceptance criteria: a pure-RenderSVG custom mark splices unwrapped,
// and an HTML-only custom mark falls back to a <foreignObject>-
// wrapped emission. Set UPDATE_GOLDENS=1 to regenerate, mirroring
// goldens_test.go's convention.
func TestPrismSVGCustomMarkGoldens(t *testing.T) {
	update := os.Getenv("UPDATE_GOLDENS") == "1"
	cases := []struct {
		name     string
		renderer string
		impl     custommark.CustomRenderer
	}{
		{name: "custom_svg_splice", renderer: "test-svg-splice", impl: fixedSVGRenderer{}},
		{name: "custom_html_foreign_object", renderer: "test-html-fallback", impl: fixedHTMLRenderer{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if err := custommark.Register(tc.renderer, tc.impl); err != nil {
				t.Fatalf("custommark.Register: %v", err)
			}
			t.Cleanup(func() { custommark.Unregister(tc.renderer) })

			got, err := renderCustomFixture(t, customFixtureSpec(tc.renderer))
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			goldenPath := filepath.Join(repoRoot(t), "testdata", "svgs", tc.name+".svg")
			if update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden %s: %v", goldenPath, err)
				}
				t.Logf("wrote golden %s (%d bytes)", goldenPath, len(got))
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (%s): %v.\nRun `UPDATE_GOLDENS=1 go test ./render/svg/...` to create.", goldenPath, err)
			}
			if !bytes.Equal(want, got) {
				t.Errorf("SVG does not match golden %s.\n--- golden ---\n%s\n--- got ---\n%s",
					goldenPath, truncate(want, 800), truncate(got, 800))
			}
		})
	}
}

// TestPrismSVGCustomMarkSplicesUnwrapped asserts the SVGCustomRenderer
// path's fragment appears verbatim in the output with no wrapping
// element beyond Prism's own positioning <g> (no <foreignObject>, no
// escaping of the fragment's own markup).
func TestPrismSVGCustomMarkSplicesUnwrapped(t *testing.T) {
	const name = "test-svg-splice-unwrapped"
	if err := custommark.Register(name, fixedSVGRenderer{}); err != nil {
		t.Fatalf("custommark.Register: %v", err)
	}
	t.Cleanup(func() { custommark.Unregister(name) })

	got, err := renderCustomFixture(t, customFixtureSpec(name))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, `fill="tomato"`) {
		t.Fatalf("expected spliced <circle> fragment in output, got:\n%s", out)
	}
	if strings.Contains(out, "foreignObject") {
		t.Errorf("SVGCustomRenderer path unexpectedly wrapped in <foreignObject>:\n%s", out)
	}
	if !strings.Contains(out, `class="prism-custom-mark"`) {
		t.Errorf("expected prism-custom-mark positioning group, got:\n%s", out)
	}
}

// TestPrismSVGCustomMarkForeignObjectFallback asserts an HTML-only
// custom mark's fragment lands inside a <foreignObject> with an inner
// XHTML-namespaced element.
func TestPrismSVGCustomMarkForeignObjectFallback(t *testing.T) {
	const name = "test-html-fallback-wrapped"
	if err := custommark.Register(name, fixedHTMLRenderer{}); err != nil {
		t.Fatalf("custommark.Register: %v", err)
	}
	t.Cleanup(func() { custommark.Unregister(name) })

	got, err := renderCustomFixture(t, customFixtureSpec(name))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, "<foreignObject") {
		t.Fatalf("expected <foreignObject> wrapper in output, got:\n%s", out)
	}
	if !strings.Contains(out, `xmlns="http://www.w3.org/1999/xhtml"`) {
		t.Errorf("expected xhtml-namespaced inner element inside foreignObject, got:\n%s", out)
	}
	if !strings.Contains(out, "hello custom") {
		t.Errorf("expected HTML fragment content in output, got:\n%s", out)
	}
}

// TestPrismSVGCustomMarkMissingRendererErrors asserts an unregistered
// renderer name produces a clear *errors.AppError, not a panic or a
// raw Go error.
func TestPrismSVGCustomMarkMissingRendererErrors(t *testing.T) {
	_, err := renderCustomFixture(t, customFixtureSpec("does-not-exist"))
	if err == nil {
		t.Fatalf("expected an error for an unregistered custom mark renderer, got nil")
	}
	if !strings.Contains(err.Error(), "PRISM_RENDER_CUSTOM_MARK_NOT_FOUND") {
		t.Errorf("expected PRISM_RENDER_CUSTOM_MARK_NOT_FOUND, got: %v", err)
	}
}
