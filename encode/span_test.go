package encode

import (
	"testing"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

func spanTable(t *testing.T, rows []map[string]any) (map[plan.NodeID]*table.Table, plan.NodeID) {
	t.Helper()
	tbl, _, err := table.FromInline("gantt", rows, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	const tip = plan.NodeID("tip")
	return map[plan.NodeID]*table.Table{tip: tbl}, tip
}

func posChannel(field, ty string) *spec.PositionChannel {
	return &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: field, Type: ty}}
}

// A ranged bar's x2 values widen the x domain, so an interval reaching
// past the x column's own maximum is not clipped at the plot edge.
func TestPrismEncodeSpanWidensBaseDomain(t *testing.T) {
	tables, tip := spanTable(t, []map[string]any{
		{"task": "design", "start": 0.0, "end": 4.0},
		{"task": "build", "start": 4.0, "end": 10.0},
	})
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "bar"},
		Encoding: &spec.Encoding{
			Y:  posChannel("task", "nominal"),
			X:  posChannel("start", "quantitative"),
			X2: posChannel("end", "quantitative"),
		},
	}
	doc, err := Encode(s, tables, tip, EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	sc := doc.Grid.Cells[0].Scene
	plot := sc.Plot
	marksOut := sc.Layers[0].Marks
	if len(marksOut) != 2 {
		t.Fatalf("marks = %d, want 2", len(marksOut))
	}
	// Without the domain widening the x scale would top out at 4 (the
	// largest `start`), so the second bar would run past the plot.
	for i, m := range marksOut {
		if m.Rect == nil {
			t.Fatalf("marks[%d] has no rect", i)
		}
		if m.Rect.X+m.Rect.W > plot.Right()+1e-6 {
			t.Errorf("marks[%d] ends at %g, past the plot right edge %g", i, m.Rect.X+m.Rect.W, plot.Right())
		}
		if m.Rect.W <= 0 {
			t.Errorf("marks[%d] has non-positive width %g", i, m.Rect.W)
		}
		// The y band scale runs bottom-to-top, so its band width is
		// negative; the extent must still come out drawable.
		if m.Rect.H <= 0 {
			t.Errorf("marks[%d] has non-positive height %g", i, m.Rect.H)
		}
		if m.Rect.Y < sc.Plot.Y-1e-6 {
			t.Errorf("marks[%d] starts at y=%g, above the plot top %g", i, m.Rect.Y, sc.Plot.Y)
		}
	}
	// The whole 0..10 range is used: the two bars tile the plot width.
	total := marksOut[0].Rect.W + marksOut[1].Rect.W
	if total < plot.W-1e-6 {
		t.Errorf("bars cover %g of the %g plot width; the x2 values did not widen the domain", total, plot.W)
	}
}

// A spec with no span channel resolves exactly as before: the helpers
// contribute nothing.
func TestPrismSpanHelpersAreNoOpsWhenUnbound(t *testing.T) {
	tbl, _, err := table.FromInline("gantt", []map[string]any{{"start": 1.0}}, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	if got := spanDomainValues(nil, tbl); got != nil {
		t.Errorf("spanDomainValues(nil) = %v, want nil", got)
	}
	if got := spanDomainValues(posChannel("", "quantitative"), tbl); got != nil {
		t.Errorf("spanDomainValues(fieldless) = %v, want nil", got)
	}
	if got := spanDomainValues(posChannel("absent", "quantitative"), tbl); got != nil {
		t.Errorf("spanDomainValues(missing column) = %v, want nil", got)
	}
	if ch := spanChannel(posChannel("start", "quantitative"), nil); ch.Field != "" {
		t.Errorf("spanChannel with no base scale = %+v, want the zero Channel", ch)
	}
}

// y2 on an area reaches the encoder as an explicit lower edge.
func TestPrismEncodeAreaSpanEndToEnd(t *testing.T) {
	tables, tip := spanTable(t, []map[string]any{
		{"t": 0.0, "lo": 2.0, "hi": 6.0},
		{"t": 1.0, "lo": 3.0, "hi": 7.0},
	})
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   &spec.Mark{Shorthand: "area"},
		Encoding: &spec.Encoding{
			X:  posChannel("t", "quantitative"),
			Y:  posChannel("hi", "quantitative"),
			Y2: posChannel("lo", "quantitative"),
		},
	}
	doc, err := Encode(s, tables, tip, EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	area := doc.Grid.Cells[0].Scene.Layers[0].Marks[0].Area
	if area == nil || len(area.Lower) != 2 {
		t.Fatalf("area lower edge = %+v, want 2 points", area)
	}
	bottom := doc.Grid.Cells[0].Scene.Plot.Bottom()
	for i, p := range area.Lower {
		if p[1] >= bottom {
			t.Errorf("Lower[%d].y = %g, want above the plot bottom %g (the baseline was not replaced)", i, p[1], bottom)
		}
	}
	if area.Lower[0][1] == area.Lower[1][1] {
		t.Error("lower edge is flat; y2 was not read per row")
	}
}
