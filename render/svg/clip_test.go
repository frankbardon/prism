package svg

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// clipDoc builds the axisDoc fixture with a plot-region clip armed and
// enough chrome (title, axis, legend) to prove the clip stays off it.
func clipDoc(t *testing.T, above bool) string {
	t.Helper()
	a := bottomAxis()
	if above {
		a.Zindex = 1
	}
	doc := axisDoc(a)
	sc := &doc.Grid.Cells[0].Scene
	sc.Title = &scene.TextElement{Content: "Clipped", X: 400, Y: 20}
	sc.Legends = []scene.Legend{{
		ID:      "legend-color",
		Channel: scene.ChannelColor,
		Title:   "series",
		Frame:   scene.Rect{X: 660, Y: 30, W: 110, H: 40},
		Entries: []scene.LegendEntry{
			{Label: "a", Swatch: scene.SwatchSpec{Color: &scene.Color{R: 1, G: 2, B: 3, A: 1}}},
			{Label: "b", Swatch: scene.SwatchSpec{Color: &scene.Color{R: 4, G: 5, B: 6, A: 1}}},
		},
	}}
	sc.ClipRef = "prism-clip-s1"
	sc.Defs = &scene.Defs{Clips: map[string]scene.Rect{"prism-clip-s1": sc.Plot}}
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(out)
}

// The clip def is emitted and the mark container references it.
func TestPrismSVGPlotClipReachesOnlyTheMarkContainer(t *testing.T) {
	out := clipDoc(t, false)

	if !strings.Contains(out, `<clipPath id="prism-clip-s1">`) {
		t.Fatal("no clipPath def emitted for Scene.Defs.Clips")
	}
	if !strings.Contains(out, `<rect x="40" y="20" width="740" height="540"/>`) {
		t.Error("clipPath rect does not match the plot rect")
	}
	if !strings.Contains(out, `<g class="prism-plot" clip-path="url(#prism-clip-s1)">`) {
		t.Fatal("prism-plot does not reference the clip")
	}
	// Exactly one element carries the reference: the mark container.
	if n := strings.Count(out, `clip-path=`); n != 1 {
		t.Errorf("clip-path attributes = %d, want 1 (the mark container alone)", n)
	}
	// The def must precede the reference so the id resolves.
	if strings.Index(out, `<clipPath`) > strings.Index(out, `clip-path=`) {
		t.Error("clipPath def is emitted after the reference")
	}

	// The chrome that legitimately sits outside the plot rect is
	// outside the clipped group.
	plotAt := strings.Index(out, `class="prism-plot"`)
	plotEnd := plotAt + strings.Index(out[plotAt:], "</g>")
	clipped := out[plotAt:plotEnd]
	for _, chrome := range []string{
		`class="prism-title"`,
		`class="prism-axes"`,
		`class="prism-legends"`,
	} {
		if strings.Contains(clipped, chrome) {
			t.Errorf("%s is inside the clipped mark container", chrome)
		}
		if !strings.Contains(out, chrome) {
			t.Errorf("%s missing from the output entirely", chrome)
		}
	}
}

// E3-S2 emits the above-marks axes group as a sibling of prism-plot
// precisely so a clip on that group cannot reach it. Verify the two
// features compose.
func TestPrismSVGPlotClipSparesTheAboveMarksAxes(t *testing.T) {
	out := clipDoc(t, true)

	plotAt := strings.Index(out, `class="prism-plot"`)
	axesAt := strings.Index(out, `class="prism-axes"`)
	if plotAt < 0 || axesAt < 0 {
		t.Fatalf("missing groups: plot=%d axes=%d", plotAt, axesAt)
	}
	if axesAt < plotAt {
		t.Fatal("zindex 1 axis did not move after the marks")
	}
	plotEnd := plotAt + strings.Index(out[plotAt:], "</g>")
	if axesAt < plotEnd {
		t.Error("above-marks axes group is inside the clipped mark container")
	}
}

// A scene with no ClipRef renders exactly as it did before E2-S2 — no
// clipPath def, no attribute.
func TestPrismSVGNoClipWhenUnarmed(t *testing.T) {
	out := renderAxisDoc(t, bottomAxis(), nil)
	if strings.Contains(out, "clipPath") || strings.Contains(out, "clip-path") {
		t.Error("an unarmed scene emitted clip markup")
	}
}
