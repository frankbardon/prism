package encode_test

import (
	"fmt"
	"testing"

	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
)

// heatmapSpec renders a two-by-two heatmap whose color channel is
// quantitative — the shape that infers a gradient legend — carrying
// the supplied legend block (empty string ⇒ no block).
func heatmapSpec(legendBlock string) string {
	block := ""
	if legendBlock != "" {
		block = ", \"legend\": " + legendBlock
	}
	return fmt.Sprintf(`{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"r": "a", "c": "x", "n": 4},
    {"r": "a", "c": "y", "n": 9},
    {"r": "b", "c": "x", "n": 1},
    {"r": "b", "c": "y", "n": 6}
  ]},
  "mark": "heatmap",
  "encoding": {
    "x": {"field": "c", "type": "nominal"},
    "y": {"field": "r", "type": "nominal"},
    "color": {"field": "n", "type": "quantitative"%s}
  }
}`, block)
}

// TestPrismLegendDirectionLaysEntriesAcross pins the frame half of
// legend.direction: a horizontal legend is wider than it is tall
// (entries side by side), a vertical one the other way round, and
// vertical is what an absent key produces.
func TestPrismLegendDirectionLaysEntriesAcross(t *testing.T) {
	def := onlyLegend(t, encodeInline(t, legendSpec("")))
	vert := onlyLegend(t, encodeInline(t, legendSpec(`{"direction": "vertical"}`)))
	horiz := onlyLegend(t, encodeInline(t, legendSpec(`{"direction": "horizontal"}`)))

	if def.Frame != vert.Frame {
		t.Errorf("absent direction frame = %+v, want the vertical frame %+v", def.Frame, vert.Frame)
	}
	if def.Direction != "" {
		t.Errorf("absent direction = %q, want empty so the IR stays byte-identical", def.Direction)
	}
	if horiz.Direction != scene.LegendHorizontal {
		t.Errorf("Direction = %q, want %q", horiz.Direction, scene.LegendHorizontal)
	}
	if horiz.Frame.W <= vert.Frame.W {
		t.Errorf("horizontal W = %v, want > vertical %v", horiz.Frame.W, vert.Frame.W)
	}
	if horiz.Frame.H >= vert.Frame.H {
		t.Errorf("horizontal H = %v, want < vertical %v", horiz.Frame.H, vert.Frame.H)
	}
}

// TestPrismLegendDirectionShallowsBottomReservation is the layout
// half: laying three entries across leaves a single drawn row, so a
// bottom-oriented legend reserves a shallower band and the plot keeps
// more height. This is the property LegendBox.Size / SideExtent buy —
// the frame and the reservation move from one measurement.
func TestPrismLegendDirectionShallowsBottomReservation(t *testing.T) {
	vert := encodeInline(t, legendSpec(`{"orient": "bottom"}`))
	horiz := encodeInline(t, legendSpec(`{"orient": "bottom", "direction": "horizontal"}`))
	if horiz.Plot.H <= vert.Plot.H {
		t.Errorf("horizontal plot H = %v, want > vertical %v", horiz.Plot.H, vert.Plot.H)
	}
}

// TestPrismLegendSymbolTypeShapesSwatch covers symbol_type: naming a
// shape switches the swatch from the default solid square to a shaped
// symbol carrying the same colour, and an absent key leaves the solid
// swatch every legend drew before.
func TestPrismLegendSymbolTypeShapesSwatch(t *testing.T) {
	def := onlyLegend(t, encodeInline(t, legendSpec("")))
	if got := def.Entries[0].Swatch; got.Type != scene.SwatchSolid || got.Shape != "" {
		t.Errorf("default swatch = %+v, want a solid swatch with no shape", got)
	}

	for _, shape := range scene.PointShapes {
		t.Run(string(shape), func(t *testing.T) {
			lg := onlyLegend(t, encodeInline(t, legendSpec(
				fmt.Sprintf(`{"symbol_type": %q}`, shape))))
			sw := lg.Entries[0].Swatch
			if sw.Type != scene.SwatchSymbol {
				t.Errorf("swatch type = %q, want %q", sw.Type, scene.SwatchSymbol)
			}
			if sw.Shape != shape {
				t.Errorf("swatch shape = %q, want %q", sw.Shape, shape)
			}
			if sw.Color == nil {
				t.Error("shaped swatch lost its colour")
			}
		})
	}

	// An unrecognised name is ignored rather than stamped into the IR
	// — PRISM_SPEC_052 is what names it for the author.
	lg := onlyLegend(t, encodeInline(t, legendSpec(`{"symbol_type": "hexagon"}`)))
	if got := lg.Entries[0].Swatch; got.Type != scene.SwatchSolid {
		t.Errorf("unknown symbol_type produced %+v, want the default solid swatch", got)
	}
}

// TestPrismLegendSymbolSizeWidensBox pins that symbol_size reaches
// the IR and that a symbol wider than the 12-px default widens the
// legend box, so the label still clears the swatch.
func TestPrismLegendSymbolSizeWidensBox(t *testing.T) {
	base := onlyLegend(t, encodeInline(t, legendSpec(`{"symbol_type": "diamond"}`)))
	big := onlyLegend(t, encodeInline(t, legendSpec(`{"symbol_type": "diamond", "symbol_size": 24}`)))
	if got := big.Entries[0].Swatch.Size; got != 24 {
		t.Errorf("swatch size = %v, want 24", got)
	}
	if big.Frame.W <= base.Frame.W {
		t.Errorf("sized frame W = %v, want > default %v", big.Frame.W, base.Frame.W)
	}
	// A symbol no larger than the default swatch column leaves the box
	// exactly where it was.
	small := onlyLegend(t, encodeInline(t, legendSpec(`{"symbol_type": "diamond", "symbol_size": 8}`)))
	if small.Frame.W != base.Frame.W {
		t.Errorf("small frame W = %v, want unchanged %v", small.Frame.W, base.Frame.W)
	}
}

// TestPrismLegendKindInference pins the decision ResolveLegendKind
// makes: a quantitative channel infers a gradient bar, everything
// else infers category swatches, and legend.type overrides either.
func TestPrismLegendKindInference(t *testing.T) {
	cases := []struct {
		name  string
		build func(string) string
		block string
		want  scene.SwatchType
	}{
		{"nominal infers symbol", legendSpec, "", scene.SwatchSolid},
		{"quantitative infers gradient", heatmapSpec, "", scene.SwatchGradient},
		{"override to gradient", heatmapSpec, `{"type": "gradient"}`, scene.SwatchGradient},
		{"override to symbol", legendSpec, `{"type": "symbol"}`, scene.SwatchSolid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lg := onlyLegend(t, encodeInline(t, tc.build(tc.block)))
			if got := lg.Entries[0].Swatch.Type; got != tc.want {
				t.Errorf("swatch type = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPrismGradientLegendWired is the story's centrepiece: a
// quantitative colour channel now builds a gradient bar, registers a
// linear gradient in the scene's Defs under the id the swatch
// references, and labels its stops from the channel's domain.
func TestPrismGradientLegendWired(t *testing.T) {
	sc := encodeInline(t, heatmapSpec(""))
	lg := onlyLegend(t, sc)

	if lg.Position != scene.LegendRight {
		t.Errorf("Position = %q, want %q — a 130-px bar cornered would lie over the cells",
			lg.Position, scene.LegendRight)
	}
	id := lg.Entries[0].Swatch.GradientID
	if id == "" {
		t.Fatal("gradient swatch carries no gradient id")
	}
	if sc.Defs == nil || len(sc.Defs.Gradients) == 0 {
		t.Fatal("scene registered no gradient for the legend to reference")
	}
	g, ok := sc.Defs.Gradients[id]
	if !ok {
		t.Fatalf("Defs.Gradients has no %q (have %v)", id, sc.Defs.Gradients)
	}
	if g.Type != "linear" || len(g.Stops) < 2 {
		t.Errorf("gradient = %+v, want a linear ramp of at least two stops", g)
	}
	// Ticks run from the domain minimum to the maximum; the data is
	// 1..9.
	if len(lg.Entries[0].Ticks) == 0 {
		t.Fatal("gradient entry carries no labelled stops")
	}
	ticks := lg.Entries[0].Ticks
	if ticks[0].Label != "1" || ticks[len(ticks)-1].Label != "9" {
		t.Errorf("tick labels run %q..%q, want 1..9", ticks[0].Label, ticks[len(ticks)-1].Label)
	}
}

// TestPrismGradientLegendReservesItsBand pins the pre-layout half: a
// right-side gradient bar reserves a margin band, so the plot rect
// shrinks clear of it rather than being drawn over.
func TestPrismGradientLegendReservesItsBand(t *testing.T) {
	with := encodeInline(t, heatmapSpec(""))
	without := encodeInline(t, heatmapSpec(`{"orient": "none"}`))
	if with.Plot.W >= without.Plot.W {
		t.Errorf("plot W with legend = %v, want < %v (the band was not reserved)",
			with.Plot.W, without.Plot.W)
	}
	lg := onlyLegend(t, with)
	if lg.Frame.X < with.Plot.Right() {
		t.Errorf("legend frame X = %v, want at or beyond the plot's right edge %v",
			lg.Frame.X, with.Plot.Right())
	}
}

// TestPrismGradientLegendNonNumericFallsBack pins the viability
// guard: a channel declared quantitative over a column with no
// numeric cell has no domain to run a bar between, so it falls back
// to the symbol legend the categories can fill — and, critically,
// reserves the band that matches whichever one it built.
func TestPrismGradientLegendNonNumericFallsBack(t *testing.T) {
	body := `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [
    {"k": "a", "v": 3, "g": "north"},
    {"k": "b", "v": 5, "g": "south"},
    {"k": "c", "v": 7, "g": "east"}
  ]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "k", "type": "nominal"},
    "y": {"field": "v", "type": "quantitative"},
    "color": {"field": "g", "type": "quantitative", "legend": {"orient": "right"}}
  }
}`
	sc := encodeInline(t, body)
	lg := onlyLegend(t, sc)
	if got := lg.Entries[0].Swatch.Type; got != scene.SwatchSolid {
		t.Errorf("swatch type = %q, want %q (no numeric domain to ramp over)", got, scene.SwatchSolid)
	}
	if sc.Defs != nil && len(sc.Defs.Gradients) > 0 {
		t.Errorf("registered %d gradients for a legend that drew none", len(sc.Defs.Gradients))
	}
}

// TestPrismLegendBoxDirection drives the measurement directly, since
// LegendBox.Size is the single place a legend's pixel extent is
// computed and everything else follows from it.
func TestPrismLegendBoxDirection(t *testing.T) {
	vert := encode.LegendBox{Entries: 4, RowH: 16, MaxChars: 10}
	horiz := vert
	horiz.Direction = scene.LegendHorizontal

	vw, vh := vert.Size()
	hw, hh := horiz.Size()
	if hw != vw*4 {
		t.Errorf("horizontal W = %v, want 4 entries wide (%v)", hw, vw*4)
	}
	if hh >= vh {
		t.Errorf("horizontal H = %v, want < vertical %v", hh, vh)
	}
	// Top / bottom reservations follow: one drawn row instead of four.
	ve := vert.SideExtent(scene.LegendBottom, 0)
	he := horiz.SideExtent(scene.LegendBottom, 0)
	if he >= ve {
		t.Errorf("horizontal bottom extent = %v, want < vertical %v", he, ve)
	}
	// Left / right reservations follow the other way: the box is wider.
	if horiz.SideExtent(scene.LegendRight, 0) <= vert.SideExtent(scene.LegendRight, 0) {
		t.Error("horizontal right extent should exceed the vertical one")
	}
}
