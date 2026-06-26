package svg_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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
)

// TestPrismThemeVariantsDistinct — required by PHASE.md. Renders
// bar_basic.json under each of light/dark/print and asserts pairwise
// byte-distinct SVG outputs (theme actually takes effect end-to-end).
func TestPrismThemeVariantsDistinct(t *testing.T) {
	fixture := filepath.Join(repoRoot(t), "examples", "specs", "bar_basic.json")
	body, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	renderTheme := func(themeName string) []byte {
		s, err := spec.DecodeBytes(body)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		dag, tipID, err := build.Build(s, build.Options{
			FS:       afero.NewOsFs(),
			Resolver: resolve.New(nil),
			Backend:  inmem.New(),
		})
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
		if err != nil {
			t.Fatalf("execute: %v", err)
		}
		doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{
			Width: 800, Height: 600, ThemeName: themeName,
		})
		if err != nil {
			t.Fatalf("encode theme=%q: %v", themeName, err)
		}
		bytes, err := svg.New().Render(doc, render.RenderOpts{Format: "svg", Width: 800, Height: 600})
		if err != nil {
			t.Fatalf("render theme=%q: %v", themeName, err)
		}
		return bytes
	}
	light := renderTheme("light")
	dark := renderTheme("dark")
	print := renderTheme("print")

	if bytes.Equal(light, dark) {
		t.Errorf("light and dark SVGs are byte-equal; theme did not propagate")
	}
	if bytes.Equal(light, print) {
		t.Errorf("light and print SVGs are byte-equal; theme did not propagate")
	}
	if bytes.Equal(dark, print) {
		t.Errorf("dark and print SVGs are byte-equal; theme did not propagate")
	}
}
