package pdf

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
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// TestPrismPDFRendererNilDoc verifies the defensive nil-doc guard.
func TestPrismPDFRendererNilDoc(t *testing.T) {
	r := New()
	if _, err := r.Render(nil, render.RenderOpts{}); err == nil {
		t.Fatalf("Render(nil) returned no error")
	}
}

// TestPrismPDFRendererMimeType pins the MIME contract.
func TestPrismPDFRendererMimeType(t *testing.T) {
	if got := New().MimeType(); got != "application/pdf" {
		t.Fatalf("MimeType = %q, want application/pdf", got)
	}
}

// TestPrismPDFRendererSingleCellSinglePage renders a minimal 1×1
// scene and asserts the output starts with %PDF- and contains
// exactly one /Type /Page entry.
func TestPrismPDFRendererSingleCellSinglePage(t *testing.T) {
	doc := minimalDoc()
	bytes, err := New().Render(doc, render.RenderOpts{Format: "pdf"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !startsWithPDFHeader(bytes) {
		t.Fatalf("output doesn't start with %%PDF-: first 16 bytes = %q", string(bytes[:16]))
	}
	got := countPages(bytes)
	if got != 1 {
		t.Fatalf("page count = %d, want 1", got)
	}
}

// TestPrismPDFRendererFromBarBasicFixture pipes the canonical
// bar_basic.json fixture through plotPipeline (compile / execute /
// encode) and renders to PDF, asserting the output is well-formed
// + non-trivial.
func TestPrismPDFRendererFromBarBasicFixture(t *testing.T) {
	doc := loadFixture(t, "../../testdata/specs/bar_basic.json")
	bs, err := New().Render(doc, render.RenderOpts{Format: "pdf"})
	if err != nil {
		t.Fatalf("Render bar_basic: %v", err)
	}
	if !startsWithPDFHeader(bs) {
		t.Fatalf("missing PDF header")
	}
	if len(bs) < 2000 {
		t.Fatalf("PDF unexpectedly small (%d bytes) — likely missing content", len(bs))
	}
}

// minimalDoc returns the smallest valid SceneDoc the renderer
// accepts: one cell, one layer, one rect mark, default theme.
func minimalDoc() *scene.SceneDoc {
	doc := scene.NewDoc()
	red, _ := scene.ColorFromHex("#ff0000")
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{{
			Row: 0, Col: 0,
			Scene: scene.Scene{
				ID:    "scene-0",
				Frame: scene.Rect{X: 0, Y: 0, W: 400, H: 300},
				Plot:  scene.Rect{X: 40, Y: 20, W: 320, H: 240},
				Layers: []scene.SceneLayer{{
					ID:   "layer-0",
					Mark: scene.MarkRect,
					Marks: []scene.Mark{{
						Type: scene.MarkRect,
						Rect: &scene.RectGeom{X: 50, Y: 50, W: 100, H: 100},
						Style: scene.Style{
							Fill: red,
						},
					}},
				}},
			},
		}},
	}
	return doc
}

// loadFixture runs a fixture through the full pipeline (build /
// execute / encode) and returns the resulting SceneDoc. Handles
// both flat + composite specs via the same branching logic the CLI
// plotPipeline uses (D049 / D050). Halts the test on any pipeline
// failure.
func loadFixture(t *testing.T, path string) *scene.SceneDoc {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	buildOpts := build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	}
	encOpts := encode.EncodeOpts{
		Width: 600, Height: 400, ThemeName: "light",
	}

	if build.IsComposite(s) {
		composite, err := build.BuildComposite(s, buildOpts)
		if err != nil {
			t.Fatalf("BuildComposite %s: %v", path, err)
		}
		per := make([]map[plan.NodeID]*table.Table, len(composite.Children))
		for i, child := range composite.Children {
			res, err := plan.Execute(context.Background(), child.DAG, plan.ExecOpts{})
			if err != nil {
				t.Fatalf("execute child %d of %s: %v", i, path, err)
			}
			per[i] = res.Tables
		}
		doc, err := encode.EncodeComposite(s, composite, per, encOpts)
		if err != nil {
			t.Fatalf("EncodeComposite %s: %v", path, err)
		}
		return doc
	}

	dag, tipID, err := build.Build(s, buildOpts)
	if err != nil {
		t.Fatalf("build %s: %v", path, err)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
	if err != nil {
		t.Fatalf("execute %s: %v", path, err)
	}
	doc, err := encode.Encode(s, res.Tables, tipID, encOpts)
	if err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	return doc
}

// startsWithPDFHeader checks the first 5 bytes equal %PDF-.
func startsWithPDFHeader(b []byte) bool {
	return len(b) >= 5 && string(b[:5]) == "%PDF-"
}

// countPages counts /Type /Page occurrences (not /Pages — which is
// the catalog parent). Pattern allows variable whitespace between
// /Type and /Page; rejects /Pages by negative-lookahead approx.
func countPages(b []byte) int {
	// Simple scan: locate every "/Type" then check next non-WS
	// chars are "/Page" but not "/Pages".
	n := 0
	for i := 0; i+5 < len(b); i++ {
		if !bytes.HasPrefix(b[i:], []byte("/Type")) {
			continue
		}
		// Skip /Type + whitespace.
		j := i + len("/Type")
		for j < len(b) && (b[j] == ' ' || b[j] == '\t' || b[j] == '\n' || b[j] == '\r') {
			j++
		}
		if !bytes.HasPrefix(b[j:], []byte("/Page")) {
			continue
		}
		// /Page must NOT be followed by "s" (which would make it /Pages).
		after := j + len("/Page")
		if after < len(b) && b[after] == 's' {
			continue
		}
		n++
	}
	return n
}

// _ keeps filepath / strings referenced for future tests that need
// path joining or substring assertions; avoids tactical import
// churn as the test surface grows.
var (
	_ = filepath.Join
	_ = strings.Contains
)
