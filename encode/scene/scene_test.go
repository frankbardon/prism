package scene

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestPrismSceneDocVersionPinned guards the wire-version constant
// (and the convenience NewDoc helper that emits it).
func TestPrismSceneDocVersionPinned(t *testing.T) {
	if CurrentVersion != "1.0" {
		t.Fatalf("CurrentVersion = %q, want %q", CurrentVersion, "1.0")
	}
	doc := NewDoc()
	if doc.Version != "1.0" {
		t.Fatalf("NewDoc().Version = %q, want %q", doc.Version, "1.0")
	}
	if doc.Theme == nil {
		t.Fatal("NewDoc().Theme is nil, want Default()")
	}
}

// TestPrismSceneIRRoundTrip — required by PHASE.md. Marshalls a
// SceneDoc to JSON, unmarshals, re-marshals, and asserts byte
// equality. Covers all 5 core marks in one table-driven sweep.
func TestPrismSceneIRRoundTrip(t *testing.T) {
	rectMark := Mark{
		Type:  MarkRect,
		ID:    "m1",
		Style: Style{Fill: mustColor(t, "#3b82f6")},
		Rect:  &RectGeom{X: 10, Y: 20, W: 30, H: 40},
	}
	lineMark := Mark{
		Type: MarkLine,
		ID:   "m2",
		Line: &LineGeom{
			Points: [][2]float64{{0, 100}, {50, 50}, {100, 0}},
			Curve:  CurveLinear,
		},
	}
	areaMark := Mark{
		Type: MarkArea,
		ID:   "m3",
		Area: &AreaGeom{
			Upper: [][2]float64{{0, 100}, {50, 50}, {100, 0}},
			Curve: CurveLinear,
		},
	}
	pointMark := Mark{
		Type:  MarkPoint,
		ID:    "m4",
		Point: &PointGeom{Cx: 25, Cy: 25, R: 4, Shape: ShapeCircle},
	}
	ruleMark := Mark{
		Type: MarkRule,
		ID:   "m5",
		Rule: &RuleGeom{X1: 0, Y1: 50, X2: 100, Y2: 50},
	}

	cases := []struct {
		name string
		mark Mark
	}{
		{"rect", rectMark},
		{"line", lineMark},
		{"area", areaMark},
		{"point", pointMark},
		{"rule", ruleMark},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := buildDoc(tc.mark)
			first, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("first marshal: %v", err)
			}
			var roundTripped SceneDoc
			if err := json.Unmarshal(first, &roundTripped); err != nil {
				t.Fatalf("unmarshal: %v\nbytes: %s", err, first)
			}
			second, err := json.Marshal(&roundTripped)
			if err != nil {
				t.Fatalf("second marshal: %v", err)
			}
			if !bytes.Equal(first, second) {
				t.Fatalf("round-trip drift:\nfirst:  %s\nsecond: %s", first, second)
			}
		})
	}
}

// TestPrismMarkUnionExclusive asserts the Validate helper catches
// multiply-populated geometries (defensive guard the encoder uses).
func TestPrismMarkUnionExclusive(t *testing.T) {
	good := Mark{Type: MarkRect, Rect: &RectGeom{}}
	if err := good.Validate(); err != nil {
		t.Fatalf("good mark validate: %v", err)
	}

	twoGeoms := Mark{Type: MarkRect, Rect: &RectGeom{}, Line: &LineGeom{}}
	if err := twoGeoms.Validate(); err == nil {
		t.Fatal("expected error for mark with two non-nil geometries")
	}

	noGeom := Mark{Type: MarkRect}
	if err := noGeom.Validate(); err == nil {
		t.Fatal("expected error for mark with zero non-nil geometries")
	}

	mismatch := Mark{Type: MarkLine, Rect: &RectGeom{}}
	if err := mismatch.Validate(); err == nil {
		t.Fatal("expected error for type/geom mismatch")
	}
}

// TestPrismColorHexRoundTrip exercises the hex parser / formatter
// for the 6- and 8-digit forms.
func TestPrismColorHexRoundTrip(t *testing.T) {
	cases := []struct{ in, want string }{
		{"#3b82f6", "#3b82f6"},
		{"#ffffff", "#ffffff"},
		{"#00000080", "#00000080"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			c, err := ColorFromHex(tc.in)
			if err != nil {
				t.Fatalf("ColorFromHex(%q): %v", tc.in, err)
			}
			got := c.Hex()
			if got != tc.want {
				t.Fatalf("round-trip %q -> %q -> %q (want %q)", tc.in, c.Hex(), got, tc.want)
			}
		})
	}
}

func mustColor(t *testing.T, hex string) *Color {
	t.Helper()
	c, err := ColorFromHex(hex)
	if err != nil {
		t.Fatalf("ColorFromHex(%q): %v", hex, err)
	}
	return c
}

// buildDoc wraps one Mark in the canonical full-nest structure for
// the round-trip test (matches design/06 § Nesting).
func buildDoc(m Mark) *SceneDoc {
	doc := NewDoc()
	doc.Grid = SceneGrid{
		Layout: GridLayout{Rows: 1, Cols: 1},
		Cells: []SceneCell{
			{
				Row: 0,
				Col: 0,
				Scene: Scene{
					ID:    "s1",
					Frame: Rect{X: 0, Y: 0, W: 800, H: 600},
					Plot:  Rect{X: 40, Y: 20, W: 740, H: 540},
					Layers: []SceneLayer{
						{
							ID:    "l1",
							Mark:  m.Type,
							Marks: []Mark{m},
						},
					},
				},
			},
		},
	}
	return doc
}
