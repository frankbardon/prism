package html_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/render"
	prismhtml "github.com/frankbardon/prism/render/html"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/theme"
)

// E4-S3 registers its own throwaway dark-paired theme, named
// distinctly from render/svg's copies so the two packages' test
// binaries (run independently by `go test ./...`) each get their own
// registration without colliding.
const (
	e4s3HTMLDarkCounterpartName  = "e4s3-html-golden-dark-counterpart"
	e4s3HTMLLightWithVariantName = "e4s3-html-golden-light-with-dark-variant"
)

func init() {
	darkCounterpart := theme.MustGet("dark")
	if err := theme.Register(e4s3HTMLDarkCounterpartName, darkCounterpart); err != nil {
		panic("dark_variant_marks_golden_test: register dark counterpart: " + err.Error())
	}
	lightWithVariant := theme.MustGet("light")
	lightWithVariant.DarkVariant = e4s3HTMLDarkCounterpartName
	if err := theme.Register(e4s3HTMLLightWithVariantName, lightWithVariant); err != nil {
		panic("dark_variant_marks_golden_test: register light-with-variant: " + err.Error())
	}
}

// e4s3CategoricalBarSpec mirrors render/svg's darkVariantCategoricalBarSpec
// (a 3-category bar chart with a categorical color encoding) — kept as
// its own copy rather than an exported cross-package symbol since the
// two _test packages don't share test-only code.
const e4s3CategoricalBarSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "title": "E4-S3 categorical mark colors",
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
  }
}`

// TestPrismHTMLGolden_DarkVariantMarkColors confirms render/html
// inherits E4-S3's mark-color auto-dark re-plumb the same way it
// already inherits typography tokens (E2-S2) and gradient/pattern
// defs (E3-S3) — by delegating the whole doc to svg.New().Render, so
// no independent theme logic exists in this package. Set
// UPDATE_GOLDENS=1 to regenerate.
func TestPrismHTMLGolden_DarkVariantMarkColors(t *testing.T) {
	got, err := renderSpecWithTheme(t, e4s3CategoricalBarSpec, e4s3HTMLLightWithVariantName)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	update := os.Getenv("UPDATE_GOLDENS") == "1"
	goldenPath := filepath.Join(repoRoot(t), "render", "html", "testdata", "htmls", "theme_dark_variant_mark_colors.html")
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
			goldenPath, truncate(want, 1600), truncate(got, 1600))
	}

	htmlText := string(got)
	if !strings.Contains(htmlText, "@media (prefers-color-scheme: dark)") {
		t.Errorf("rendered HTML missing dark-variant media query block")
	}
	barCount := strings.Count(htmlText, `class="prism-mark-bar"`)
	varCount := strings.Count(htmlText, `fill="var(--prism-resolved-`)
	if barCount == 0 {
		t.Fatalf("fixture produced no bar marks — test is vacuous")
	}
	if varCount != barCount {
		t.Errorf("expected all %d bar marks to carry a var(--prism-resolved-N) fill, found %d", barCount, varCount)
	}
}

// renderSpecWithTheme decodes an inline spec literal and renders it
// through the html backend under themeName — mirrors render/svg's
// helper of the same name (separate _test package, so no shared code).
func renderSpecWithTheme(t *testing.T, specJSON, themeName string) ([]byte, error) {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(specJSON))
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
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{
		Width: 800, Height: 600, ThemeName: themeName,
	})
	if err != nil {
		return nil, err
	}
	return prismhtml.New().Render(doc, render.RenderOpts{Format: "html", Width: 800, Height: 600})
}
