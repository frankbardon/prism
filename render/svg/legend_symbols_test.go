package svg

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// symbolLegend wraps a symbol legend carrying the supplied swatches
// into a writer-level render, returning the emitted SVG.
func symbolLegend(dir scene.LegendDirection, swatches ...scene.SwatchSpec) string {
	entries := make([]scene.LegendEntry, len(swatches))
	for i, sw := range swatches {
		entries[i] = scene.LegendEntry{Label: "e", Swatch: sw}
	}
	w := NewWriter()
	renderLegend(w, scene.Legend{
		ID:        "legend-color",
		Channel:   scene.ChannelColor,
		Position:  scene.LegendTopRight,
		Direction: dir,
		Frame:     scene.Rect{X: 100, Y: 50, W: 300, H: 60},
		Entries:   entries,
	}, nil)
	return w.String()
}

// TestPrismLegendSwatchUsesPointEmitter is the anti-divergence check:
// a legend's shaped swatch and a point mark of the same shape emit
// the SAME geometry, because both go through writeSymbolGeom. A
// second emitter would drift from this within a release.
func TestPrismLegendSwatchUsesPointEmitter(t *testing.T) {
	for _, shape := range scene.PointShapes {
		t.Run(string(shape), func(t *testing.T) {
			const cx, cy, r = 20.0, 30.0, 6.0

			pw := NewWriter()
			renderPoint(pw, scene.Mark{
				Type:  scene.MarkPoint,
				Point: &scene.PointGeom{Cx: cx, Cy: cy, R: r, Shape: shape},
			})
			sw := NewWriter()
			sw.OpenTag(symbolTag(shape))
			writeSymbolGeom(sw, shape, cx, cy, r)
			sw.SelfClose()

			point, swatch := pw.String(), sw.String()
			geom := symbolGeomOf(t, swatch)
			if !strings.Contains(point, geom) {
				t.Errorf("point mark geometry does not contain the swatch geometry.\npoint:  %s\nswatch: %s", point, swatch)
			}
		})
	}
}

// symbolGeomOf strips the element wrapper off a bare symbol render,
// leaving the geometry attributes the point mark must also carry.
func symbolGeomOf(t *testing.T, s string) string {
	t.Helper()
	i := strings.Index(s, " ")
	j := strings.LastIndex(s, "/>")
	if i < 0 || j < 0 || j <= i {
		t.Fatalf("unrecognised symbol render %q", s)
	}
	return s[i+1 : j]
}

// TestPrismLegendCircleStaysACircle pins that the default (and only
// encoder-produced) shape still emits <circle cx cy r>, which is what
// keeps every committed point-mark golden byte-identical.
func TestPrismLegendCircleStaysACircle(t *testing.T) {
	if got := symbolTag(scene.ShapeCircle); got != "circle" {
		t.Errorf("symbolTag(circle) = %q, want circle", got)
	}
	if got := symbolTag(""); got != "circle" {
		t.Errorf("symbolTag(\"\") = %q, want circle", got)
	}
	for _, shape := range []scene.PointShape{scene.ShapeSquare, scene.ShapeTriangle, scene.ShapeCross, scene.ShapeDiamond} {
		if got := symbolTag(shape); got != "path" {
			t.Errorf("symbolTag(%q) = %q, want path", shape, got)
		}
	}
}

// TestPrismLegendSymbolSwatchRendered covers the SwatchSymbol arm:
// the shape is drawn, the colour lands on it, and symbol_size sets
// the radius.
func TestPrismLegendSymbolSwatchRendered(t *testing.T) {
	red, err := scene.ColorFromHex("#ff0000")
	if err != nil {
		t.Fatalf("ColorFromHex: %v", err)
	}
	got := symbolLegend("", scene.SwatchSpec{
		Type:  scene.SwatchSymbol,
		Shape: scene.ShapeDiamond,
		Color: red,
		Size:  20,
	})
	for _, want := range []string{`class="prism-legend-symbol"`, `<path`, `fill="#ff0000"`} {
		if !strings.Contains(got, want) {
			t.Errorf("symbol swatch missing %q: %s", want, got)
		}
	}
	// A 20-px diamond centred in the 12-px swatch column: centre
	// (100+4+6, 50+8+6) = (110, 64), apex 10 px above it.
	if !strings.Contains(got, `d="M110,54 L120,64 L110,74 L100,64 Z"`) {
		t.Errorf("diamond geometry not as expected: %s", got)
	}
}

// TestPrismLegendHorizontalLaysEntriesAcross pins the renderer half
// of legend.direction: entries share one row and step across by the
// frame's per-entry width instead of stepping down.
func TestPrismLegendHorizontalLaysEntriesAcross(t *testing.T) {
	three := []scene.SwatchSpec{
		{Type: scene.SwatchSolid}, {Type: scene.SwatchSolid}, {Type: scene.SwatchSolid},
	}
	vert := swatchXY(t, symbolLegend(scene.LegendVertical, three...))
	horiz := swatchXY(t, symbolLegend(scene.LegendHorizontal, three...))

	for i := 1; i < len(vert); i++ {
		if vert[i][0] != vert[0][0] || vert[i][1] <= vert[i-1][1] {
			t.Fatalf("vertical swatches did not step down a single column: %v", vert)
		}
	}
	for i := 1; i < len(horiz); i++ {
		if horiz[i][1] != horiz[0][1] || horiz[i][0] <= horiz[i-1][0] {
			t.Fatalf("horizontal swatches did not step across a single row: %v", horiz)
		}
	}
	// The default (empty) direction reproduces the vertical layout.
	if def := swatchXY(t, symbolLegend("", three...)); def[1] != vert[1] {
		t.Errorf("default direction swatch = %v, want the vertical %v", def[1], vert[1])
	}
}

// swatchXY pulls the (x, y) of each rendered swatch rect out of an
// SVG fragment, in document order.
func swatchXY(t *testing.T, svg string) [][2]string {
	t.Helper()
	var out [][2]string
	for _, chunk := range strings.Split(svg, `<rect class="prism-legend-swatch"`)[1:] {
		out = append(out, [2]string{attrOf(t, chunk, "x"), attrOf(t, chunk, "y")})
	}
	if len(out) == 0 {
		t.Fatalf("no swatches in %s", svg)
	}
	return out
}

// attrOf reads the first value of the named attribute in a fragment.
func attrOf(t *testing.T, s, name string) string {
	t.Helper()
	key := " " + name + `="`
	i := strings.Index(s, key)
	if i < 0 {
		t.Fatalf("attribute %q missing from %q", name, s)
	}
	rest := s[i+len(key):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		t.Fatalf("unterminated attribute %q in %q", name, s)
	}
	return rest[:j]
}
