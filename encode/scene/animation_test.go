package scene

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestPrismAnimationRoundTrip asserts that an Animation block on a
// Scene plus a Key on a Mark survive JSON round-trip without drift.
func TestPrismAnimationRoundTrip(t *testing.T) {
	mark := Mark{
		Type:  MarkRect,
		ID:    "m1",
		Key:   "region=west",
		Style: Style{},
		Rect:  &RectGeom{X: 10, Y: 20, W: 30, H: 40},
	}
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
						{ID: "l1", Mark: mark.Type, Marks: []Mark{mark}},
					},
					Animation: &Animation{
						DurationMs: 600,
						Easing:     "cubic_in_out",
						StaggerMs:  30,
						Enter:      "fade",
						Exit:       "fade",
					},
				},
			},
		},
	}

	first, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("first marshal: %v", err)
	}
	if !bytes.Contains(first, []byte(`"animation"`)) {
		t.Errorf("first marshal missing animation key: %s", first)
	}
	if !bytes.Contains(first, []byte(`"key":"region=west"`)) {
		t.Errorf("first marshal missing mark key: %s", first)
	}
	var roundTripped SceneDoc
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	second, err := json.Marshal(&roundTripped)
	if err != nil {
		t.Fatalf("second marshal: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("animation round-trip drift:\nfirst:  %s\nsecond: %s", first, second)
	}
}

// TestPrismAnimationOmitemptyClean asserts that a Scene with no
// Animation and Marks without Key produce JSON that contains
// neither "animation" nor "key" keys — guards against accidentally
// flipping omitempty behaviour and breaking existing goldens.
func TestPrismAnimationOmitemptyClean(t *testing.T) {
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
						{ID: "l1", Mark: MarkRect, Marks: []Mark{
							{Type: MarkRect, ID: "m1", Rect: &RectGeom{X: 1, Y: 2, W: 3, H: 4}},
						}},
					},
				},
			},
		},
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(out)
	if strings.Contains(body, `"animation"`) {
		t.Errorf("emitted animation key in zero-animation doc: %s", body)
	}
	if strings.Contains(body, `"key":`) {
		t.Errorf("emitted key field in zero-key mark: %s", body)
	}
}
