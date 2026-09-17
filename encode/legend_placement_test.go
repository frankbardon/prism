package encode_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// legendSpec renders a three-category bar chart whose color legend
// carries the supplied legend block (empty string ⇒ no block).
func legendSpec(legendBlock string) string {
	block := ""
	if legendBlock != "" {
		block = ", \"legend\": " + legendBlock
	}
	return fmt.Sprintf(`{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"k": "a", "v": 3, "g": "north"},
    {"k": "b", "v": 5, "g": "south"},
    {"k": "c", "v": 7, "g": "east"}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "k", "type": "nominal"},
    "y": {"field": "v", "type": "quantitative"},
    "color": {"field": "g", "type": "nominal"%s}
  }
}`, block)
}

// encodeInlineCompositeScene is encodeInline's composite twin (returning the
// assembled *scene.Scene; encode_hide_test.go's encodeInlineComposite returns
// the *scene.SceneDoc wrapper): it decodes
// an inline layered spec, executes each child, and returns the single
// assembled Scene.
func encodeInlineCompositeScene(t *testing.T, body string) *scene.Scene {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	c, err := build.BuildComposite(s, build.Options{
		FS:       afero.NewMemMapFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("BuildComposite: %v", err)
	}
	per := make([]map[plan.NodeID]*table.Table, len(c.Children))
	for i, child := range c.Children {
		res, err := plan.Execute(context.Background(), child.DAG, plan.ExecOpts{})
		if err != nil {
			t.Fatalf("Execute child %d: %v", i, err)
		}
		if len(res.Errors) > 0 {
			t.Fatalf("child %d had %d errors: %v", i, len(res.Errors), res.Errors)
		}
		per[i] = res.Tables
	}
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	if len(doc.Grid.Cells) != 1 {
		t.Fatalf("cells = %d, want 1", len(doc.Grid.Cells))
	}
	return &doc.Grid.Cells[0].Scene
}

func onlyLegend(t *testing.T, sc *scene.Scene) scene.Legend {
	t.Helper()
	if len(sc.Legends) != 1 {
		t.Fatalf("legends = %d, want 1", len(sc.Legends))
	}
	return sc.Legends[0]
}

// TestPrismLegendOrientReachesEveryPosition pins that all eight
// scene.LegendPosition values are reachable from legend.orient.
func TestPrismLegendOrientReachesEveryPosition(t *testing.T) {
	all := []scene.LegendPosition{
		scene.LegendLeft, scene.LegendRight, scene.LegendTop, scene.LegendBottom,
		scene.LegendTopLeft, scene.LegendTopRight,
		scene.LegendBottomLeft, scene.LegendBottomRight,
	}
	for _, want := range all {
		t.Run(string(want), func(t *testing.T) {
			sc := encodeInline(t, legendSpec(fmt.Sprintf(`{"orient": %q}`, want)))
			if got := onlyLegend(t, sc).Position; got != want {
				t.Errorf("Position = %q, want %q", got, want)
			}
		})
	}
}

// TestPrismLegendOrientNoneSuppresses pins that orient "none" builds
// no legend at all — and reserves no margin for one.
func TestPrismLegendOrientNoneSuppresses(t *testing.T) {
	sc := encodeInline(t, legendSpec(`{"orient": "none"}`))
	if len(sc.Legends) != 0 {
		t.Fatalf("legends = %d, want 0", len(sc.Legends))
	}
	bare := encodeInline(t, legendSpec(""))
	if sc.Plot != bare.Plot {
		t.Errorf("suppressed Plot = %+v, want the default %+v", sc.Plot, bare.Plot)
	}
}

// TestPrismLegendDefaultPlacementUnchanged pins that a spec with no
// legend block keeps the historical top-right overlay: the frame sits
// inside the plot rect and the plot rect is the default one.
func TestPrismLegendDefaultPlacementUnchanged(t *testing.T) {
	sc := encodeInline(t, legendSpec(""))
	lg := onlyLegend(t, sc)
	if lg.Position != scene.LegendTopRight {
		t.Errorf("Position = %q, want top-right", lg.Position)
	}
	if sc.Plot.X != 40 || sc.Plot.Y != 20 {
		t.Errorf("Plot = %+v, want the default {40,20,...}", sc.Plot)
	}
	if lg.Frame.Right() != sc.Plot.Right() || lg.Frame.Y != sc.Plot.Y {
		t.Errorf("Frame = %+v, want flush with the plot's top-right corner %+v", lg.Frame, sc.Plot)
	}
}

// TestPrismLegendSidePlacementsReserveMargin is the heart of E1-S3: a
// side orient widens the padding on that side and parks the legend in
// the reserved band, clear of the axis chrome.
func TestPrismLegendSidePlacementsReserveMargin(t *testing.T) {
	bare := encodeInline(t, legendSpec(""))
	for _, pos := range []scene.LegendPosition{
		scene.LegendLeft, scene.LegendRight, scene.LegendTop, scene.LegendBottom,
	} {
		t.Run(string(pos), func(t *testing.T) {
			sc := encodeInline(t, legendSpec(fmt.Sprintf(`{"orient": %q}`, pos)))
			lg := onlyLegend(t, sc)
			if sc.Plot == bare.Plot {
				t.Fatalf("Plot = %+v unchanged — the side reserved no margin", sc.Plot)
			}
			// The legend must land outside the plot rect...
			if overlapsRect(lg.Frame, sc.Plot) {
				t.Errorf("Frame %+v overlaps Plot %+v", lg.Frame, sc.Plot)
			}
			// ...and inside the canvas.
			if lg.Frame.X < 0 || lg.Frame.Y < 0 ||
				lg.Frame.Right() > sc.Frame.Right() || lg.Frame.Bottom() > sc.Frame.Bottom() {
				t.Errorf("Frame %+v escapes the canvas %+v", lg.Frame, sc.Frame)
			}
		})
	}
}

// TestPrismLegendLeftClearsTheAxis pins the acceptance case verbatim:
// orient "left" must not overlap the y axis, which lives in the 20-px
// axis band immediately left of the plot.
func TestPrismLegendLeftClearsTheAxis(t *testing.T) {
	sc := encodeInline(t, legendSpec(`{"orient": "left"}`))
	lg := onlyLegend(t, sc)
	var yAxis *scene.Axis
	for i := range sc.Axes {
		if sc.Axes[i].Channel == scene.ChannelY {
			yAxis = &sc.Axes[i]
		}
	}
	if yAxis == nil {
		t.Fatal("no y axis in the scene")
	}
	// The axis renders its ticks and labels leftward from the plot
	// edge; the legend must finish before that band starts.
	axisBandStart := sc.Plot.X - 20
	if lg.Frame.Right() > axisBandStart {
		t.Errorf("legend right edge %v intrudes on the y-axis band starting at %v",
			lg.Frame.Right(), axisBandStart)
	}
	if lg.Frame.X < 0 {
		t.Errorf("legend X = %v, want it inside the canvas", lg.Frame.X)
	}
}

// TestPrismLegendCornerPlacementsOverlay pins the other half of the
// split: corners never reserve a side, they overlay the plot.
func TestPrismLegendCornerPlacementsOverlay(t *testing.T) {
	bare := encodeInline(t, legendSpec(""))
	for _, pos := range []scene.LegendPosition{
		scene.LegendTopLeft, scene.LegendTopRight,
		scene.LegendBottomLeft, scene.LegendBottomRight,
	} {
		t.Run(string(pos), func(t *testing.T) {
			sc := encodeInline(t, legendSpec(fmt.Sprintf(`{"orient": %q}`, pos)))
			if sc.Plot != bare.Plot {
				t.Errorf("Plot = %+v, want the unreserved default %+v", sc.Plot, bare.Plot)
			}
			lg := onlyLegend(t, sc)
			if !overlapsRect(lg.Frame, sc.Plot) {
				t.Errorf("Frame %+v does not overlay Plot %+v", lg.Frame, sc.Plot)
			}
		})
	}
}

// TestPrismLegendOffsetWidensTheGap pins legend.offset: on a side
// placement it grows the gap between the legend and the plot; on a
// corner placement it insets the frame from the corner.
func TestPrismLegendOffsetWidensTheGap(t *testing.T) {
	base := encodeInline(t, legendSpec(`{"orient": "left"}`))
	wide := encodeInline(t, legendSpec(`{"orient": "left", "offset": 40}`))
	baseGap := base.Plot.X - onlyLegend(t, base).Frame.Right()
	wideGap := wide.Plot.X - onlyLegend(t, wide).Frame.Right()
	if wideGap-baseGap != 30 {
		t.Errorf("gap grew by %v, want 30 (offset 10 → 40)", wideGap-baseGap)
	}

	corner := encodeInline(t, legendSpec(`{"orient": "top-left", "offset": 12}`))
	lg := onlyLegend(t, corner)
	if lg.Frame.X != corner.Plot.X+12 || lg.Frame.Y != corner.Plot.Y+12 {
		t.Errorf("corner Frame = %+v, want inset 12 from Plot %+v", lg.Frame, corner.Plot)
	}
}

// TestPrismLegendPaddingGrowsTheFrame pins legend.padding: it grows
// the frame in both dimensions, travels to the renderer on the Scene
// IR, and widens the reserved band rather than eating into the plot.
func TestPrismLegendPaddingGrowsTheFrame(t *testing.T) {
	base := onlyLegend(t, encodeInline(t, legendSpec(`{"orient": "left"}`)))
	padded := encodeInline(t, legendSpec(`{"orient": "left", "padding": 6}`))
	lg := onlyLegend(t, padded)
	if lg.Padding != 6 {
		t.Errorf("scene Legend.Padding = %v, want 6", lg.Padding)
	}
	if lg.Frame.W-base.Frame.W != 12 || lg.Frame.H-base.Frame.H != 12 {
		t.Errorf("Frame grew by %vx%v, want 12x12", lg.Frame.W-base.Frame.W, lg.Frame.H-base.Frame.H)
	}
	if overlapsRect(lg.Frame, padded.Plot) {
		t.Errorf("padded Frame %+v overlaps Plot %+v", lg.Frame, padded.Plot)
	}
}

// TestPrismResolveLegendPlacementDefaults covers the spec → placement
// mapping directly, including the unknown-orient fallback.
func TestPrismResolveLegendPlacementDefaults(t *testing.T) {
	pl, ok := encode.ResolveLegendPlacement(nil, scene.LegendTopRight)
	if !ok || pl.Position != scene.LegendTopRight || pl.Offset != 0 || pl.Padding != 0 {
		t.Errorf("nil block → %+v ok=%v, want the bare top-right default", pl, ok)
	}
	pl, ok = encode.ResolveLegendPlacement(&spec.Legend{Orient: "left"}, scene.LegendTopRight)
	if !ok || pl.Position != scene.LegendLeft || pl.Offset != 10 {
		t.Errorf("orient left → %+v ok=%v, want left with the 10-px default gap", pl, ok)
	}
	pl, ok = encode.ResolveLegendPlacement(&spec.Legend{Orient: "sideways"}, scene.LegendTopRight)
	if !ok || pl.Position != scene.LegendTopRight {
		t.Errorf("unknown orient → %+v ok=%v, want the default position", pl, ok)
	}
	if _, ok := encode.ResolveLegendPlacement(&spec.Legend{Orient: encode.LegendOrientNone}, scene.LegendTopRight); ok {
		t.Error("orient none → enabled, want suppressed")
	}
}

// TestPrismIsSideLegend pins the side/corner split the whole story
// turns on.
func TestPrismIsSideLegend(t *testing.T) {
	sides := map[scene.LegendPosition]bool{
		scene.LegendLeft: true, scene.LegendRight: true,
		scene.LegendTop: true, scene.LegendBottom: true,
		scene.LegendTopLeft: false, scene.LegendTopRight: false,
		scene.LegendBottomLeft: false, scene.LegendBottomRight: false,
	}
	for pos, want := range sides {
		if got := encode.IsSideLegend(pos); got != want {
			t.Errorf("IsSideLegend(%q) = %v, want %v", pos, got, want)
		}
	}
}

// twoLayerLegendSpec layers two color-bound marks, each carrying the
// supplied legend block, so both resolve a legend at the same anchor.
func twoLayerLegendSpec(legendBlock string) string {
	return fmt.Sprintf(`{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"k": "a", "v": 3, "g": "north", "h": "alpha"},
    {"k": "b", "v": 5, "g": "south", "h": "beta"},
    {"k": "c", "v": 7, "g": "east",  "h": "gamma"}
  ]},
  "layer": [
    {"mark": "bar", "encoding": {
      "x": {"field": "k", "type": "nominal"},
      "y": {"field": "v", "type": "quantitative"},
      "color": {"field": "g", "type": "nominal", "legend": %[1]s}}},
    {"mark": "point", "encoding": {
      "x": {"field": "k", "type": "nominal"},
      "y": {"field": "v", "type": "quantitative"},
      "color": {"field": "h", "type": "nominal", "legend": %[1]s}}}
  ]
}`, legendBlock)
}

// TestPrismLegendLayerStackingSurvivesSideOrient pins the composite
// half of E1-S3: two layers anchored to the same side still stack
// without colliding, and the reserved band holds both.
func TestPrismLegendLayerStackingSurvivesSideOrient(t *testing.T) {
	for _, orient := range []string{"right", "top-right"} {
		t.Run(orient, func(t *testing.T) {
			sc := encodeInlineCompositeScene(t, twoLayerLegendSpec(fmt.Sprintf(`{"orient": %q}`, orient)))
			if len(sc.Legends) != 2 {
				t.Fatalf("legends = %d, want 2", len(sc.Legends))
			}
			a, b := sc.Legends[0].Frame, sc.Legends[1].Frame
			if overlapsRect(a, b) {
				t.Errorf("stacked legend frames collide: %+v vs %+v", a, b)
			}
			for i, lg := range sc.Legends {
				if lg.Frame.Right() > sc.Frame.Right() || lg.Frame.Bottom() > sc.Frame.Bottom() {
					t.Errorf("legends[%d].Frame %+v escapes the canvas %+v", i, lg.Frame, sc.Frame)
				}
			}
		})
	}
}

// overlapsRect reports whether two rects share any area.
func overlapsRect(a, b scene.Rect) bool {
	return a.X < b.Right() && a.Right() > b.X && a.Y < b.Bottom() && a.Bottom() > b.Y
}
