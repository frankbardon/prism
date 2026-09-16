package svg

import (
	"regexp"
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// dAttr pulls the d="…" payload out of a rendered path element.
var dAttr = regexp.MustCompile(`\sd="([^"]*)"`)

func renderLineD(t *testing.T, g *scene.LineGeom) string {
	t.Helper()
	w := NewWriter()
	renderLine(w, scene.Mark{Type: scene.MarkLine, Line: g})
	got := w.String()
	if !strings.Contains(got, "<path") {
		t.Fatalf("curved line did not render as <path>: %s", got)
	}
	m := dAttr.FindStringSubmatch(got)
	if m == nil {
		t.Fatalf("no d= attribute in %s", got)
	}
	return m[1]
}

// curveRef pins each emitter against d3-shape's own output for the
// same five points. The expectations were derived by replaying d3's
// streaming curve state machines (Step / MonotoneX / Cardinal) and
// formatting through the same 3-decimal precision contract; they are
// the parity anchor for the Go renderer, TinyGo-via-WASM and the
// browser bundle.
var curveRefPoints = [][2]float64{{0, 100}, {25, 40}, {50, 60}, {75, 10}, {100, 80}}

func TestPrismSVGCurveMatchesD3Reference(t *testing.T) {
	cases := []struct {
		name    string
		curve   scene.CurveType
		tension float64
		want    string
	}{
		{
			name:  "step",
			curve: scene.CurveStep,
			want:  "M0,100 L12.5,100 L12.5,40 L37.5,40 L37.5,60 L62.5,60 L62.5,10 L87.5,10 L87.5,80 L100,80",
		},
		{
			name:  "step-before",
			curve: scene.CurveStepBefore,
			want:  "M0,100 L0,40 L25,40 L25,60 L50,60 L50,10 L75,10 L75,80 L100,80",
		},
		{
			name:  "step-after",
			curve: scene.CurveStepAfter,
			want:  "M0,100 L25,100 L25,40 L50,40 L50,60 L75,60 L75,10 L100,10 L100,80",
		},
		{
			name:  "monotone",
			curve: scene.CurveMonotone,
			want:  "M0,100 C8.333,70 16.667,40 25,40 C33.333,40 41.667,60 50,60 C58.333,60 66.667,10 75,10 C83.333,10 91.667,45 100,80",
		},
		{
			name:  "cardinal default tension",
			curve: scene.CurveCardinal,
			want:  "M0,100 C0,100 16.667,46.667 25,40 C33.333,33.333 41.667,65 50,60 C58.333,55 66.667,6.667 75,10 C83.333,13.333 100,80 100,80",
		},
		{
			name:    "cardinal tension 0.5",
			curve:   scene.CurveCardinal,
			tension: 0.5,
			want:    "M0,100 C0,100 20.833,43.333 25,40 C29.167,36.667 45.833,62.5 50,60 C54.167,57.5 70.833,8.333 75,10 C79.167,11.667 100,80 100,80",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderLineD(t, &scene.LineGeom{
				Points:  curveRefPoints,
				Curve:   tc.curve,
				Tension: tc.tension,
			})
			if got != tc.want {
				t.Errorf("d=\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestPrismSVGCurveShortRuns(t *testing.T) {
	two := [][2]float64{{0, 0}, {10, 20}}
	three := [][2]float64{{0, 0}, {10, 20}, {20, 5}}
	cases := []struct {
		name   string
		curve  scene.CurveType
		points [][2]float64
		want   string
	}{
		{name: "monotone two points", curve: scene.CurveMonotone, points: two, want: "M0,0 L10,20"},
		{name: "cardinal two points", curve: scene.CurveCardinal, points: two, want: "M0,0 L10,20"},
		{name: "step two points", curve: scene.CurveStep, points: two, want: "M0,0 L5,0 L5,20 L10,20"},
		{name: "monotone three points", curve: scene.CurveMonotone, points: three, want: "M0,0 C3.333,10 6.667,20 10,20 C13.333,20 16.667,12.5 20,5"},
		{name: "cardinal three points", curve: scene.CurveCardinal, points: three, want: "M0,0 C0,0 6.667,19.167 10,20 C13.333,20.833 20,5 20,5"},
		{name: "single point", curve: scene.CurveMonotone, points: [][2]float64{{7, 9}}, want: "M7,9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderLineD(t, &scene.LineGeom{Points: tc.points, Curve: tc.curve})
			if got != tc.want {
				t.Errorf("d=\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// TestPrismSVGLinearLineStaysPolyline is the byte-shape guard for the
// committed linear goldens and the testdata/cross_impl fixtures.
func TestPrismSVGLinearLineStaysPolyline(t *testing.T) {
	for _, c := range []scene.CurveType{"", scene.CurveLinear} {
		w := NewWriter()
		renderLine(w, scene.Mark{
			Type: scene.MarkLine,
			Line: &scene.LineGeom{Points: curveRefPoints, Curve: c},
		})
		got := w.String()
		if !strings.Contains(got, "<polyline") {
			t.Errorf("curve %q rendered as %s, want <polyline>", c, got)
		}
		if strings.Contains(got, "<path") {
			t.Errorf("curve %q leaked a <path>: %s", c, got)
		}
	}
}

// TestPrismSVGAreaCurveBothEdges asserts the curve is applied to the
// reversed lower edge as well as the upper one, with a straight
// connector between them and a closing Z.
func TestPrismSVGAreaCurveBothEdges(t *testing.T) {
	w := NewWriter()
	renderArea(w, scene.Mark{
		Type: scene.MarkArea,
		Area: &scene.AreaGeom{
			Upper: [][2]float64{{0, 40}, {10, 20}, {20, 30}},
			Lower: [][2]float64{{0, 50}, {10, 50}, {20, 50}},
			Curve: scene.CurveStepAfter,
		},
	})
	got := dAttr.FindStringSubmatch(w.String())
	if got == nil {
		t.Fatalf("no d= attribute: %s", w.String())
	}
	want := "M0,40 L10,40 L10,20 L20,20 L20,30" + // upper, step-after
		" L20,50" + // straight connector down onto the lower edge
		" L10,50 L10,50 L0,50 L0,50" + // reversed lower, same curve
		" Z"
	if got[1] != want {
		t.Errorf("area d=\n got %s\nwant %s", got[1], want)
	}
}

// TestPrismSVGAreaLinearUnchanged pins the historic linear byte shape
// (M + " L" run down the reversed lower edge + " Z").
func TestPrismSVGAreaLinearUnchanged(t *testing.T) {
	w := NewWriter()
	renderArea(w, scene.Mark{
		Type: scene.MarkArea,
		Area: &scene.AreaGeom{
			Upper: [][2]float64{{0, 40}, {10, 20}, {20, 30}},
			Lower: [][2]float64{{0, 50}, {10, 50}, {20, 50}},
		},
	})
	got := dAttr.FindStringSubmatch(w.String())
	if got == nil {
		t.Fatalf("no d= attribute: %s", w.String())
	}
	const want = "M0,40 L10,20 L20,30 L20,50 L10,50 L0,50 Z"
	if got[1] != want {
		t.Errorf("area d=\n got %s\nwant %s", got[1], want)
	}
}
