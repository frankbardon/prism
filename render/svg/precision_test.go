package svg

import (
	"bytes"
	"encoding/xml"
	"regexp"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

// TestPrismRenderPrecisionPinned — required by PHASE.md. Renders a
// SceneDoc with intentionally messy coordinates, parses every
// numeric attribute out of the SVG via encoding/xml, and asserts
// every value matches the precision pattern. Catches accidental
// %g / %.6f drift in the writer.
func TestPrismRenderPrecisionPinned(t *testing.T) {
	doc := scene.NewDoc()
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: scene.Scene{
				ID:    "s",
				Frame: scene.Rect{W: 800, H: 600},
				Plot:  scene.Rect{X: 40, Y: 20, W: 740, H: 540},
				Layers: []scene.SceneLayer{
					{
						ID:   "l1",
						Mark: scene.MarkRect,
						Marks: []scene.Mark{
							// Intentionally messy coords.
							{Type: scene.MarkRect, ID: "m1", Rect: &scene.RectGeom{
								X: 12.345678, Y: 67.891011, W: 123.456789, H: 234.567891,
							}},
							{Type: scene.MarkRect, ID: "m2", Rect: &scene.RectGeom{
								X: 0.0001, Y: 1.0, W: 50, H: 50,
							}},
						},
					},
				},
			}},
		},
	}
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Parse the SVG and walk every attribute looking for numeric
	// values. Any attribute whose value parses as a number must
	// match the precision pattern.
	dec := xml.NewDecoder(bytes.NewReader(out))
	numericAttr := regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	precisionOK := regexp.MustCompile(`^-?\d+(\.\d{1,3})?$`)
	numericAttrs := []string{"x", "y", "width", "height", "x1", "y1", "x2", "y2", "cx", "cy", "r", "rx"}
	contains := func(xs []string, s string) bool {
		for _, x := range xs {
			if x == s {
				return true
			}
		}
		return false
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range start.Attr {
			if !contains(numericAttrs, a.Name.Local) {
				continue
			}
			if !numericAttr.MatchString(a.Value) {
				continue
			}
			if !precisionOK.MatchString(a.Value) {
				t.Errorf("attribute %s=%q on <%s> exceeds RenderPrecision=%d", a.Name.Local, a.Value, start.Name.Local, render.RenderPrecision)
			}
		}
	}
}

func TestPrismRenderPrecisionFromConstant(t *testing.T) {
	// Sanity check that the constant + format function agree.
	if render.FormatFloat(1.0/7.0) != "0.143" {
		t.Errorf("FormatFloat(1/7) = %q, want 0.143", render.FormatFloat(1.0/7.0))
	}
}
