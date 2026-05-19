package svg

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/render"
)

func TestPrismSVGRendererMimeType(t *testing.T) {
	r := New()
	if got := r.MimeType(); got != "image/svg+xml" {
		t.Errorf("MimeType = %q, want image/svg+xml", got)
	}
}

func TestPrismSVGEmptyScene(t *testing.T) {
	doc := scene.NewDoc()
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: scene.Scene{
				ID:    "empty",
				Frame: scene.Rect{W: 800, H: 600},
				Plot:  scene.Rect{X: 40, Y: 20, W: 740, H: 540},
			}},
		},
	}
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("<svg")) {
		t.Errorf("output does not start with <svg: %s", out[:50])
	}
	// Should parse as valid XML.
	dec := xml.NewDecoder(bytes.NewReader(out))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		_ = tok
	}
	// Should contain the prism-scene group.
	if !bytes.Contains(out, []byte(`class="prism-scene"`)) {
		t.Errorf("output missing prism-scene class")
	}
}

func TestPrismSVGWellFormedXML(t *testing.T) {
	doc := scene.NewDoc()
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: scene.Scene{
				ID:    "s1",
				Frame: scene.Rect{W: 800, H: 600},
				Plot:  scene.Rect{X: 40, Y: 20, W: 740, H: 540},
				Layers: []scene.SceneLayer{
					{
						ID:   "l1",
						Mark: scene.MarkRect,
						Marks: []scene.Mark{
							{Type: scene.MarkRect, ID: "m1", Rect: &scene.RectGeom{X: 10, Y: 20, W: 30, H: 40}},
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
	dec := xml.NewDecoder(bytes.NewReader(out))
	depth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	if depth != 0 {
		t.Errorf("XML unbalanced: depth %d at EOF", depth)
	}
}

func TestPrismSVGXMLSafeText(t *testing.T) {
	// Title containing < and & — must be escaped, not raw.
	doc := scene.NewDoc()
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: scene.Scene{
				ID:    "s",
				Frame: scene.Rect{W: 800, H: 600},
				Plot:  scene.Rect{X: 40, Y: 20, W: 740, H: 540},
				Title: &scene.TextElement{Content: "<script>alert('x')</script>", X: 400, Y: 20},
			}},
		},
	}
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if bytes.Contains(out, []byte("<script>")) {
		t.Errorf("raw <script> in output — escaping failed:\n%s", out)
	}
	if !bytes.Contains(out, []byte("&lt;script&gt;")) {
		t.Errorf("escaped <script> missing — expected &lt;script&gt;:\n%s", out)
	}
}

func TestPrismSVGViewBoxFromFrame(t *testing.T) {
	doc := scene.NewDoc()
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: scene.Scene{
				ID:    "s",
				Frame: scene.Rect{W: 1200, H: 400},
				Plot:  scene.Rect{X: 40, Y: 20, W: 1140, H: 340},
			}},
		},
	}
	out, err := New().Render(doc, render.RenderOpts{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `viewBox="0 0 1200 400"`) {
		t.Errorf("viewBox not derived from frame; output: %s", out[:200])
	}
}
