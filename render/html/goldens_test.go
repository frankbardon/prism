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
)

// TestPrismHTMLGoldensStable proves the html backend + dispatch works
// end-to-end using the simplest existing mark (bar) — there is no
// table-mark Scene IR yet (E1-S2/E1-S3), so this is the integration
// test for E1-S1's foundation. Mirrors render/svg/goldens_test.go's
// shape; set UPDATE_GOLDENS=1 to regenerate.
func TestPrismHTMLGoldensStable(t *testing.T) {
	fixtures := []string{
		"bar_basic.json",
		// E2-S2: confirms render/html inherits the line-height /
		// letter-spacing typography-token wiring from render/svg's
		// shared emitters (the html backend delegates to svg.Render
		// verbatim — no independent theme logic of its own).
		"point_typography_tokens.json",
	}
	update := os.Getenv("UPDATE_GOLDENS") == "1"
	for _, fix := range fixtures {
		fix := fix
		t.Run(fix, func(t *testing.T) {
			got, err := renderFixture(t, fix)
			if err != nil {
				t.Fatalf("render %s: %v", fix, err)
			}
			goldenName := strings.TrimSuffix(fix, ".json") + ".html"
			goldenPath := filepath.Join(repoRoot(t), "render", "html", "testdata", "htmls", goldenName)
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
					goldenPath, truncate(want, 800), truncate(got, 800))
			}
		})
	}
}

// TestPrismHTMLMimeType pins the html backend's MIME type.
func TestPrismHTMLMimeType(t *testing.T) {
	if got := prismhtml.New().MimeType(); got != "text/html" {
		t.Errorf("MimeType = %q, want text/html", got)
	}
}

// TestPrismHTMLIsWellFormedDoc is a light structural sanity check
// (not a golden) so a golden regen accident that drops the doctype
// or the embedded <svg> fails loudly and specifically.
func TestPrismHTMLIsWellFormedDoc(t *testing.T) {
	got, err := renderFixture(t, "bar_basic.json")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(got)
	for _, want := range []string{"<!doctype html>", "<html>", "<svg", "</html>"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, truncate(got, 800))
		}
	}
}

// TestPrismHTMLInheritsThemeFilters is a structural (non-golden)
// check for E1-S2: the html backend delegates the whole non-table,
// non-custom doc straight to svg.New().Render and splices the bytes
// verbatim (see render/html/renderer.go), so it must inherit the
// same <filter> defs, filter="url(#...)" mark attrs, and raw_css
// passthrough the svg backend emits — with no separate glue code.
// Verified by test rather than visual inspection per the story's
// acceptance criteria.
func TestPrismHTMLInheritsThemeFilters(t *testing.T) {
	t.Run("mark filter", func(t *testing.T) {
		got, err := renderFixture(t, "bar_mark_filter.json")
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		s := string(got)
		for _, want := range []string{
			`<filter id="prism-filter-drop-shadow">`,
			`filter="url(#prism-filter-drop-shadow)"`,
		} {
			if !strings.Contains(s, want) {
				t.Errorf("output missing %q:\n%s", want, truncate(got, 1200))
			}
		}
	})

	t.Run("raw_css", func(t *testing.T) {
		got, err := renderFixture(t, "bar_raw_css.json")
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		s := string(got)
		if !strings.Contains(s, ".prism-title{letter-spacing:0.5px;}") {
			t.Errorf("output missing raw_css passthrough:\n%s", truncate(got, 1200))
		}
	})
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
	return prismhtml.New().Render(doc, render.RenderOpts{Format: "html", Width: 800, Height: 600})
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
