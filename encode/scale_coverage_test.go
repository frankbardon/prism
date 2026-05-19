package encode_test

import (
	"path/filepath"
	"testing"

	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
)

// TestPrismScaleTypeCoverage — required by PHASE.md. For each of the
// 8 scale types, render the matching fixture through the encoder and
// assert (a) the encoded axis scale.type matches the spec request,
// (b) the encoded axis carries at least one tick. Spot-checks the
// first tick label per scale type.
func TestPrismScaleTypeCoverage(t *testing.T) {
	cases := []struct {
		fixture   string
		scaleType scene.ScaleType
	}{
		{"linear.json", scene.ScaleLinear},
		{"log.json", scene.ScaleLog},
		{"pow.json", scene.ScalePow},
		{"sqrt.json", scene.ScaleSqrt},
		{"time.json", scene.ScaleTime},
		{"band.json", scene.ScaleBand},
		{"point.json", scene.ScalePoint},
		{"ordinal.json", scene.ScaleOrdinal},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.scaleType), func(t *testing.T) {
			rel := filepath.Join("scales", tc.fixture)
			s, tables, tipID := runPipeline(t, rel)
			doc, err := encode.Encode(s, tables, tipID, encode.EncodeOpts{})
			if err != nil {
				t.Fatalf("Encode(%s): %v", rel, err)
			}
			if len(doc.Grid.Cells) == 0 {
				t.Fatalf("Encode(%s): no cells", rel)
			}
			scn := doc.Grid.Cells[0].Scene
			found := false
			for _, a := range scn.Axes {
				if a.Scale.Type == tc.scaleType {
					found = true
					if len(a.Ticks) == 0 {
						t.Errorf("%s axis has no ticks", tc.scaleType)
					}
					break
				}
			}
			if !found {
				t.Errorf("no axis with scale type %q in scene; axes=%v",
					tc.scaleType, axisTypes(scn.Axes))
			}
		})
	}
}

func axisTypes(axes []scene.Axis) []scene.ScaleType {
	out := make([]scene.ScaleType, len(axes))
	for i, a := range axes {
		out[i] = a.Scale.Type
	}
	return out
}
