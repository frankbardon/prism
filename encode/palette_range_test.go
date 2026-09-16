package encode

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/theme"
)

// rangeTestTheme carries both a category slot and a ramp slot so a
// resolution that ignored scale.range would visibly pick these up.
func rangeTestTheme() *theme.Theme {
	return &theme.Theme{
		Range: &theme.Range{
			Category: &theme.RangeSlot{Colors: []string{"#111111", "#222222"}},
			Ramp:     &theme.RangeSlot{Colors: []string{"#333333", "#444444"}},
		},
	}
}

func hexes(t *testing.T, cs []*scene.Color) []string {
	t.Helper()
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Hex()
	}
	return out
}

func TestPrismScaleOptsFromSpecLiftsRangeAndInterpolate(t *testing.T) {
	opts := ScaleOptsFromSpec(&spec.Scale{
		Scheme:      "viridis",
		Range:       []any{"#aabbcc", "#ddeeff"},
		Interpolate: "lab",
	})
	if opts.Scheme != "viridis" {
		t.Errorf("Scheme = %q, want viridis", opts.Scheme)
	}
	if got := opts.Range; len(got) != 2 || got[0] != "#aabbcc" || got[1] != "#ddeeff" {
		t.Errorf("Range = %v, want the two inline colors", got)
	}
	if opts.Interpolate != "lab" {
		t.Errorf("Interpolate = %q, want lab", opts.Interpolate)
	}
}

func TestPrismScaleOptsIgnoresNonColorRange(t *testing.T) {
	// A numeric range (Vega-Lite's size / opacity form) and the
	// string form are both left unlifted — Prism honours neither.
	for _, raw := range []any{[]any{0.0, 200.0}, "viridis", []any{}, nil} {
		if got := ScaleOptsFromSpec(&spec.Scale{Range: raw}).Range; got != nil {
			t.Errorf("range %v lifted to %v, want nil", raw, got)
		}
	}
}

func TestPrismCategoricalRangeOutranksSchemeAndTheme(t *testing.T) {
	opts := ScaleOptsFromSpec(&spec.Scale{
		Scheme: "viridis",
		Range:  []any{"#ff0000", "#00ff00", "#0000ff"},
	})
	got := hexes(t, ResolveCategoricalPaletteWithOpts(rangeTestTheme(), opts))
	want := []string{"#ff0000", "#00ff00", "#0000ff"}
	if len(got) != len(want) {
		t.Fatalf("palette = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("palette = %v, want %v", got, want)
		}
	}
}

func TestPrismCategoricalRangeIgnoresInterpolate(t *testing.T) {
	// A categorical palette is indexed positionally, never
	// traversed, so the interpolation space must not resample it.
	opts := ScaleOptsFromSpec(&spec.Scale{
		Range:       []any{"#000000", "#ffffff"},
		Interpolate: "lab",
	})
	if got := ResolveCategoricalPaletteWithOpts(nil, opts); len(got) != 2 {
		t.Fatalf("want the 2 declared colors, got %d", len(got))
	}
}

func TestPrismSequentialRangeOutranksThemeRampSlot(t *testing.T) {
	opts := ScaleOptsFromSpec(&spec.Scale{Range: []any{"#000000", "#ffffff"}})
	got := hexes(t, ResolveSequentialPaletteWithOpts(rangeTestTheme(), opts))
	if len(got) != 2 || got[0] != "#000000" || got[1] != "#ffffff" {
		t.Fatalf("ramp = %v, want the inline range", got)
	}
}

func TestPrismSequentialRangeResampledInLab(t *testing.T) {
	opts := ScaleOptsFromSpec(&spec.Scale{
		Range:       []any{"#000000", "#ffffff"},
		Interpolate: "lab",
	})
	got := ResolveSequentialPaletteWithOpts(nil, opts)
	if len(got) != InterpolatedRampStops {
		t.Fatalf("want the ramp resampled to %d stops, got %d", InterpolatedRampStops, len(got))
	}
	mid := got[len(got)/2]
	if mid.Hex() != "#777777" {
		t.Fatalf("resampled midpoint = %s, want the CIELAB L*=50 grey #777777", mid.Hex())
	}
}

func TestPrismSequentialRGBLeavesRampAlone(t *testing.T) {
	opts := ScaleOptsFromSpec(&spec.Scale{Range: []any{"#000000", "#ffffff"}})
	if got := ResolveSequentialPaletteWithOpts(nil, opts); len(got) != 2 {
		t.Fatalf("the default rgb space must not resample; got %d stops", len(got))
	}
}

func TestPrismUnparseableRangeFallsThrough(t *testing.T) {
	opts := ScaleOptsFromSpec(&spec.Scale{Range: []any{"chartreuse", "rebeccapurple"}})
	got := hexes(t, ResolveCategoricalPaletteWithOpts(rangeTestTheme(), opts))
	if len(got) != 2 || got[0] != "#111111" {
		t.Fatalf("a wholly unparseable range must fall through to the theme slot, got %v", got)
	}
}

func TestPrismPartialRangeKeepsParseableEntries(t *testing.T) {
	opts := ScaleOptsFromSpec(&spec.Scale{Range: []any{"#ff0000", "not-a-color", "#0000ff"}})
	got := hexes(t, ResolveCategoricalPaletteWithOpts(rangeTestTheme(), opts))
	if len(got) != 2 || got[0] != "#ff0000" || got[1] != "#0000ff" {
		t.Fatalf("want the parseable entries to still win the cascade, got %v", got)
	}
}

func TestPrismLegacyPaletteWrappersUnchanged(t *testing.T) {
	th := rangeTestTheme()
	if got := hexes(t, ResolveCategoricalPalette(th, "")); len(got) != 2 || got[0] != "#111111" {
		t.Fatalf("categorical wrapper = %v, want the theme category slot", got)
	}
	if got := hexes(t, ResolveSequentialPalette(th, "")); len(got) != 2 || got[0] != "#333333" {
		t.Fatalf("sequential wrapper = %v, want the theme ramp slot", got)
	}
}

func TestPrismGradientStopsFromPalette(t *testing.T) {
	pal := []*scene.Color{
		mustColor(t, "#000000"),
		mustColor(t, "#808080"),
		mustColor(t, "#ffffff"),
	}
	stops := GradientStopsFromPalette(pal)
	if len(stops) != 3 {
		t.Fatalf("want 3 stops, got %d", len(stops))
	}
	for i, want := range []float64{0, 0.5, 1} {
		if stops[i].Offset != want {
			t.Errorf("stop %d offset = %v, want %v", i, stops[i].Offset, want)
		}
	}
	if stops[2].Color.Hex() != "#ffffff" {
		t.Errorf("last stop = %s, want #ffffff", stops[2].Color.Hex())
	}
	if GradientStopsFromPalette(nil) != nil {
		t.Error("an empty palette must yield no stops")
	}
	if got := GradientStopsFromPalette(pal[:1]); len(got) != 1 || got[0].Offset != 0 {
		t.Errorf("a single-color palette must yield one stop at offset 0, got %v", got)
	}
}
