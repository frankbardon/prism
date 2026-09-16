package svg

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// E3-S2 — the renderer half of the seven axis fields: component
// suppression, the tick-size / label-padding precedence chain, and the
// zindex split around the mark container.

func floatPtr(v float64) *float64 { return &v }

// axisDoc wraps one axis in a minimal single-cell SceneDoc carrying a
// mark, so the axes can be located relative to the plot group.
func axisDoc(a scene.Axis) *scene.SceneDoc {
	doc := scene.NewDoc()
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: scene.Scene{
				ID:    "s1",
				Frame: scene.Rect{W: 800, H: 600},
				Plot:  scene.Rect{X: 40, Y: 20, W: 740, H: 540},
				Axes:  []scene.Axis{a},
				Layers: []scene.SceneLayer{{
					ID:   "l1",
					Mark: scene.MarkRect,
					Marks: []scene.Mark{
						{Type: scene.MarkRect, ID: "m1", Rect: &scene.RectGeom{X: 10, Y: 20, W: 30, H: 40}},
					},
				}},
			}},
		},
	}
	return doc
}

// bottomAxis is a two-tick bottom axis with everything drawn.
func bottomAxis() scene.Axis {
	return scene.Axis{
		ID:       "x-axis",
		Channel:  scene.ChannelX,
		Position: scene.AxisPositionBottom,
		Domain:   scene.Line{X1: 40, Y1: 560, X2: 780, Y2: 560},
		Ticks: []scene.Tick{
			{Value: "a", Pixel: 100, Label: "a"},
			{Value: "b", Pixel: 400, Label: "b"},
		},
	}
}

func renderAxisDoc(t *testing.T, a scene.Axis, theme *scene.Theme) string {
	t.Helper()
	doc := axisDoc(a)
	doc.Theme = theme
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(out)
}

func TestPrismSVGAxisComponentSuppression(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*scene.Axis)
		absent  string
		present []string
	}{
		{"labels", func(a *scene.Axis) { a.HideLabels = true },
			`class="prism-axis-label"`, []string{`class="prism-axis-tick"`, `class="prism-axis-domain"`}},
		{"ticks", func(a *scene.Axis) { a.HideTicks = true },
			`class="prism-axis-tick"`, []string{`class="prism-axis-label"`, `class="prism-axis-domain"`}},
		{"domain", func(a *scene.Axis) { a.HideDomain = true },
			`class="prism-axis-domain"`, []string{`class="prism-axis-tick"`, `class="prism-axis-label"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := bottomAxis()
			tc.mutate(&a)
			out := renderAxisDoc(t, a, nil)
			if strings.Contains(out, tc.absent) {
				t.Errorf("output still carries %s", tc.absent)
			}
			for _, want := range tc.present {
				if !strings.Contains(out, want) {
					t.Errorf("output lost %s, which this switch must not touch", want)
				}
			}
		})
	}
}

// Nothing suppressed reproduces the historical 5 px tick and 18 px
// label offset, so no committed golden moves.
func TestPrismSVGAxisDefaultMetricsUnchanged(t *testing.T) {
	out := renderAxisDoc(t, bottomAxis(), nil)
	if !strings.Contains(out, `y2="565"`) {
		t.Error("default major tick no longer ends 5px below the plot")
	}
	if !strings.Contains(out, `y="578"`) {
		t.Error("default tick label no longer sits 18px below the plot")
	}
}

// Precedence: the axis's own value wins, the theme token fills in
// where the axis is silent, and the built-in metric is the floor.
func TestPrismSVGAxisMetricPrecedence(t *testing.T) {
	themed := &scene.Theme{AxisTickSize: floatPtr(9), AxisLabelPadding: floatPtr(10)}

	t.Run("theme applies when the spec is silent", func(t *testing.T) {
		out := renderAxisDoc(t, bottomAxis(), themed)
		if !strings.Contains(out, `y2="569"`) {
			t.Error("theme tick_size of 9 did not lengthen the tick mark")
		}
		if !strings.Contains(out, `y="584"`) {
			t.Error("theme label_padding of 10 did not move the label")
		}
	})

	t.Run("spec beats theme", func(t *testing.T) {
		a := bottomAxis()
		a.TickSize = floatPtr(20)
		a.LabelPadding = floatPtr(0)
		out := renderAxisDoc(t, a, themed)
		if !strings.Contains(out, `y2="580"`) {
			t.Error("spec tick_size of 20 did not win over the theme's 9")
		}
		if !strings.Contains(out, `y="574"`) {
			t.Error("spec label_padding of 0 did not win over the theme's 10")
		}
	})
}

// Minor ticks stay proportionally shorter than majors at any size.
func TestPrismSVGAxisMinorTickScalesWithTickSize(t *testing.T) {
	a := bottomAxis()
	a.TickSize = floatPtr(10)
	a.Ticks = append(a.Ticks, scene.Tick{Value: 1.5, Pixel: 250, Minor: true})
	out := renderAxisDoc(t, a, nil)
	if !strings.Contains(out, `y2="570"`) {
		t.Error("major tick did not honour tick_size 10")
	}
	if !strings.Contains(out, `y2="566"`) {
		t.Error("minor tick did not scale to 0.6 of tick_size 10")
	}
}

// zindex 0 (the default) keeps the axes group before the mark
// container; any positive value moves it after, where no plot-region
// clip path can reach it.
func TestPrismSVGAxisZindexOrdersAroundTheMarks(t *testing.T) {
	order := func(t *testing.T, z int) (axesAt, plotAt int) {
		t.Helper()
		a := bottomAxis()
		a.Zindex = z
		out := renderAxisDoc(t, a, nil)
		axesAt = strings.Index(out, `class="prism-axes"`)
		plotAt = strings.Index(out, `class="prism-plot"`)
		if axesAt < 0 || plotAt < 0 {
			t.Fatalf("missing groups: axes=%d plot=%d", axesAt, plotAt)
		}
		return axesAt, plotAt
	}

	if axesAt, plotAt := order(t, 0); axesAt > plotAt {
		t.Error("default zindex drew the axes after the marks")
	}
	if axesAt, plotAt := order(t, 1); axesAt < plotAt {
		t.Error("zindex 1 drew the axes before the marks")
	}
}

// The above-marks group is a sibling of prism-plot, never nested
// inside it, so E2-S2's plot-region clip path cannot clip it.
func TestPrismSVGAboveMarksAxisIsOutsideThePlotGroup(t *testing.T) {
	a := bottomAxis()
	a.Zindex = 1
	out := renderAxisDoc(t, a, nil)
	plotAt := strings.Index(out, `class="prism-plot"`)
	axesAt := strings.Index(out, `class="prism-axes"`)
	plotEnd := strings.Index(out[plotAt:], "</g>")
	if plotEnd < 0 {
		t.Fatal("plot group never closes")
	}
	if axesAt < plotAt+plotEnd {
		t.Error("above-marks axes group is nested inside prism-plot; a clip path would clip it")
	}
}

// A scene mixing zindexes emits both halves, each holding only its own
// axes.
func TestPrismSVGAxisZindexSplitsMixedScene(t *testing.T) {
	below := bottomAxis()
	above := scene.Axis{
		ID:       "y-axis",
		Channel:  scene.ChannelY,
		Position: scene.AxisPositionLeft,
		Zindex:   1,
		Domain:   scene.Line{X1: 40, Y1: 20, X2: 40, Y2: 560},
		Ticks:    []scene.Tick{{Value: 0.0, Pixel: 560, Label: "0"}},
	}
	doc := axisDoc(below)
	doc.Grid.Cells[0].Scene.Axes = append(doc.Grid.Cells[0].Scene.Axes, above)
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n := strings.Count(string(out), `class="prism-axes"`); n != 2 {
		t.Fatalf("prism-axes groups = %d, want 2 (one per side of the marks)", n)
	}
	first := string(out)[:strings.Index(string(out), `class="prism-plot"`)]
	if !strings.Contains(first, "prism-axis-x") {
		t.Error("below-marks group is missing the zindex-0 x axis")
	}
	if strings.Contains(first, "prism-axis-y") {
		t.Error("the zindex-1 y axis leaked into the below-marks group")
	}
}
