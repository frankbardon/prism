package encode_test

import (
	"context"
	"strings"
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

// hideBarSpec is the shared bar fixture with one slot per channel.
// Each slot takes either "" (key absent) or a `, "axis": null` /
// `, "legend": null` suffix, so one template covers every state of
// the tri-state.
const hideBarSpec = `{
  "data": {"values": [{"cat": "a", "val": 3}, {"cat": "b", "val": 5}, {"cat": "c", "val": 2}]},
  "mark": {"type": "bar"},
  "encoding": {
    "x": {"field": "cat", "type": "nominal"<X>},
    "y": {"field": "val", "type": "quantitative"<Y>},
    "color": {"field": "cat", "type": "nominal"<COLOR>}
  }
}`

func buildHideSpec(x, y, color string) string {
	return strings.NewReplacer("<X>", x, "<Y>", y, "<COLOR>", color).Replace(hideBarSpec)
}

func axisChannels(sc *scene.Scene) []scene.Channel {
	out := make([]scene.Channel, 0, len(sc.Axes))
	for _, a := range sc.Axes {
		out = append(out, a.Channel)
	}
	return out
}

// TestPrismEncodeAxisNullSuppressesAxis asserts the whole axis goes —
// scene.Axis carries the domain line, ticks, labels, title and grid,
// so dropping it drops all five.
func TestPrismEncodeAxisNullSuppressesAxis(t *testing.T) {
	sc := encodeInline(t, buildHideSpec(`, "axis": null`, "", ""))
	got := axisChannels(sc)
	if len(got) != 1 || got[0] != scene.ChannelY {
		t.Fatalf("axes = %v, want [y] only", got)
	}
}

// TestPrismEncodeAxisNullReleasesPadding pins the E1-S1 acceptance
// criterion: the hidden axis's side reserves nothing, so the plot
// expands into the freed space.
func TestPrismEncodeAxisNullReleasesPadding(t *testing.T) {
	base := encodeInline(t, buildHideSpec("", "", "")).Plot
	hidX := encodeInline(t, buildHideSpec(`, "axis": null`, "", "")).Plot
	hidBoth := encodeInline(t, buildHideSpec(`, "axis": null`, `, "axis": null`, "")).Plot

	if base.H != 540 || base.W != 740 || base.X != 40 {
		t.Fatalf("baseline plot = %+v, want {40,20,740,540}", base)
	}
	// The x axis sits on the bottom: hiding it returns layoutAxisReserve
	// (20px) of height, and nothing else moves.
	if hidX.H != base.H+20 {
		t.Errorf("hidden-x plot H = %v, want %v", hidX.H, base.H+20)
	}
	if hidX.X != base.X || hidX.W != base.W || hidX.Y != base.Y {
		t.Errorf("hidden-x plot = %+v, want only H to change from %+v", hidX, base)
	}
	// The y axis sits on the left: hiding it returns 20px of the left
	// inset, so the plot starts further left and grows wider.
	if hidBoth.X != base.X-20 || hidBoth.W != base.W+20 {
		t.Errorf("hidden-both plot = %+v, want X=%v W=%v", hidBoth, base.X-20, base.W+20)
	}
}

// TestPrismEncodeLegendNullSuppressesLegend covers the mark-channel
// half of the story.
func TestPrismEncodeLegendNullSuppressesLegend(t *testing.T) {
	base := encodeInline(t, buildHideSpec("", "", ""))
	if len(base.Legends) != 1 {
		t.Fatalf("baseline legends = %d, want 1", len(base.Legends))
	}
	hidden := encodeInline(t, buildHideSpec("", "", `, "legend": null`))
	if len(hidden.Legends) != 0 {
		t.Fatalf("legends = %d, want 0", len(hidden.Legends))
	}
	// Hiding the legend must not disturb the axes or the plot rect —
	// legends overlay the plot rather than reserving a side.
	if len(hidden.Axes) != 2 {
		t.Errorf("axes = %d, want 2", len(hidden.Axes))
	}
	if hidden.Plot != base.Plot {
		t.Errorf("plot = %+v, want %+v", hidden.Plot, base.Plot)
	}
}

// TestPrismEncodeAbsentAxisKeyUnchanged is the negative control: no
// axis key at all still renders both axes and the default padding.
func TestPrismEncodeAbsentAxisKeyUnchanged(t *testing.T) {
	sc := encodeInline(t, buildHideSpec("", "", ""))
	if len(sc.Axes) != 2 {
		t.Fatalf("axes = %d, want 2", len(sc.Axes))
	}
	if sc.Plot.X != 40 || sc.Plot.Y != 20 || sc.Plot.W != 740 || sc.Plot.H != 540 {
		t.Errorf("plot = %+v, want {40,20,740,540}", sc.Plot)
	}
}

// TestPrismEncodeEmptyAxisObjectStillRenders keeps `"axis": {}` — a
// configured-but-empty block — on the default-rendering side of the
// tri-state.
func TestPrismEncodeEmptyAxisObjectStillRenders(t *testing.T) {
	sc := encodeInline(t, buildHideSpec(`, "axis": {}`, "", ""))
	if len(sc.Axes) != 2 {
		t.Fatalf("axes = %d, want 2", len(sc.Axes))
	}
	if sc.Plot.H != 540 {
		t.Errorf("plot H = %v, want 540 (padding must stay reserved)", sc.Plot.H)
	}
}

// TestPrismAxisPlacementHiddenReleasesSide exercises the layout
// primitive on its own: a hidden axis claims no side, so that side
// falls back to the bare margin.
func TestPrismAxisPlacementHiddenReleasesSide(t *testing.T) {
	p := encode.DefaultAxisPlacement()
	p.XHidden = true
	got := encode.LayoutOpts{Width: 800, Height: 600, Sides: p.Sides()}.Padding()
	want := encode.Padding{Top: 20, Right: 20, Bottom: 20, Left: 40}
	if got != want {
		t.Errorf("Padding = %+v, want %+v", got, want)
	}

	p.YHidden = true
	got = encode.LayoutOpts{Width: 800, Height: 600, Sides: p.Sides()}.Padding()
	want = encode.Padding{Top: 20, Right: 20, Bottom: 20, Left: 20}
	if got != want {
		t.Errorf("both-hidden Padding = %+v, want %+v", got, want)
	}
}

// encodeInlineComposite is the composite counterpart of encodeInline:
// build → execute each child → EncodeComposite, without a fixture file.
func encodeInlineComposite(t *testing.T, body string) *scene.SceneDoc {
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
			t.Fatalf("execute child %d: %v", i, err)
		}
		if len(res.Errors) > 0 {
			t.Fatalf("child %d: %v", i, res.Errors)
		}
		per[i] = res.Tables
	}
	doc, err := encode.EncodeComposite(s, c, per, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("EncodeComposite: %v", err)
	}
	return doc
}

// layerHideSpec stacks a bar and a line layer; each layer's x channel
// takes the same slot treatment as hideBarSpec.
const layerHideSpec = `{
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

// TestPrismEncodeLayerAxisNullNeedsUnanimity pins the layered rule:
// layers share one pair of axes, so the x axis (and the padding its
// side reserves) survives unless every layer that binds x hides it.
func TestPrismEncodeLayerAxisNullNeedsUnanimity(t *testing.T) {
	const hide = `, "axis": null`
	encodeLayers := func(x0, x1 string) *scene.SceneDoc {
		return encodeInlineComposite(t, strings.NewReplacer("<X0>", x0, "<X1>", x1).Replace(layerHideSpec))
	}

	both := encodeLayers(hide, hide)
	if both.Grid.Shared.X != nil {
		t.Error("unanimous hide: shared x axis still emitted")
	}
	if got := both.Grid.Cells[0].Scene.Plot.H; got != 560 {
		t.Errorf("unanimous hide: plot H = %v, want 560 (bottom padding released)", got)
	}

	one := encodeLayers(hide, "")
	if one.Grid.Shared.X == nil {
		t.Error("split hide: shared x axis dropped; one layer still wants it")
	}
	if got := one.Grid.Cells[0].Scene.Plot.H; got != 540 {
		t.Errorf("split hide: plot H = %v, want 540 (padding retained)", got)
	}
}
