package svg

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// E8-S1 — the renderer half of the per-axis theme blocks: the tokens
// that cannot ride a CSS variable (tick geometry, label gap, the
// SVG-attribute typography, the group filter) resolve through the
// per-axis block before the shared one.

// leftAxis is the y-axis counterpart of bottomAxis (axes_components_test.go).
func leftAxis() scene.Axis {
	return scene.Axis{
		ID:       "y-axis",
		Channel:  scene.ChannelY,
		Position: scene.AxisPositionLeft,
		Domain:   scene.Line{X1: 40, Y1: 20, X2: 40, Y2: 560},
		Ticks: []scene.Tick{
			{Value: 0, Pixel: 100, Label: "0"},
			{Value: 1, Pixel: 400, Label: "1"},
		},
	}
}

// The per-axis block outranks the shared one for the axis it names,
// and the other axis keeps the shared value — the per-property merge,
// exercised on geometry rather than colour.
func TestPrismSVGPerAxisMetricPrecedence(t *testing.T) {
	th := &scene.Theme{
		AxisTickSize: floatPtr(9),
		AxisX:        &scene.AxisTokens{TickSize: floatPtr(20)},
	}

	xOut := renderAxisDoc(t, bottomAxis(), th)
	if !strings.Contains(xOut, `y2="580"`) {
		t.Error("axis_x.tick_size of 20 did not win over the shared axis.tick_size of 9")
	}

	yOut := renderAxisDoc(t, leftAxis(), th)
	if !strings.Contains(yOut, `x2="31"`) {
		t.Error("the y axis did not keep the shared axis.tick_size of 9")
	}
}

// A per-axis block that states only one token leaves the others on the
// shared value — nothing is replaced wholesale.
func TestPrismSVGPerAxisMetricFallsThroughPerProperty(t *testing.T) {
	th := &scene.Theme{
		AxisTickSize:     floatPtr(9),
		AxisLabelPadding: floatPtr(10),
		AxisX:            &scene.AxisTokens{TickSize: floatPtr(20)},
	}
	out := renderAxisDoc(t, bottomAxis(), th)
	if !strings.Contains(out, `y2="580"`) {
		t.Error("axis_x.tick_size did not apply")
	}
	if !strings.Contains(out, `y="584"`) {
		t.Error("axis_x stating only tick_size dropped the shared label_padding of 10")
	}
}

// The spec's own axis block still outranks both theme levels.
func TestPrismSVGSpecBeatsPerAxisTheme(t *testing.T) {
	th := &scene.Theme{
		AxisTickSize: floatPtr(9),
		AxisX:        &scene.AxisTokens{TickSize: floatPtr(20)},
	}
	a := bottomAxis()
	a.TickSize = floatPtr(2)
	out := renderAxisDoc(t, a, th)
	if !strings.Contains(out, `y2="562"`) {
		t.Error("the spec-level axis.tick_size did not beat theme.axis_x")
	}
}

// Typography tokens emitted as SVG attributes follow the same
// per-property fall-through.
func TestPrismSVGPerAxisTypography(t *testing.T) {
	th := &scene.Theme{
		AxisLabelLetterSpacing: floatPtr(1),
		AxisY:                  &scene.AxisTokens{LabelLetterSpacing: floatPtr(4)},
	}

	if out := renderAxisDoc(t, leftAxis(), th); !strings.Contains(out, `letter-spacing="4"`) {
		t.Error("axis_y.label_letter_spacing did not reach the y tick labels")
	}
	if out := renderAxisDoc(t, bottomAxis(), th); !strings.Contains(out, `letter-spacing="1"`) {
		t.Error("the x axis did not keep the shared axis.label_letter_spacing")
	}
}

// A per-axis filter lands on that axis's own group, composing with
// (not replacing) the shared filter on the enclosing prism-axes group.
func TestPrismSVGPerAxisFilter(t *testing.T) {
	th := &scene.Theme{
		Filters:    map[string]string{"blur": "<feGaussianBlur stdDeviation=\"2\"/>"},
		AxisFilter: "blur",
		AxisX:      &scene.AxisTokens{Filter: "blur"},
	}
	out := renderAxisDoc(t, bottomAxis(), th)
	if strings.Count(out, `filter="url(#prism-filter-blur)"`) != 2 {
		t.Errorf("want the filter on both the axes group and the x-axis group, got:\n%s", out)
	}
}

// A nil / empty per-axis block leaves the output exactly as it was
// before the blocks existed.
func TestPrismSVGPerAxisAbsentIsByteIdentical(t *testing.T) {
	base := renderAxisDoc(t, bottomAxis(), &scene.Theme{AxisTickSize: floatPtr(9)})
	with := renderAxisDoc(t, bottomAxis(), &scene.Theme{AxisTickSize: floatPtr(9), AxisY: &scene.AxisTokens{TickSize: floatPtr(30)}})
	if base != with {
		t.Error("an axis_y block changed the x axis's rendered bytes")
	}
}

// The x axis owns the vertical grid lines and the y axis the
// horizontal ones, and each sits inside a group classed by its
// channel — which is what lets theme/css.go scope colour tokens per
// orientation.
func TestPrismSVGGridLinesAreScopedByChannel(t *testing.T) {
	x := bottomAxis()
	x.Grid = []scene.Line{{X1: 100, Y1: 20, X2: 100, Y2: 560}}
	out := renderAxisDoc(t, x, nil)
	gi := strings.Index(out, `class="prism-axis prism-axis-x"`)
	li := strings.Index(out, `class="prism-grid-line"`)
	if gi < 0 || li < gi {
		t.Fatalf("x grid line is not nested inside the prism-axis-x group:\n%s", out)
	}
}
