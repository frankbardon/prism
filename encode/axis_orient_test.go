package encode_test

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
)

// orientBarSpec is the flat fixture: one slot per position channel so
// a single template covers every combination of `axis.orient`,
// `"axis": null` and an absent key.
const orientBarSpec = `{
  "data": {"values": [{"cat": "a", "val": 3}, {"cat": "b", "val": 5}, {"cat": "c", "val": 2}]},
  "mark": {"type": "bar"},
  "encoding": {
    "x": {"field": "cat", "type": "nominal"<X>},
    "y": {"field": "val", "type": "quantitative"<Y>}
  }
}`

func buildOrientSpec(x, y string) string {
	return strings.NewReplacer("<X>", x, "<Y>", y).Replace(orientBarSpec)
}

// axisByChannel returns the built axis for one channel, or nil.
func axisByChannel(sc *scene.Scene, ch scene.Channel) *scene.Axis {
	for i := range sc.Axes {
		if sc.Axes[i].Channel == ch {
			return &sc.Axes[i]
		}
	}
	return nil
}

// TestPrismAxisPositionFor pins the orient → side mapping, including
// the fallbacks: an empty orient and one meaningless for the channel
// both land on the channel's default side. The meaningless case is
// PRISM_SPEC_044's job to report; the encoder only has to stay total.
func TestPrismAxisPositionFor(t *testing.T) {
	cases := []struct {
		channel scene.Channel
		orient  string
		want    scene.AxisPosition
	}{
		{scene.ChannelX, "", scene.AxisPositionBottom},
		{scene.ChannelX, "bottom", scene.AxisPositionBottom},
		{scene.ChannelX, "top", scene.AxisPositionTop},
		{scene.ChannelX, "left", scene.AxisPositionBottom},
		{scene.ChannelX, "sideways", scene.AxisPositionBottom},
		{scene.ChannelY, "", scene.AxisPositionLeft},
		{scene.ChannelY, "left", scene.AxisPositionLeft},
		{scene.ChannelY, "right", scene.AxisPositionRight},
		{scene.ChannelY, "top", scene.AxisPositionLeft},
		{scene.ChannelColor, "top", ""},
	}
	for _, c := range cases {
		if got := encode.AxisPositionFor(c.channel, c.orient); got != c.want {
			t.Errorf("AxisPositionFor(%q, %q) = %q, want %q", c.channel, c.orient, got, c.want)
		}
	}
}

// TestPrismEncodeAxisOrientTopMovesXAxis is the E1-S2 headline: an x
// axis with `"orient": "top"` renders above the plot and takes its
// padding reservation with it — the top side reserves room for the
// labels and title, and the bottom side releases it.
func TestPrismEncodeAxisOrientTopMovesXAxis(t *testing.T) {
	base := encodeInline(t, buildOrientSpec("", ""))
	top := encodeInline(t, buildOrientSpec(`, "axis": {"orient": "top"}`, ""))

	if base.Plot.Y != 20 || base.Plot.H != 540 {
		t.Fatalf("baseline plot = %+v, want Y=20 H=540", base.Plot)
	}
	// Top gains the 20px axis reservation, bottom gives it back: the
	// plot keeps its height and slides down.
	if top.Plot.Y != 40 || top.Plot.H != 540 {
		t.Errorf("top-oriented plot = %+v, want Y=40 H=540", top.Plot)
	}

	ax := axisByChannel(top, scene.ChannelX)
	if ax == nil {
		t.Fatal("no x axis built")
	}
	if ax.Position != scene.AxisPositionTop {
		t.Errorf("x axis Position = %q, want top", ax.Position)
	}
	// The domain line moves to the plot's top edge...
	if ax.Domain.Y1 != top.Plot.Y || ax.Domain.Y2 != top.Plot.Y {
		t.Errorf("x domain line = %+v, want both Y at plot top %v", ax.Domain, top.Plot.Y)
	}
	// ...and the grid lines still span the plot vertically, one per
	// major tick, anchored to the moved rect.
	if len(ax.Grid) == 0 {
		t.Fatal("top-oriented x axis emitted no grid lines")
	}
	for _, g := range ax.Grid {
		if g.Y1 != top.Plot.Y || g.Y2 != top.Plot.Bottom() {
			t.Errorf("grid line %+v does not span the plot rect %+v", g, top.Plot)
		}
	}
}

// TestPrismEncodeAxisOrientRightMovesYAxis is the symmetric case.
func TestPrismEncodeAxisOrientRightMovesYAxis(t *testing.T) {
	right := encodeInline(t, buildOrientSpec("", `, "axis": {"orient": "right"}`))

	// Left releases its 20px axis reservation, right claims it.
	if right.Plot.X != 20 || right.Plot.W != 740 {
		t.Errorf("right-oriented plot = %+v, want X=20 W=740", right.Plot)
	}
	ax := axisByChannel(right, scene.ChannelY)
	if ax == nil {
		t.Fatal("no y axis built")
	}
	if ax.Position != scene.AxisPositionRight {
		t.Errorf("y axis Position = %q, want right", ax.Position)
	}
	if ax.Domain.X1 != right.Plot.Right() || ax.Domain.X2 != right.Plot.Right() {
		t.Errorf("y domain line = %+v, want both X at plot right %v", ax.Domain, right.Plot.Right())
	}
	for _, g := range ax.Grid {
		if g.X1 != right.Plot.X || g.X2 != right.Plot.Right() {
			t.Errorf("grid line %+v does not span the plot rect %+v", g, right.Plot)
		}
	}
}

// TestPrismEncodeAxisOrientBothSidesMirrorDefault pins the whole
// point of deriving padding from the placement: flipping both axes
// mirrors the default layout rather than distorting it.
func TestPrismEncodeAxisOrientBothSidesMirrorDefault(t *testing.T) {
	flipped := encodeInline(t, buildOrientSpec(
		`, "axis": {"orient": "top"}`, `, "axis": {"orient": "right"}`))
	if flipped.Plot.X != 20 || flipped.Plot.Y != 40 ||
		flipped.Plot.W != 740 || flipped.Plot.H != 540 {
		t.Errorf("flipped plot = %+v, want {20,40,740,540}", flipped.Plot)
	}
}

// TestPrismEncodeAxisOrientComposesWithHidden pins the E1-S4
// interaction: a hidden axis reserves nothing on any side, so moving
// the *other* axis is the only thing that shifts the plot.
func TestPrismEncodeAxisOrientComposesWithHidden(t *testing.T) {
	sc := encodeInline(t, buildOrientSpec(`, "axis": null`, `, "axis": {"orient": "right"}`))
	if got := axisByChannel(sc, scene.ChannelX); got != nil {
		t.Error("hidden x axis still emitted")
	}
	// x hidden: no top or bottom reservation. y on the right: left
	// bare, right reserved.
	if sc.Plot.X != 20 || sc.Plot.Y != 20 || sc.Plot.W != 740 || sc.Plot.H != 560 {
		t.Errorf("plot = %+v, want {20,20,740,560}", sc.Plot)
	}
}

// layerOrientSpec stacks a bar and a line layer; each layer's x
// channel takes a slot so a test can decide which layer (if any)
// declares the orient.
const layerOrientSpec = `{
  "data": {"values": [{"cat": "a", "val": 3}, {"cat": "b", "val": 5}, {"cat": "c", "val": 2}]},
  "layer": [
    {
      "mark": {"type": "bar"},
      "encoding": {
        "x": {"field": "cat", "type": "nominal"<X0>},
        "y": {"field": "val", "type": "quantitative"}
      }
    },
    {
      "mark": {"type": "line"},
      "encoding": {
        "x": {"field": "cat", "type": "nominal"<X1>},
        "y": {"field": "val", "type": "quantitative"}
      }
    }
  ]
}`

// TestPrismEncodeLayerAxisOrientShared is the acceptance case the
// story calls out: orient has to survive composition under the
// *default* (shared) resolve mode, not just on a flat spec. E3-S5's
// reflective fold covers `orient` for free — this pins that it does,
// and that the shared padding follows the shared axis.
func TestPrismEncodeLayerAxisOrientShared(t *testing.T) {
	encodeLayers := func(x0, x1 string) *scene.SceneDoc {
		return encodeInlineComposite(t, strings.NewReplacer("<X0>", x0, "<X1>", x1).Replace(layerOrientSpec))
	}

	base := encodeLayers("", "")
	if got := base.Grid.Cells[0].Scene.Plot.Y; got != 20 {
		t.Fatalf("baseline layer plot Y = %v, want 20", got)
	}

	// Only the first layer asks for top; the shared axis honours it.
	top := encodeLayers(`, "axis": {"orient": "top"}`, "")
	if top.Grid.Shared.X == nil {
		t.Fatal("no shared x axis built")
	}
	if got := top.Grid.Shared.X.Position; got != scene.AxisPositionTop {
		t.Errorf("shared x axis Position = %q, want top", got)
	}
	cell := top.Grid.Cells[0].Scene
	if cell.Plot.Y != 40 || cell.Plot.H != 540 {
		t.Errorf("layered plot = %+v, want Y=40 H=540 (top reserved, bottom released)", cell.Plot)
	}
	if got := top.Grid.Shared.X.Domain.Y1; got != cell.Plot.Y {
		t.Errorf("shared x domain Y = %v, want plot top %v", got, cell.Plot.Y)
	}

	// A second layer that agrees adds no conflict warning.
	agreed := encodeLayers(`, "axis": {"orient": "top"}`, `, "axis": {"orient": "top"}`)
	for _, w := range agreed.Warnings {
		if w.Code == scene.WarnAxisConfigConflict {
			t.Errorf("agreeing layers raised a conflict warning: %s", w.Message)
		}
	}

	// A second layer that disagrees loses (first specified wins) and
	// says so exactly once — the early fold that places the padding
	// must not double-report.
	conflict := encodeLayers(`, "axis": {"orient": "top"}`, `, "axis": {"orient": "bottom"}`)
	if got := conflict.Grid.Shared.X.Position; got != scene.AxisPositionTop {
		t.Errorf("conflicting layers: Position = %q, want top (first specified wins)", got)
	}
	n := 0
	for _, w := range conflict.Warnings {
		if w.Code == scene.WarnAxisConfigConflict {
			n++
		}
	}
	if n != 1 {
		t.Errorf("axis conflict warnings = %d, want exactly 1", n)
	}
}

// facetOrientSpec facets a bar chart by one column; the child spec's
// x channel takes the orient slot.
const facetOrientSpec = `{
  "data": {"values": [
    {"region": "NA", "cat": "a", "val": 3},
    {"region": "NA", "cat": "b", "val": 5},
    {"region": "EU", "cat": "a", "val": 2},
    {"region": "EU", "cat": "b", "val": 4}
  ]},
  "facet": {"row": {"field": "region", "type": "nominal"}},
  "spec": {
    "mark": {"type": "bar"},
    "encoding": {
      "x": {"field": "cat", "type": "nominal"<X>},
      "y": {"field": "val", "type": "quantitative"}
    }
  }
}`

// TestPrismEncodeFacetAxisOrientShared is the faceted counterpart:
// the child spec's orient places the shared axis for the whole grid,
// and the anchor cell follows it — a row facet stacks the cells, so
// the bottom-anchored default and the top-anchored orient land on
// different rects.
func TestPrismEncodeFacetAxisOrientShared(t *testing.T) {
	base := encodeInlineComposite(t, strings.Replace(facetOrientSpec, "<X>", "", 1))
	if base.Grid.Shared.X == nil {
		t.Fatal("baseline: no shared x axis")
	}
	if got := base.Grid.Shared.X.Position; got != scene.AxisPositionBottom {
		t.Fatalf("baseline shared x Position = %q, want bottom", got)
	}
	last := base.Grid.Cells[len(base.Grid.Cells)-1].Scene.Plot
	if len(base.Grid.Cells) < 2 {
		t.Fatalf("baseline cells = %d, want a stacked row facet", len(base.Grid.Cells))
	}
	if got := base.Grid.Shared.X.Domain.Y1; got != last.Bottom() {
		t.Fatalf("baseline shared x domain Y = %v, want last cell's plot bottom %v", got, last.Bottom())
	}

	top := encodeInlineComposite(t,
		strings.Replace(facetOrientSpec, "<X>", `, "axis": {"orient": "top"}`, 1))
	if top.Grid.Shared.X == nil {
		t.Fatal("no shared x axis built")
	}
	if got := top.Grid.Shared.X.Position; got != scene.AxisPositionTop {
		t.Errorf("shared x axis Position = %q, want top", got)
	}
	// Every cell's plot rect slides down inside its own cell box by the
	// reservation that moved from the bottom to the top.
	if len(top.Grid.Cells) != len(base.Grid.Cells) {
		t.Fatalf("cells = %d, want %d", len(top.Grid.Cells), len(base.Grid.Cells))
	}
	for i, c := range top.Grid.Cells {
		if got := c.Scene.Plot.Y - base.Grid.Cells[i].Scene.Plot.Y; got != 20 {
			t.Errorf("cell (%d,%d) plot Y moved %v, want 20", c.Row, c.Col, got)
		}
	}
	// The axis anchors to the first (top) cell, not the last.
	if got := top.Grid.Shared.X.Domain.Y1; got != top.Grid.Cells[0].Scene.Plot.Y {
		t.Errorf("shared x domain Y = %v, want first cell's plot top %v",
			got, top.Grid.Cells[0].Scene.Plot.Y)
	}
}
