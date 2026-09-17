package svg

import (
	"math"
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

func fp(v float64) *float64 { return &v }

// renderMarks wraps marks in the minimal SceneDoc the renderer needs
// and returns the SVG bytes as a string.
func renderMarks(t *testing.T, marks ...scene.Mark) string {
	t.Helper()
	doc := scene.NewDoc()
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: scene.Scene{
				ID:     "s1",
				Frame:  scene.Rect{W: 800, H: 600},
				Plot:   scene.Rect{X: 40, Y: 20, W: 740, H: 540},
				Layers: []scene.SceneLayer{{ID: "l1", Marks: marks}},
			}},
		},
	}
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(out)
}

// markElement returns the single line of SVG holding the element
// whose opening tag matches prefix, so an assertion about a mark's
// attributes can't accidentally match the document's <style> block.
func markElement(t *testing.T, svg, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(svg, "\n") {
		if strings.Contains(line, prefix) {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("no element matching %q in:\n%s", prefix, svg)
	return ""
}

// TestRender_PaintOpacitiesAreIndependent covers E4-S1's precedence
// decision: fill-opacity / stroke-opacity do NOT override opacity —
// all three ride out as separate attributes and compose
// multiplicatively, matching Vega's canvas rule
// (alpha = opacity * (fillOpacity ?? 1)) and SVG's own compositing.
func TestRender_PaintOpacitiesAreIndependent(t *testing.T) {
	fill, _ := scene.ColorFromHex("#4c78a8")
	s := renderMarks(t, scene.Mark{
		Type: scene.MarkRect,
		Rect: &scene.RectGeom{X: 10, Y: 10, W: 20, H: 30},
		Style: scene.Style{
			Fill:          fill,
			Stroke:        fill,
			StrokeWidth:   1,
			Opacity:       0.5,
			FillOpacity:   fp(0.25),
			StrokeOpacity: fp(0.75),
		},
	})
	for _, want := range []string{
		`fill-opacity="0.25"`,
		`stroke-opacity="0.75"`,
		`opacity="0.5"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %s:\n%s", want, s)
		}
	}
}

// TestRender_PaintOpacityZeroSurvives asserts the pointer-typed
// fields keep an explicit 0 (a fully transparent paint), unlike
// Style.Opacity whose float64 zero has always meant "unset".
func TestRender_PaintOpacityZeroSurvives(t *testing.T) {
	fill, _ := scene.ColorFromHex("#4c78a8")
	s := renderMarks(t, scene.Mark{
		Type:  scene.MarkRect,
		Rect:  &scene.RectGeom{X: 10, Y: 10, W: 20, H: 30},
		Style: scene.Style{Fill: fill, FillOpacity: fp(0)},
	})
	if !strings.Contains(s, `fill-opacity="0"`) {
		t.Errorf("explicit zero fill-opacity was swallowed:\n%s", s)
	}
}

// TestRender_PaintOpacitiesOmittedWhenUnset keeps every pre-E4-S1
// mark byte-identical.
func TestRender_PaintOpacitiesOmittedWhenUnset(t *testing.T) {
	fill, _ := scene.ColorFromHex("#4c78a8")
	s := renderMarks(t, scene.Mark{
		Type:  scene.MarkRect,
		Rect:  &scene.RectGeom{X: 10, Y: 10, W: 20, H: 30},
		Style: scene.Style{Fill: fill},
	})
	if strings.Contains(s, "fill-opacity") || strings.Contains(s, "stroke-opacity") {
		t.Errorf("unset paint opacities emitted an attribute:\n%s", s)
	}
}

// TestRender_TextFontAttrs covers font / font_weight / font_style
// reaching a text mark's glyph.
func TestRender_TextFontAttrs(t *testing.T) {
	fill, _ := scene.ColorFromHex("#111827")
	s := renderMarks(t, scene.Mark{
		Type: scene.MarkText,
		Text: &scene.TextGeom{X: 100, Y: 200, Content: "hi", FontSize: 11},
		Style: scene.Style{
			Fill:       fill,
			FontFamily: "Inter",
			FontWeight: 700,
			FontStyle:  "italic",
		},
	})
	want := `font-size="11" font-family="Inter" font-weight="700" font-style="italic"`
	if !strings.Contains(s, want) {
		t.Errorf("text mark missing font attrs.\nwant substring: %s\ngot:\n%s", want, s)
	}
}

// TestRender_TextFontAttrsOnlyOnText asserts the font attributes stay
// off glyph-free marks even when the style carries them (a bar whose
// mark_def set `font` must not sprout a font-family attribute).
func TestRender_TextFontAttrsOnlyOnText(t *testing.T) {
	fill, _ := scene.ColorFromHex("#4c78a8")
	s := renderMarks(t, scene.Mark{
		Type:  scene.MarkRect,
		Rect:  &scene.RectGeom{X: 10, Y: 10, W: 20, H: 30},
		Style: scene.Style{Fill: fill, FontFamily: "Inter", FontWeight: 700, FontStyle: "italic"},
	})
	// Scope the assertion to the mark element — the document's <style>
	// block legitimately mentions font-family for the axis / legend
	// classes.
	el := markElement(t, s, `<rect class="prism-mark-bar"`)
	if strings.Contains(el, "font-family") || strings.Contains(el, "font-weight") || strings.Contains(el, "font-style") {
		t.Errorf("rect mark sprouted font attrs: %s", el)
	}
}

// TestRender_TextDxDy asserts dx / dy emit as SVG presentation
// attributes rather than being folded into x / y, and that they
// survive alongside the rotate-about-anchor transform (the ordering
// that reproduces Vega's translate(x,y) rotate(a) translate(dx,dy)).
func TestRender_TextDxDy(t *testing.T) {
	fill, _ := scene.ColorFromHex("#111827")
	s := renderMarks(t, scene.Mark{
		Type:  scene.MarkText,
		Text:  &scene.TextGeom{X: 100, Y: 200, Content: "hi", Dx: 4, Dy: -6.25, Angle: 45},
		Style: scene.Style{Fill: fill},
	})
	if !strings.Contains(s, `x="100" y="200" dx="4" dy="-6.25"`) {
		t.Errorf("text mark missing dx/dy attrs:\n%s", s)
	}
	if !strings.Contains(s, `transform="rotate(45 100 200)"`) {
		t.Errorf("text mark lost its rotate-about-anchor transform:\n%s", s)
	}
}

// TestPrismPaddedArcAngles drives the pad_angle inset directly.
func TestPrismPaddedArcAngles(t *testing.T) {
	cases := []struct {
		name             string
		g                scene.ArcGeom
		wantStart, wantE float64
	}{
		{
			name:      "no pad passes through",
			g:         scene.ArcGeom{StartAngle: 0, EndAngle: 1},
			wantStart: 0, wantE: 1,
		},
		{
			// Half the pad off each end, so the gap between two
			// sectors meeting at a shared boundary is the full 0.2.
			name:      "half the pad off each end",
			g:         scene.ArcGeom{StartAngle: 0, EndAngle: 1, PadAngle: 0.2},
			wantStart: 0.1, wantE: 0.9,
		},
		{
			name:      "sector narrower than the pad collapses, never inverts",
			g:         scene.ArcGeom{StartAngle: 0, EndAngle: 0.1, PadAngle: 0.5},
			wantStart: 0.05, wantE: 0.05,
		},
		{
			name:      "degenerate span is left alone",
			g:         scene.ArcGeom{StartAngle: 1, EndAngle: 1, PadAngle: 0.5},
			wantStart: 1, wantE: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotS, gotE := paddedArcAngles(&tc.g)
			if math.Abs(gotS-tc.wantStart) > 1e-12 || math.Abs(gotE-tc.wantE) > 1e-12 {
				t.Errorf("paddedArcAngles = %g,%g want %g,%g", gotS, gotE, tc.wantStart, tc.wantE)
			}
			if gotE < gotS {
				t.Errorf("inverted sector: %g > %g", gotS, gotE)
			}
		})
	}
}

// TestRender_ArcPadAngleShrinksPath asserts a padded sector's path
// differs from the unpadded one and that two adjacent padded sectors
// no longer share an endpoint.
func TestRender_ArcPadAngleShrinksPath(t *testing.T) {
	base := scene.ArcGeom{Cx: 400, Cy: 300, StartAngle: 0, EndAngle: math.Pi / 2, InnerR: 50, OuterR: 100}
	padded := base
	padded.PadAngle = 0.1

	if arcPath(&base) == arcPath(&padded) {
		t.Fatalf("pad_angle did not change the arc path: %s", arcPath(&base))
	}

	next := scene.ArcGeom{Cx: 400, Cy: 300, StartAngle: math.Pi / 2, EndAngle: math.Pi, InnerR: 50, OuterR: 100, PadAngle: 0.1}
	// The first sector's padded end and the second's padded start must
	// not coincide — that separation is the visible gap.
	_, e1 := paddedArcAngles(&padded)
	s2, _ := paddedArcAngles(&next)
	if gap := s2 - e1; math.Abs(gap-0.1) > 1e-12 {
		t.Errorf("gap between adjacent sectors = %g, want the declared pad_angle 0.1", gap)
	}
}
