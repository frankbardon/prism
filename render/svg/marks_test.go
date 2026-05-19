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
