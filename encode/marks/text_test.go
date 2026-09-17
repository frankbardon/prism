package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

func contentsOf(t *testing.T, marks []scene.Mark) []string {
	t.Helper()
	out := make([]string, 0, len(marks))
	for i, m := range marks {
		if m.Type != scene.MarkText || m.Text == nil {
			t.Fatalf("marks[%d] is not a text mark: type=%s", i, m.Type)
		}
		out = append(out, m.Text.Content)
	}
	return out
}

func TestPrismEncodeTextChannelField(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"label": []string{"alpha", "beta", "gamma"},
		"score": []float64{0.4, 0.55, 0.7},
		"idx":   []float64{1, 2, 3},
	})
	plot := plotRect()
	marks, _, err := Encode("text", Inputs{
		Table:  tbl,
		X:      Channel{Field: "idx", Scale: &linScale{dmin: 1, dmax: 3, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: "score", Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}},
		Layout: plot,
		Text:   &spec.TextChannel{Field: "label", Type: "nominal"},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got := contentsOf(t, marks)
	want := []string{"alpha", "beta", "gamma"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("marks[%d].Text.Content = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPrismEncodeTextChannelFormat(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{80.04, 12.5},
		"idx":   []float64{1, 2},
	})
	plot := plotRect()
	marks, _, err := Encode("text", Inputs{
		Table:  tbl,
		X:      Channel{Field: "idx", Scale: &linScale{dmin: 1, dmax: 2, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: "score", Scale: &linScale{dmin: 0, dmax: 100, rmin: plot.Bottom(), rmax: plot.Y}},
		Layout: plot,
		Text:   &spec.TextChannel{Field: "score", Type: "quantitative", Format: ".1f"},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got := contentsOf(t, marks)
	want := []string{"80.0", "12.5"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("marks[%d].Text.Content = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPrismEncodeTextChannelValueLiteral(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.4, 0.55},
		"idx":   []float64{1, 2},
	})
	plot := plotRect()
	marks, _, err := Encode("text", Inputs{
		Table:  tbl,
		X:      Channel{Field: "idx", Scale: &linScale{dmin: 1, dmax: 2, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: "score", Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}},
		Layout: plot,
		Text:   &spec.TextChannel{Value: "n/a"},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for i, c := range contentsOf(t, marks) {
		if c != "n/a" {
			t.Errorf("marks[%d].Text.Content = %q, want %q", i, c, "n/a")
		}
	}
}

// TestPrismEncodeTextFallbackUnchanged pins the acceptance criterion
// that a text mark with NO text channel keeps rendering the y value
// verbatim — the behaviour every committed text-mark golden encodes.
func TestPrismEncodeTextFallbackUnchanged(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.4, 0.55, 0.7},
		"idx":   []float64{1, 2, 3},
	})
	plot := plotRect()
	marks, _, err := Encode("text", Inputs{
		Table:  tbl,
		X:      Channel{Field: "idx", Scale: &linScale{dmin: 1, dmax: 3, rmin: plot.X, rmax: plot.Right()}},
		Y:      Channel{Field: "score", Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}},
		Layout: plot,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := []string{"0.4", "0.55", "0.7"}
	for i, c := range contentsOf(t, marks) {
		if c != want[i] {
			t.Errorf("marks[%d].Text.Content = %q, want %q", i, c, want[i])
		}
	}
}

// TestPrismEncodeTextNoYChannel covers a label-only mark: x bound,
// y absent. The unbound axis centres the label in the plot region
// rather than failing on an empty field name.
func TestPrismEncodeTextNoYChannel(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"label": []string{"alpha", "beta"},
		"idx":   []float64{1, 2},
	})
	plot := plotRect()
	marks, _, err := Encode("text", Inputs{
		Table:  tbl,
		X:      Channel{Field: "idx", Scale: &linScale{dmin: 1, dmax: 2, rmin: plot.X, rmax: plot.Right()}},
		Layout: plot,
		Text:   &spec.TextChannel{Field: "label", Type: "nominal"},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(marks) != 2 {
		t.Fatalf("len(marks) = %d, want 2", len(marks))
	}
	for i, c := range contentsOf(t, marks) {
		if want := []string{"alpha", "beta"}[i]; c != want {
			t.Errorf("marks[%d].Text.Content = %q, want %q", i, c, want)
		}
		if marks[i].Text.Y != plot.CenterY() {
			t.Errorf("marks[%d].Text.Y = %g, want plot centre %g", i, marks[i].Text.Y, plot.CenterY())
		}
	}
}

// TestPrismEncodeTextNoXChannel is the mirror case: y bound, x absent.
func TestPrismEncodeTextNoXChannel(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"label": []string{"alpha", "beta"},
		"score": []float64{0.4, 0.8},
	})
	plot := plotRect()
	marks, _, err := Encode("text", Inputs{
		Table:  tbl,
		Y:      Channel{Field: "score", Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}},
		Layout: plot,
		Text:   &spec.TextChannel{Field: "label", Type: "nominal"},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for i := range marks {
		if marks[i].Text.X != plot.CenterX() {
			t.Errorf("marks[%d].Text.X = %g, want plot centre %g", i, marks[i].Text.X, plot.CenterX())
		}
	}
}

func TestPrismEncodeTextNoPositionBinding(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"label": []string{"alpha", "beta"},
	})
	_, _, err := Encode("text", Inputs{
		Table:  tbl,
		Layout: plotRect(),
		Text:   &spec.TextChannel{Field: "label", Type: "nominal"},
	})
	if err == nil {
		t.Fatal("expected error for text mark with neither x nor y bound, got nil")
	}
}

func TestPrismEncodeTextMissingTextField(t *testing.T) {
	tbl := buildTable(t, map[string]any{
		"score": []float64{0.4},
	})
	plot := plotRect()
	_, _, err := Encode("text", Inputs{
		Table:  tbl,
		Y:      Channel{Field: "score", Scale: &linScale{dmin: 0, dmax: 1, rmin: plot.Bottom(), rmax: plot.Y}},
		Layout: plot,
		Text:   &spec.TextChannel{Field: "nope", Type: "nominal"},
	})
	if err == nil {
		t.Fatal("expected PRISM_ENCODE_001 for a missing text field, got nil")
	}
}
