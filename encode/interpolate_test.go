package encode

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
)

func mustColor(t *testing.T, hex string) *scene.Color {
	t.Helper()
	c, err := scene.ColorFromHex(hex)
	if err != nil {
		t.Fatalf("ColorFromHex(%q): %v", hex, err)
	}
	return c
}

func TestPrismNormalizeInterpolate(t *testing.T) {
	cases := map[string]string{
		"":     InterpolateRGB,
		"rgb":  InterpolateRGB,
		"hsl":  InterpolateHSL,
		"lab":  InterpolateLab,
		"hcl":  InterpolateRGB, // rejected by the schema; never reaches here
		"nope": InterpolateRGB,
	}
	for in, want := range cases {
		if got := NormalizeInterpolate(in); got != want {
			t.Errorf("NormalizeInterpolate(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrismInterpolateColorEndpoints(t *testing.T) {
	a := mustColor(t, "#000000")
	b := mustColor(t, "#ffffff")
	for _, space := range []string{InterpolateRGB, InterpolateHSL, InterpolateLab} {
		if got := InterpolateColor(a, b, 0, space); got.Hex() != "#000000" {
			t.Errorf("%s t=0: got %s, want #000000", space, got.Hex())
		}
		if got := InterpolateColor(a, b, 1, space); got.Hex() != "#ffffff" {
			t.Errorf("%s t=1: got %s, want #ffffff", space, got.Hex())
		}
	}
}

func TestPrismInterpolateColorRGBMidpoint(t *testing.T) {
	got := InterpolateColor(mustColor(t, "#000000"), mustColor(t, "#ffffff"), 0.5, InterpolateRGB)
	// Plain component lerp: 0 + 0.5*255 = 127.5, rounded to 128.
	if got.Hex() != "#808080" {
		t.Fatalf("rgb midpoint = %s, want #808080", got.Hex())
	}
}

func TestPrismInterpolateColorLabMidGrey(t *testing.T) {
	a := mustColor(t, "#000000")
	b := mustColor(t, "#ffffff")
	rgb := InterpolateColor(a, b, 0.5, InterpolateRGB)
	lab := InterpolateColor(a, b, 0.5, InterpolateLab)
	// The CIELAB midpoint of black and white is L* = 50, the tone
	// that reads as half-way bright. It lands at sRGB 119, not the
	// 128 a component average gives — asking for "lab" is asking for
	// exactly that perceptual re-spacing.
	if lab.Hex() != "#777777" {
		t.Fatalf("lab midpoint = %s, want #777777 (L* = 50)", lab.Hex())
	}
	if lab.R >= rgb.R {
		t.Fatalf("lab midpoint R=%d must differ from rgb midpoint R=%d", lab.R, rgb.R)
	}
}

func TestPrismInterpolateColorHSLKeepsSaturation(t *testing.T) {
	// Red to lime. In sRGB the midpoint is a muddy olive; in HSL the
	// hue sweeps through yellow at full saturation.
	got := InterpolateColor(mustColor(t, "#ff0000"), mustColor(t, "#00ff00"), 0.5, InterpolateHSL)
	if got.Hex() != "#ffff00" {
		t.Fatalf("hsl midpoint red→lime = %s, want #ffff00", got.Hex())
	}
}

func TestPrismInterpolateColorHSLGreyBorrowsHue(t *testing.T) {
	// An achromatic endpoint has no hue; blending must stay on the
	// chromatic endpoint's hue rather than sweeping the wheel.
	got := InterpolateColor(mustColor(t, "#808080"), mustColor(t, "#ff0000"), 0.5, InterpolateHSL)
	if got.R <= got.G || got.G != got.B {
		t.Fatalf("grey→red midpoint must stay on the red hue, got %s", got.Hex())
	}
}

func TestPrismInterpolateColorNilEndpoints(t *testing.T) {
	c := mustColor(t, "#123456")
	if got := InterpolateColor(nil, nil, 0.5, InterpolateLab); got != nil {
		t.Fatalf("two nil endpoints must yield nil, got %v", got)
	}
	if got := InterpolateColor(nil, c, 0.5, InterpolateLab); got.Hex() != "#123456" {
		t.Fatalf("nil start must yield the end color, got %s", got.Hex())
	}
	if got := InterpolateColor(c, nil, 0.5, InterpolateLab); got.Hex() != "#123456" {
		t.Fatalf("nil end must yield the start color, got %s", got.Hex())
	}
}

func TestPrismInterpolateColorAlphaBlends(t *testing.T) {
	a := mustColor(t, "#00000000")
	b := mustColor(t, "#000000ff")
	got := InterpolateColor(a, b, 0.5, InterpolateLab)
	if got.A < 127 || got.A > 128 {
		t.Fatalf("alpha must blend linearly whatever the space, got %d", got.A)
	}
}

func TestPrismResampleRampRGBIsIdentity(t *testing.T) {
	in := []*scene.Color{mustColor(t, "#000000"), mustColor(t, "#ffffff")}
	got := ResampleRamp(in, InterpolateRGB, InterpolatedRampStops)
	if len(got) != 2 || got[0] != in[0] || got[1] != in[1] {
		t.Fatalf("the rgb space must leave the ramp untouched, got %d stops", len(got))
	}
}

func TestPrismResampleRampLabExpandsAndPinsEndpoints(t *testing.T) {
	in := []*scene.Color{mustColor(t, "#000000"), mustColor(t, "#ffffff")}
	got := ResampleRamp(in, InterpolateLab, InterpolatedRampStops)
	if len(got) != InterpolatedRampStops {
		t.Fatalf("want %d stops, got %d", InterpolatedRampStops, len(got))
	}
	if got[0].Hex() != "#000000" {
		t.Errorf("first stop = %s, want #000000", got[0].Hex())
	}
	if got[len(got)-1].Hex() != "#ffffff" {
		t.Errorf("last stop = %s, want #ffffff", got[len(got)-1].Hex())
	}
	// Monotone lightness, and no neighbour further apart than the
	// banding threshold the stop count is chosen for.
	for i := 1; i < len(got); i++ {
		if got[i].R < got[i-1].R {
			t.Fatalf("stop %d (%s) darkens after %s", i, got[i].Hex(), got[i-1].Hex())
		}
		if int(got[i].R)-int(got[i-1].R) > 16 {
			t.Fatalf("stops %d..%d jump %d sRGB units", i-1, i, int(got[i].R)-int(got[i-1].R))
		}
	}
}

func TestPrismResampleRampDegenerateInputs(t *testing.T) {
	one := []*scene.Color{mustColor(t, "#abcdef")}
	if got := ResampleRamp(one, InterpolateLab, InterpolatedRampStops); len(got) != 1 {
		t.Fatalf("a single-stop ramp cannot be resampled, got %d stops", len(got))
	}
	if got := ResampleRamp(nil, InterpolateLab, InterpolatedRampStops); got != nil {
		t.Fatalf("a nil ramp must stay nil, got %v", got)
	}
	two := []*scene.Color{mustColor(t, "#000000"), mustColor(t, "#ffffff")}
	if got := ResampleRamp(two, InterpolateLab, 1); len(got) != 2 {
		t.Fatalf("n < 2 must leave the ramp untouched, got %d stops", len(got))
	}
}
