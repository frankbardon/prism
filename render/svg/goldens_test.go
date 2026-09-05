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
)

// TestPrismSVGGoldensStable — required by PHASE.md. For each of the
// 5 core marks, run spec -> build -> execute -> encode -> render,
// and diff against the committed golden under testdata/svgs/. Set
// UPDATE_GOLDENS=1 to regenerate.
func TestPrismSVGGoldensStable(t *testing.T) {
	fixtures := []string{
		"bar_basic.json",
		"line_basic.json",
		"area_basic.json",
		"point_scatter.json",
		"rule_basic.json",
		// E1-S2: theme filter escape hatch (raw <filter> defs +
		// filter="url(#...)" attrs) and raw_css passthrough.
		"bar_mark_filter.json",
		"bar_raw_css.json",
		// E2-S2: line-height / letter-spacing typography tokens —
		// axis label/title + legend label/title + chart title
		// (point_typography_tokens.json) and per-mark text-glyph
		// letter-spacing/line-height (text_typography_tokens.json).
		"point_typography_tokens.json",
		"text_typography_tokens.json",
		// E3-S3: gradient/pattern <defs> emission + fill="url(#...)"
		// resolution — one fixture per required coverage case (linear
		// gradient, radial gradient, each of the 4 built-in pattern
		// types, and a raw-content pattern).
		"bar_gradient_linear.json",
		"bar_gradient_radial.json",
		"bar_pattern_diagonal_stripes.json",
		"bar_pattern_dots.json",
		"bar_pattern_cross_hatch.json",
		"bar_pattern_grid.json",
		"bar_pattern_raw_content.json",
	}
	update := os.Getenv("UPDATE_GOLDENS") == "1"
	for _, fix := range fixtures {
		fix := fix
		t.Run(fix, func(t *testing.T) {
			got, err := renderFixture(t, fix)
			if err != nil {
				t.Fatalf("render %s: %v", fix, err)
			}
			goldenName := strings.TrimSuffix(fix, ".json") + ".svg"
			goldenPath := filepath.Join(repoRoot(t), "testdata", "svgs", goldenName)
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

func renderFixture(t *testing.T, name string) ([]byte, error) {
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
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{Width: 800, Height: 600})
	if err != nil {
		return nil, err
	}
	return svg.New().Render(doc, render.RenderOpts{Format: "svg", Width: 800, Height: 600})
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found")
		}
		dir = parent
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...[truncated]"
}
