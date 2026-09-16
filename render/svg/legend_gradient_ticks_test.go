package svg

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// gradientLegendDoc wraps one gradient legend carrying the supplied
// labelled stops into a renderable doc.
func gradientLegendDoc(ticks []scene.LegendTick) *scene.SceneDoc {
	doc := scene.NewDoc()
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: scene.Scene{
				ID:    "s1",
				Frame: scene.Rect{W: 800, H: 600},
				Plot:  scene.Rect{X: 40, Y: 20, W: 600, H: 540},
				Legends: []scene.Legend{{
					ID:       "legend-color",
					Channel:  scene.ChannelColor,
					Position: scene.LegendRight,
					Frame:    scene.Rect{X: 650, Y: 20, W: 104, H: 146},
					Entries: []scene.LegendEntry{{
						Label:  "0–100",
						Swatch: scene.SwatchSpec{Type: scene.SwatchGradient, GradientID: "grad-0"},
						Ticks:  ticks,
					}},
				}},
			}},
		},
	}
	return doc
}

// TestPrismLegendGradientTicksRendered covers E3-S3's renderer half:
// a gradient entry carrying labelled stops emits one <text> per stop,
// placed proportionally down the bar, instead of the single summary
// label.
func TestPrismLegendGradientTicksRendered(t *testing.T) {
	doc := gradientLegendDoc([]scene.LegendTick{
		{Offset: 0, Label: "0"},
		{Offset: 0.5, Label: "50"},
		{Offset: 1, Label: "100"},
	})
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	// Bar height is Frame.H - 16 = 130; the first row sits at
	// Frame.Y + 8 = 28, so the stops land at 32 / 97 / 162.
	for _, want := range []string{
		`<text class="prism-legend-label" x="672" y="32">0</text>`,
		`<text class="prism-legend-label" x="672" y="97">50</text>`,
		`<text class="prism-legend-label" x="672" y="162">100</text>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %s\ngot:\n%s", want, s)
		}
	}
	if strings.Contains(s, ">0–100<") {
		t.Error("summary label rendered alongside the labelled stops; want stops only")
	}
}

// TestPrismLegendGradientNoTicksFallsBack pins the back-compatible
// branch: an entry with no stops still draws its own summary label.
func TestPrismLegendGradientNoTicksFallsBack(t *testing.T) {
	out, err := New().Render(gradientLegendDoc(nil), render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(out), ">0–100<") {
		t.Errorf("output missing the summary label:\n%s", out)
	}
}
