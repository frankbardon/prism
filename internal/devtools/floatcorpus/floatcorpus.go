// Package floatcorpus holds a single representative corpus of float64
// values plus the deterministic render.FormatFloat rendering of them.
//
// It exists so the exact same inputs and the exact same formatting code
// path run in two places during the TinyGo float-parity check:
//
//  1. the host Go test (native amd64/arm64 build), and
//  2. the TinyGo js/wasm probe.
//
// (TinyGo is now the only wasm build; the standard-Go js/wasm probe that
// once formed a third comparison point was retired with that target.)
//
// render/precision.go's FormatFloat is the one funnel every SVG
// coordinate flows through, and TinyGo ships its own strconv. If TinyGo
// rounded or stringified floats differently from the host Go build, the
// SVG coordinate goldens would drift. Pinning the corpus here and
// comparing Format() across both builds proves they agree byte-for-byte.
package floatcorpus

import (
	"math"
	"strings"

	"github.com/frankbardon/prism/render"
)

// Values returns the representative float64 corpus. It intentionally
// stresses the code paths render.FormatFloat can diverge on across
// toolchains: half-way rounding at the 3rd decimal, trailing-zero
// trimming, sign handling (including negative zero), magnitude
// extremes, binary-fraction artefacts, and the non-finite guards.
func Values() []float64 {
	return []float64{
		0,
		math.Copysign(0, -1), // negative zero
		1,
		-1,
		0.5,
		-0.5,
		1.5,
		2.5,
		1.23,
		1.230,
		1.2345,
		1.2355,
		-12.3456,
		0.001,
		0.0005,
		0.0004,
		0.0006,
		0.00049999999,
		0.0001,
		100,
		100.0,
		1234567.891,
		-1234567.891,
		0.30000000000000004, // 0.1 + 0.2
		0.125,
		0.375,
		2.6755,
		3.14159265358979,
		2.71828182845905,
		1e-10,
		1e10,
		1e-3,
		1e3,
		9999.9995,
		9999.9994,
		-0.0009,
		42.0,
		math.NaN(),
		math.Inf(1),
		math.Inf(-1),
	}
}

// Format renders each corpus value through render.FormatFloat and joins
// the results with newlines. This is the exact string compared across
// the host and TinyGo wasm builds.
func Format() string {
	vs := Values()
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = render.FormatFloat(v)
	}
	return strings.Join(parts, "\n")
}
