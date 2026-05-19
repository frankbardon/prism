package svg_test

import (
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
	"github.com/frankbardon/prism/table"
)

func renderFacetSpec(t *testing.T, fixture string) []byte {
	t.Helper()
	root := repoRoot(t)
	path := filepath.Join(root, "testdata", "specs", fixture)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{
		FS: afero.NewOsFs(), Resolver: resolve.New(nil), Backend: inmem.New(),
	})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	per := make([]map[plan.NodeID]*table.Table, len(c.Children))
	for i, ch := range c.Children {
		res, err := plan.Execute(context.Background(), ch.DAG, plan.ExecOpts{})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}
		per[i] = res.Tables
	}
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	out, err := svg.New().Render(doc, render.RenderOpts{Format: "svg", Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return out
}

// TestPrismRenderFacetEmitsHeaders pins facet column headers: the
// facet_by_region fixture (current variant facets by column only,
// so only Top headers populate) emits one grid-headers group with
// one <text> per column.
func TestPrismRenderFacetEmitsHeaders(t *testing.T) {
	out := renderFacetSpec(t, "facet_by_region.json")
	s := string(out)
	if got := strings.Count(s, `class="prism-grid-headers"`); got != 1 {
		t.Errorf("prism-grid-headers count = %d, want 1", got)
	}
	if got := strings.Count(s, "prism-facet-header"); got == 0 {
		t.Errorf("no prism-facet-header found")
	}
}
