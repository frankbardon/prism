package svg

import (
	"bytes"
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func TestPrismSVGRenderRect(t *testing.T) {
	w := NewWriter()
	m := scene.Mark{
		Type: scene.MarkRect,
		ID:   "m1",
		Rect: &scene.RectGeom{X: 10, Y: 20, W: 30, H: 40},
	}
	renderRect(w, m)
	got := w.String()
	for _, want := range []string{`<rect`, `x="10"`, `y="20"`, `width="30"`, `height="40"`, `data-prism-id="m1"`, `class="prism-mark-bar"`} {
		if !strings.Contains(got, want) {
			t.Errorf("rect output missing %q: %s", want, got)
		}
	}
}

func TestPrismSVGRenderLineEmitsPolyline(t *testing.T) {
	w := NewWriter()
	m := scene.Mark{
		Type: scene.MarkLine,
		Line: &scene.LineGeom{
			Points: [][2]float64{{0, 100}, {50, 50}, {100, 0}},
		},
	}
	renderLine(w, m)
	got := w.String()
	if !strings.Contains(got, "<polyline") {
		t.Errorf("line output not polyline: %s", got)
	}
	if !strings.Contains(got, "0,100 50,50 100,0") {
		t.Errorf("polyline points missing: %s", got)
	}
}

func TestPrismSVGRenderPointEmitsCircle(t *testing.T) {
	w := NewWriter()
	m := scene.Mark{
		Type:  scene.MarkPoint,
		Point: &scene.PointGeom{Cx: 50, Cy: 50, R: 4},
	}
	renderPoint(w, m)
	got := w.String()
	for _, want := range []string{`<circle`, `cx="50"`, `cy="50"`, `r="4"`} {
		if !strings.Contains(got, want) {
			t.Errorf("point output missing %q: %s", want, got)
		}
	}
}

func TestPrismSVGRenderRuleEmitsLine(t *testing.T) {
	w := NewWriter()
	m := scene.Mark{
		Type: scene.MarkRule,
		Rule: &scene.RuleGeom{X1: 0, Y1: 100, X2: 500, Y2: 100},
	}
	renderRule(w, m)
	got := w.String()
	for _, want := range []string{`<line`, `x1="0"`, `y1="100"`, `x2="500"`, `y2="100"`} {
		if !strings.Contains(got, want) {
			t.Errorf("rule output missing %q: %s", want, got)
		}
	}
}

func TestPrismSVGRenderAreaEmitsPath(t *testing.T) {
	w := NewWriter()
	m := scene.Mark{
		Type: scene.MarkArea,
		Area: &scene.AreaGeom{
			Upper: [][2]float64{{0, 100}, {50, 50}, {100, 0}},
		},
	}
	renderArea(w, m)
	got := w.String()
	if !strings.Contains(got, "<path") {
		t.Errorf("area output not path: %s", got)
	}
	if !strings.Contains(got, `d="M0,100 L50,50 L100,0`) {
		t.Errorf("area d-attr missing prefix: %s", got)
	}
	if !bytes.HasSuffix(w.Bytes(), []byte("/>")) {
		t.Errorf("area not self-closed: %s", got)
	}
}

func TestPrismRenderArcEmitsPath(t *testing.T) {
	w := NewWriter()
	m := scene.Mark{
		Type: scene.MarkArc,
		ID:   "pie-0",
		Arc: &scene.ArcGeom{
			Cx: 100, Cy: 100,
			StartAngle: 0, EndAngle: 1.0,
			OuterR: 50, InnerR: 0,
		},
	}
	renderArc(w, m)
	got := w.String()
	if !strings.Contains(got, `<path`) {
		t.Errorf("arc output not path: %s", got)
	}
	if !strings.Contains(got, `class="prism-mark-arc"`) {
		t.Errorf("arc missing prism-mark-arc class: %s", got)
	}
	// Pie sector: one M, one L (to outer start), one A, one Z.
	if strings.Count(got, " A") != 1 {
		t.Errorf("pie sector should have exactly 1 A command: %s", got)
	}
}

func TestPrismRenderArcDonutHasInnerArc(t *testing.T) {
	w := NewWriter()
	m := scene.Mark{
		Type: scene.MarkArc,
		ID:   "donut-0",
		Arc: &scene.ArcGeom{
			Cx: 100, Cy: 100,
			StartAngle: 0, EndAngle: 1.0,
			OuterR: 50, InnerR: 27.5,
		},
	}
	renderArc(w, m)
	got := w.String()
	// Donut sector: two A commands (outer + inner arcs).
	if strings.Count(got, " A") != 2 {
		t.Errorf("donut sector should have exactly 2 A commands: %s", got)
	}
}

func TestPrismRenderPathEmitsPath(t *testing.T) {
	w := NewWriter()
	m := scene.Mark{
		Type: scene.MarkPath,
		ID:   "path-0",
		Path: &scene.PathGeom{D: "M 0 0 L 10 10"},
	}
	renderPath(w, m)
	got := w.String()
	for _, want := range []string{
		`<path`,
		`class="prism-mark-path"`,
		`d="M 0 0 L 10 10"`,
		`data-prism-id="path-0"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("path output missing %q: %s", want, got)
		}
	}
	if !bytes.HasSuffix(w.Bytes(), []byte("/>")) {
		t.Errorf("path not self-closed: %s", got)
	}
}

func TestPrismRenderPathEscapesQuotes(t *testing.T) {
	w := NewWriter()
	m := scene.Mark{
		Type: scene.MarkPath,
		Path: &scene.PathGeom{D: `M 0 0 L "x" 1`}, // pathological
	}
	renderPath(w, m)
	got := w.String()
	if strings.Contains(got, `"x"`) {
		t.Errorf("unescaped quote in path d-attr: %s", got)
	}
	if !strings.Contains(got, `&quot;`) {
		t.Errorf("expected &quot; entity for quote: %s", got)
	}
}

func TestPrismRenderImageEmitsImage(t *testing.T) {
	w := NewWriter()
	href := "data:image/png;base64,AAA="
	m := scene.Mark{
		Type:  scene.MarkImage,
		ID:    "image-0",
		Image: &scene.ImageGeom{X: 100, Y: 50, W: 64, H: 64, Href: href},
	}
	renderImage(w, m)
	got := w.String()
	for _, want := range []string{
		`<image`,
		`class="prism-mark-image"`,
		`x="100"`,
		`y="50"`,
		`width="64"`,
		`height="64"`,
		`href="data:image/png;base64,AAA="`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("image output missing %q: %s", want, got)
		}
	}
}
