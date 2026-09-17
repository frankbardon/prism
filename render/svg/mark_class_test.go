package svg

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

// scene.Mark.Class overrides the geom-derived class. Two rects that
// mean different things (a progress mark's track and its value bar)
// would otherwise both render as prism-mark-bar, leaving downstream
// CSS no way to scope to one of them.
func TestPrismSVGMarkClassOverridesGeomClass(t *testing.T) {
	w := NewWriter()
	renderRect(w, scene.Mark{
		Type:  scene.MarkRect,
		ID:    "progress-track-0",
		Class: "prism-mark-progress-track",
		Rect:  &scene.RectGeom{X: 1, Y: 2, W: 3, H: 4},
	})
	got := w.String()
	if !strings.Contains(got, `class="prism-mark-progress-track"`) {
		t.Errorf("rect did not take the mark's Class: %s", got)
	}
	if strings.Contains(got, `class="prism-mark-bar"`) {
		t.Errorf("rect kept the geom-derived class alongside the override: %s", got)
	}
}

// An unset Class keeps the class the mark has always emitted — that is
// what holds every committed golden byte-identical, since no mark but
// progress sets one.
func TestPrismSVGUnsetMarkClassKeepsGeomClass(t *testing.T) {
	cases := []struct {
		name string
		mark scene.Mark
		want string
	}{
		{"rect", scene.Mark{Type: scene.MarkRect, Rect: &scene.RectGeom{W: 3, H: 4}}, "prism-mark-bar"},
		{"point", scene.Mark{Type: scene.MarkPoint, Point: &scene.PointGeom{Cx: 1, Cy: 2, R: 3}}, "prism-mark-point"},
		{"rule", scene.Mark{Type: scene.MarkRule, Rule: &scene.RuleGeom{X1: 0, Y1: 0, X2: 4, Y2: 0}}, "prism-mark-rule"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWriter()
			renderMark(w, tc.mark)
			got := w.String()
			if !strings.Contains(got, `class="`+tc.want+`"`) {
				t.Errorf("output missing class %q: %s", tc.want, got)
			}
		})
	}
}
