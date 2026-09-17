package svg

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// E8-S2 — `title_padding` was emitted as --prism-axis-title-padding
// from the first theme but never read: the axis title's coordinate was
// hard-coded at ±34 / 28 / 30. These tests pin the coordinate to the
// same precedence chain tick_size and label_padding use, and pin the
// built-in padding to the value that reproduces the historical
// offsets.
//
// The plot rect the helpers build is {X: 40, Y: 20, W: 740, H: 540},
// so Bottom() is 560, Right() 780 and the centres 410 / 290.

// titledBottomAxis / titledLeftAxis are the shared fixtures with a
// title, which is the only thing that makes the renderer emit one.
func titledBottomAxis() scene.Axis {
	a := bottomAxis()
	a.Title = "Category"
	return a
}

func titledLeftAxis() scene.Axis {
	a := leftAxis()
	a.Title = "Value"
	return a
}

// With nothing stated anywhere, the title lands exactly where it
// always did — the built-in 8 px on top of the per-side text
// allowance.
func TestPrismSVGAxisTitleBuiltinPaddingIsUnchanged(t *testing.T) {
	if out := renderAxisDoc(t, titledBottomAxis(), nil); !strings.Contains(out, `y="594"`) {
		t.Errorf("bottom axis title moved off its historical y=594:\n%s", out)
	}
	out := renderAxisDoc(t, titledLeftAxis(), nil)
	if !strings.Contains(out, `x="10"`) {
		t.Errorf("left axis title moved off its historical x=10:\n%s", out)
	}
	// The rotation pivot must track the same coordinate, or the
	// rotated title swings away from where it was drawn.
	if !strings.Contains(out, "rotate(-90 10 290)") {
		t.Errorf("left axis title rotation pivot did not follow the title x:\n%s", out)
	}
}

// The shared theme block moves the title.
func TestPrismSVGAxisTitleThemePadding(t *testing.T) {
	th := &scene.Theme{AxisTitlePadding: floatPtr(20)}
	if out := renderAxisDoc(t, titledBottomAxis(), th); !strings.Contains(out, `y="606"`) {
		t.Errorf("theme axis.title_padding of 20 did not move the bottom title to y=606:\n%s", out)
	}
	if out := renderAxisDoc(t, titledLeftAxis(), th); !strings.Contains(out, `x="-2"`) {
		t.Errorf("theme axis.title_padding of 20 did not move the left title to x=-2:\n%s", out)
	}
}

// The per-axis block outranks the shared one, for its axis only.
func TestPrismSVGAxisTitlePerAxisPadding(t *testing.T) {
	th := &scene.Theme{
		AxisTitlePadding: floatPtr(20),
		AxisX:            &scene.AxisTokens{TitlePadding: floatPtr(40)},
	}
	if out := renderAxisDoc(t, titledBottomAxis(), th); !strings.Contains(out, `y="626"`) {
		t.Errorf("axis_x.title_padding of 40 did not win over the shared 20:\n%s", out)
	}
	if out := renderAxisDoc(t, titledLeftAxis(), th); !strings.Contains(out, `x="-2"`) {
		t.Errorf("the y axis did not keep the shared axis.title_padding of 20:\n%s", out)
	}
}

// The spec's own `axis.title_padding` beats both theme levels.
func TestPrismSVGAxisTitleSpecBeatsTheme(t *testing.T) {
	th := &scene.Theme{
		AxisTitlePadding: floatPtr(20),
		AxisX:            &scene.AxisTokens{TitlePadding: floatPtr(40)},
	}
	a := titledBottomAxis()
	a.TitlePadding = floatPtr(0)
	if out := renderAxisDoc(t, a, th); !strings.Contains(out, `y="586"`) {
		t.Errorf("spec axis.title_padding of 0 did not beat both theme levels:\n%s", out)
	}
}

// Stating title_padding on one axis leaves the other axis — and every
// other token — exactly as it was.
func TestPrismSVGAxisTitlePaddingIsPerProperty(t *testing.T) {
	base := renderAxisDoc(t, titledBottomAxis(), &scene.Theme{AxisTitlePadding: floatPtr(8)})
	with := renderAxisDoc(t, titledBottomAxis(), &scene.Theme{
		AxisTitlePadding: floatPtr(8),
		AxisY:            &scene.AxisTokens{TitlePadding: floatPtr(40)},
	})
	if base != with {
		t.Error("axis_y.title_padding leaked into the x axis")
	}
}
