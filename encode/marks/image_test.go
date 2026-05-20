package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

func TestPrismEncodeImagePrimitive(t *testing.T) {
	tbl := buildTable(t, map[string]any{"_": []float64{0}})
	url := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="
	in := Inputs{
		Table:  tbl,
		Layout: plotRect(),
		Mark:   &spec.MarkDef{Type: "image", URL: url},
	}
	marks, err := encodeImage(in)
	if err != nil {
		t.Fatalf("encodeImage: %v", err)
	}
	if len(marks) != 1 {
		t.Fatalf("want 1 mark, got %d", len(marks))
	}
	if marks[0].Type != scene.MarkImage {
		t.Errorf("type = %v, want MarkImage", marks[0].Type)
	}
	if marks[0].Image == nil {
		t.Fatal("Image geom nil")
	}
	if marks[0].Image.Href != url {
		t.Errorf("href = %q", marks[0].Image.Href)
	}
	if marks[0].Image.W != 64 || marks[0].Image.H != 64 {
		t.Errorf("default size W=%g H=%g, want 64x64", marks[0].Image.W, marks[0].Image.H)
	}
}

func TestPrismEncodeImageEmptyURLRejected(t *testing.T) {
	tbl := buildTable(t, map[string]any{"_": []float64{0}})
	in := Inputs{
		Table:  tbl,
		Layout: plotRect(),
		Mark:   &spec.MarkDef{Type: "image", URL: ""},
	}
	_, err := encodeImage(in)
	if err == nil {
		t.Fatal("expected error for empty url")
	}
}

func TestPrismEncodeImageSizeOverride(t *testing.T) {
	tbl := buildTable(t, map[string]any{"_": []float64{0}})
	size := 128.0
	in := Inputs{
		Table:  tbl,
		Layout: plotRect(),
		Mark:   &spec.MarkDef{Type: "image", URL: "logo.png", Size: &size},
	}
	marks, err := encodeImage(in)
	if err != nil {
		t.Fatalf("encodeImage: %v", err)
	}
	if marks[0].Image.W != 128 || marks[0].Image.H != 128 {
		t.Errorf("size override failed: W=%g H=%g", marks[0].Image.W, marks[0].Image.H)
	}
}
