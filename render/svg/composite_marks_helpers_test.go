package svg_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/marks"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// loadAndEncodeBytes decodes the spec body, runs build + execute +
// encode, and returns the SceneDoc. Used by the P10 in-IR test
// gates that inspect Mark.Tooltip / ArcGeom angles directly.
func loadAndEncodeBytes(t *testing.T, body []byte) *scene.SceneDoc {
	t.Helper()
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
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{Width: 800, Height: 600})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return doc
}

// callHistogramEncoderDirect loads histogram.json + executes its
// pipeline + calls EncodeHistogram directly so the test can inspect
// BinEdges / Counts (which the SceneDoc does not expose). Used by
// TestPrismHistogramAutoBin.
func callHistogramEncoderDirect(t *testing.T) *marks.HistogramResult {
	t.Helper()
	path := filepath.Join(repoRoot(t), "examples", "specs", "histogram.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
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
	tbl := res.Tables[tipID]
	in := marks.Inputs{
		Table:  tbl,
		X:      marks.Channel{Field: s.Encoding.X.Field},
		Layout: scene.Rect{X: 40, Y: 20, W: 740, H: 540},
	}
	hr, err := marks.EncodeHistogram(in)
	if err != nil {
		t.Fatalf("EncodeHistogram: %v", err)
	}
	return hr
}
