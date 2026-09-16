package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

func f64(v float64) *float64 { return &v }

// TestPrismEncodeArcCarriesPadAngle asserts mark_def.pad_angle reaches
// the Arc geometry untouched — the renderer, not the encoder, does the
// inset (see render/svg.paddedArcAngles), so the raw declared value is
// what must ride in the IR.
func TestPrismEncodeArcCarriesPadAngle(t *testing.T) {
	tbl := buildTable(t, map[string]any{"value": []float64{1, 1, 2}})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "value"},
		Layout: plotRect(),
		Style:  scene.Style{},
		Mark:   &spec.MarkDef{Type: "pie", PadAngle: f64(0.04)},
	}
	got, err := encodeArc(in, "pie")
	if err != nil {
		t.Fatalf("encodeArc: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len marks = %d, want 3", len(got))
	}
	for i, m := range got {
		if m.Arc.PadAngle != 0.04 {
			t.Errorf("mark[%d].Arc.PadAngle = %g, want 0.04", i, m.Arc.PadAngle)
		}
	}
}

// TestPrismEncodeArcPadAngleDefaultsZero keeps the no-pad_angle case
// byte-identical to pre-E4-S1 output.
func TestPrismEncodeArcPadAngleDefaultsZero(t *testing.T) {
	tbl := buildTable(t, map[string]any{"value": []float64{1, 1}})
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "value"},
		Layout: plotRect(),
		Style:  scene.Style{},
		Mark:   &spec.MarkDef{Type: "pie"},
	}
	got, err := encodeArc(in, "pie")
	if err != nil {
		t.Fatalf("encodeArc: %v", err)
	}
	for i, m := range got {
		if m.Arc.PadAngle != 0 {
			t.Errorf("mark[%d].Arc.PadAngle = %g, want 0", i, m.Arc.PadAngle)
		}
	}
}

// TestPrismEncodeTextDxDy asserts mark_def.dx / dy land on TextGeom
// rather than being folded into X / Y — the renderer needs them
// separate so the offset applies inside the rotated frame.
func TestPrismEncodeTextDxDy(t *testing.T) {
	tbl := buildTable(t, map[string]any{"val": []float64{1, 2}})
	plot := plotRect()
	sc := &linScale{dmin: 0, dmax: 2, rmin: plot.X, rmax: plot.Right()}
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "val", Scale: sc},
		Y:      Channel{Field: "val", Scale: sc},
		Layout: plot,
		Style:  scene.Style{},
		Mark:   &spec.MarkDef{Type: "text", Dx: f64(3), Dy: f64(-7.5)},
	}
	got, err := encodeText(in)
	if err != nil {
		t.Fatalf("encodeText: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len marks = %d, want 2", len(got))
	}
	for i, m := range got {
		if m.Text == nil {
			t.Fatalf("mark[%d] has no Text geom", i)
		}
		if m.Text.Dx != 3 || m.Text.Dy != -7.5 {
			t.Errorf("mark[%d] Dx/Dy = %g/%g, want 3/-7.5", i, m.Text.Dx, m.Text.Dy)
		}
	}
}

// TestPrismEncodeTextDxDyDefaultZero keeps the unset case at the
// pre-E4-S1 zero offset.
func TestPrismEncodeTextDxDyDefaultZero(t *testing.T) {
	tbl := buildTable(t, map[string]any{"val": []float64{1}})
	plot := plotRect()
	sc := &linScale{dmin: 0, dmax: 2, rmin: plot.X, rmax: plot.Right()}
	in := Inputs{
		Table:  tbl,
		X:      Channel{Field: "val", Scale: sc},
		Y:      Channel{Field: "val", Scale: sc},
		Layout: plot,
		Style:  scene.Style{},
		Mark:   &spec.MarkDef{Type: "text"},
	}
	got, err := encodeText(in)
	if err != nil {
		t.Fatalf("encodeText: %v", err)
	}
	if got[0].Text.Dx != 0 || got[0].Text.Dy != 0 {
		t.Errorf("Dx/Dy = %g/%g, want 0/0", got[0].Text.Dx, got[0].Text.Dy)
	}
}
