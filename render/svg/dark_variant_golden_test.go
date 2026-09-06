package svg_test

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
	"github.com/frankbardon/prism/render/svg"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/theme"
)

// E4-S2 registers a throwaway dark-paired theme pair for golden
// coverage — per the story's explicit note, no shipped built-in theme
// sets DarkVariant (that stays a future decision), so the pairing
// under test here is a test-only fixture cloned from the built-in
// light/dark themes.
const (
	e4s2DarkCounterpartName  = "e4s2-golden-dark-counterpart"
	e4s2LightWithVariantName = "e4s2-golden-light-with-dark-variant"
)

func init() {
	darkCounterpart := theme.MustGet("dark")
	if err := theme.Register(e4s2DarkCounterpartName, darkCounterpart); err != nil {
		panic("dark_variant_golden_test: register dark counterpart: " + err.Error())
	}
	lightWithVariant := theme.MustGet("light")
	lightWithVariant.DarkVariant = e4s2DarkCounterpartName
	if err := theme.Register(e4s2LightWithVariantName, lightWithVariant); err != nil {
		panic("dark_variant_golden_test: register light-with-variant: " + err.Error())
	}
}

// TestPrismSVGGolden_DarkVariantChromeSwap covers E4-S2's acceptance
// criteria end-to-end: rendering bar_basic.json under a theme whose
// DarkVariant resolves to a registered counterpart must emit a
// doubled <style> block — the base :root{} plus a second
// @media (prefers-color-scheme: dark) { :root{...} } rule carrying
// the counterpart's chrome values — and the golden captures that
// doubled content byte-for-byte. Set UPDATE_GOLDENS=1 to regenerate.
//
// E4-S3 extended this same fixture: bar_basic.json's bars have no
// color-channel encoding, so their fill comes from the static
// per-mark-type theme color (theme.Marks["bar"].Fill) — which now
// also resolves against both light/dark and emits
// fill="var(--prism-resolved-N)" instead of a baked hex once
// DarkVariant is active (see resolveDarkPairedColor /
// applyThemeMarkStyle in encode/encode.go). This golden's byte
// content grew a --prism-resolved-0/1 declaration pair accordingly;
// see dark_variant_marks_golden_test.go for the dedicated categorical
// scale-driven-color coverage E4-S3 added.
func TestPrismSVGGolden_DarkVariantChromeSwap(t *testing.T) {
	got, err := renderFixtureWithTheme(t, "bar_basic.json", e4s2LightWithVariantName)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	update := os.Getenv("UPDATE_GOLDENS") == "1"
	goldenPath := filepath.Join(repoRoot(t), "testdata", "svgs", "theme_dark_variant_chrome_swap.svg")
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
			goldenPath, truncate(want, 1200), truncate(got, 1200))
	}

	// Belt-and-braces structural assertions, independent of the
	// golden diff above, so a future golden regen can't silently
	// drop the feature under test.
	svgText := string(got)
	if !strings.Contains(svgText, "@media (prefers-color-scheme: dark){:root{") {
		t.Errorf("rendered SVG missing dark-variant media query block")
	}
	if strings.Count(svgText, ":root{") != 2 {
		t.Errorf("rendered SVG expected exactly 2 :root{} blocks (base + dark), got %d", strings.Count(svgText, ":root{"))
	}
}

// TestPrismSVGGolden_NoDarkVariant_Unaffected is the negative-space
// twin of the above: the same fixture rendered under the plain
// built-in "light" theme (no DarkVariant) must carry no media query
// at all — proving the feature is additive/opt-in and existing themes
// are untouched.
func TestPrismSVGGolden_NoDarkVariant_Unaffected(t *testing.T) {
	got, err := renderFixtureWithTheme(t, "bar_basic.json", "light")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(got), "prefers-color-scheme") {
		t.Errorf("rendered SVG under plain 'light' theme unexpectedly contains a dark media query")
	}
}

func renderFixtureWithTheme(t *testing.T, name, themeName string) ([]byte, error) {
	t.Helper()
	path := filepath.Join(repoRoot(t), "examples", "specs", name)
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s, err := spec.DecodeBytes(body)
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
