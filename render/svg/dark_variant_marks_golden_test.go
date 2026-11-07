package svg_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/render/svg"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// E4-S3 registers its own throwaway dark-paired theme pair — separate
// names from dark_variant_golden_test.go's E4-S2 pair so the two
// tests don't fight over the shared package-level theme registry, but
// the same "clone a built-in light/dark pair" approach (per the
// story's explicit note: no shipped built-in theme sets DarkVariant).
const (
	e4s3DarkCounterpartName  = "e4s3-golden-dark-counterpart"
	e4s3LightWithVariantName = "e4s3-golden-light-with-dark-variant"
)

func init() {
	darkCounterpart := theme.MustGet("dark")
	if err := theme.Register(e4s3DarkCounterpartName, darkCounterpart); err != nil {
		panic("dark_variant_marks_golden_test: register dark counterpart: " + err.Error())
	}
	lightWithVariant := theme.MustGet("light")
	lightWithVariant.DarkVariant = e4s3DarkCounterpartName
	if err := theme.Register(e4s3LightWithVariantName, lightWithVariant); err != nil {
		panic("dark_variant_marks_golden_test: register light-with-variant: " + err.Error())
	}
}

// darkVariantCategoricalBarSpec is an intentionally small (3-category)
// bar chart with a categorical color encoding — the "small fixture
// first" case the story calls out. Built-in light/dark themes resolve
// their categorical Range.Category slot to different named schemes
// (tableau10 vs observable10 — see theme/light.go / theme/dark.go),
// so this fixture's three bars get three genuinely distinct
// light/dark resolved-color pairs, not just one repeated pair.
const darkVariantCategoricalBarSpec = `{
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

// TestPrismSVGGolden_DarkVariantMarkColors is E4-S3's headline
// coverage: a categorical color-encoded bar chart rendered under a
// theme whose DarkVariant resolves to a registered counterpart must
// resolve each distinct scale color against BOTH themes, emit one
// --prism-resolved-N declaration pair per distinct pairing (light
// under :root, dark under the E4-S2 @media block), and have every bar
// reference its pair via fill="var(--prism-resolved-N)" instead of a
// baked hex literal. Set UPDATE_GOLDENS=1 to regenerate.
func TestPrismSVGGolden_DarkVariantMarkColors(t *testing.T) {
	got, err := renderSpecWithTheme(t, darkVariantCategoricalBarSpec, e4s3LightWithVariantName)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	update := os.Getenv("UPDATE_GOLDENS") == "1"
	goldenPath := filepath.Join(repoRoot(t), "testdata", "svgs", "theme_dark_variant_mark_colors.svg")
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
			goldenPath, truncate(want, 1600), truncate(got, 1600))
	}

	svgText := string(got)

	// Structural assertions, independent of the golden diff above, so
	// a future golden regen can't silently drop the feature under
	// test.
	rootBlock, mediaBlock, ok := splitRootAndMediaBlocks(svgText)
	if !ok {
		t.Fatalf("could not locate both :root{} blocks in rendered SVG")
	}
	lightResolvedCount := strings.Count(rootBlock, "--prism-resolved-")
	darkResolvedCount := strings.Count(mediaBlock, "--prism-resolved-")
	if lightResolvedCount == 0 {
		t.Errorf("base :root{} block has no --prism-resolved-N declarations")
	}
	if darkResolvedCount != lightResolvedCount {
		t.Errorf("dark media block has %d --prism-resolved-N declarations, want %d (must match the base block 1:1)",
			darkResolvedCount, lightResolvedCount)
	}
	// Every bar mark — and, since v0.18, every legend swatch — must
	// reference a resolved var, never a baked hex, once auto-dark is
	// active.
	barFillCount := strings.Count(svgText, `class="prism-mark-bar"`)
	swatchCount := strings.Count(svgText, `class="prism-legend-swatch"`)
	varFillCount := strings.Count(svgText, `fill="var(--prism-resolved-`)
	if barFillCount == 0 || swatchCount == 0 {
		t.Fatalf("fixture produced no bar marks or no legend — test is vacuous")
	}
	if varFillCount != barFillCount+swatchCount {
		t.Errorf("expected all %d bar marks and %d legend swatches to carry a var(--prism-resolved-*) fill, found %d", barFillCount, swatchCount, varFillCount)
	}
	if strings.Contains(svgText, `fill="#`) && strings.Count(svgText, `class="prism-mark-bar"`) > 0 {
		// A literal hex fill can legitimately appear elsewhere (e.g. a
		// geoshape stroke default) but must never appear on a bar mark
		// once auto-dark is active — check per-line instead of a bare
		// substring match.
		for _, line := range strings.Split(svgText, "\n") {
			if strings.Contains(line, `class="prism-mark-bar"`) && strings.Contains(line, `fill="#`) {
				t.Errorf("bar mark still carries a baked hex fill under an active DarkVariant: %s", strings.TrimSpace(line))
			}
		}
	}
}

// TestPrismSVGGolden_NoDarkVariant_MarkColorsUnaffected is the
// negative-space twin: the identical categorical-color spec rendered
// under the plain built-in "light" theme (no DarkVariant) must keep
// baking literal hex fills — zero var(--prism-resolved-*) references
// anywhere — proving E4-S3's mark re-plumb is strictly opt-in.
func TestPrismSVGGolden_NoDarkVariant_MarkColorsUnaffected(t *testing.T) {
	got, err := renderSpecWithTheme(t, darkVariantCategoricalBarSpec, "light")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	svgText := string(got)
	if strings.Contains(svgText, "prism-resolved-") {
		t.Errorf("rendered SVG under plain 'light' theme unexpectedly contains a resolved-var reference")
	}
	if strings.Contains(svgText, "prefers-color-scheme") {
		t.Errorf("rendered SVG under plain 'light' theme unexpectedly contains a dark media query")
	}
	if !strings.Contains(svgText, `class="prism-mark-bar"`) || !strings.Contains(svgText, `fill="#`) {
		t.Errorf("rendered SVG expected to keep baking literal hex fills on bar marks without DarkVariant")
	}
}

// darkVariantHConcatSpec is a 2-panel hconcat where each child has its
// own categorical color encoding. E4-S3 left concat marks baked (a
// child that registered its own resolved vars in a theme the composite
// then discarded would have pointed at undeclared properties). Since
// v0.18 the top-level composite owns ONE registry, hands it to every
// cell (encode/autodark.go) and emits the pairs once, so concat marks
// and legend swatches dark-swap like a single view.
const darkVariantHConcatSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "title": "E4-S3 concat + DarkVariant regression",
  "hconcat": [
    {
      "$schema": "urn:prism:schema:v1:spec",
      "data": {"values": [
        {"q": "Q1", "sales": 120}, {"q": "Q2", "sales": 180}
      ]},
      "mark": "bar",
      "encoding": {
        "x": {"field": "q", "type": "nominal"},
        "y": {"field": "sales", "type": "quantitative"},
        "color": {"field": "q", "type": "nominal"}
      }
    },
    {
      "$schema": "urn:prism:schema:v1:spec",
      "data": {"values": [
        {"region": "NA", "share": 0.42}, {"region": "EMEA", "share": 0.3}
      ]},
      "mark": "bar",
      "encoding": {
        "x": {"field": "region", "type": "nominal"},
        "y": {"field": "share", "type": "quantitative"},
        "color": {"field": "region", "type": "nominal"}
      }
    }
  ]
}`

// TestPrismSVGGolden_DarkVariantConcatMarksPaired: a 2-panel hconcat
// under an active DarkVariant paints every bar and every legend swatch
// through a resolved var, and every var it references is declared in
// both the light :root block and the dark media block.
func TestPrismSVGGolden_DarkVariantConcatMarksPaired(t *testing.T) {
	s, err := spec.DecodeBytes([]byte(darkVariantHConcatSpec))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	per := make([]map[plan.NodeID]*table.Table, len(c.Children))
	for i, child := range c.Children {
		res, err := plan.Execute(context.Background(), child.DAG, plan.ExecOpts{})
		if err != nil {
			t.Fatalf("Execute child %d: %v", i, err)
		}
		per[i] = res.Tables
	}
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{
		Width: 800, Height: 600, ThemeName: e4s3LightWithVariantName,
	})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	got, err := svg.New().Render(doc, render.RenderOpts{Format: "svg", Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	svgText := string(got)
	root, media, ok := splitRootAndMediaBlocks(svgText)
	if !ok {
		t.Fatalf("expected a light :root block and a dark media block")
	}
	barCount := strings.Count(svgText, `class="prism-mark-bar"`)
	if barCount != 4 {
		t.Fatalf("expected 4 bar marks (2 panels x 2 rows), got %d", barCount)
	}
	refs := regexp.MustCompile(`(?:fill|stroke)="var\(--(prism-resolved-[^)]+)\)"`).FindAllStringSubmatch(svgText, -1)
	if len(refs) < barCount+4 {
		t.Errorf("expected every bar and the 4 legend swatches to paint through a resolved var, found %d references", len(refs))
	}
	for _, m := range refs {
		if !strings.Contains(root, "--"+m[1]+":") || !strings.Contains(media, "--"+m[1]+":") {
			t.Errorf("var --%s is referenced but not declared in both blocks", m[1])
		}
	}
	if regexp.MustCompile(`class="prism-mark-bar"[^>]*fill="#`).MatchString(svgText) {
		t.Errorf("a bar mark still bakes a hex fill under DarkVariant")
	}
}

// splitRootAndMediaBlocks extracts the base :root{...} declaration
// body and the @media (prefers-color-scheme: dark){:root{...}} body
// from a rendered SVG's <style> block.
func splitRootAndMediaBlocks(svgText string) (root, media string, ok bool) {
	const rootStart = ":root{"
	i := strings.Index(svgText, rootStart)
	if i < 0 {
		return "", "", false
	}
	rest := svgText[i+len(rootStart):]
	end := strings.Index(rest, "}")
	if end < 0 {
		return "", "", false
	}
	root = rest[:end]

	const mediaMarker = "@media (prefers-color-scheme: dark){:root{"
	j := strings.Index(svgText, mediaMarker)
	if j < 0 {
		return "", "", false
	}
	mrest := svgText[j+len(mediaMarker):]
	mend := strings.Index(mrest, "}}")
	if mend < 0 {
		return "", "", false
	}
	media = mrest[:mend]
	return root, media, true
}

// renderSpecWithTheme mirrors renderFixtureWithTheme (dark_variant_
// golden_test.go) but decodes an inline spec literal instead of
// reading examples/specs/<name>.json — this fixture is test-only
// scaffolding, not part of the shared example/gallery corpus.
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
	return svg.New().Render(doc, render.RenderOpts{Format: "svg", Width: 800, Height: 600})
}
