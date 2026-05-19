// Package scale holds the per-type scale implementations consumed by
// the encode stage. P06 ships the 8 canonical scale types: linear,
// log, pow, sqrt, time, band, point, ordinal. Each implementation
// satisfies the Scale interface declared here.
//
// The package is leaf-y on purpose: it imports encode/scene (for the
// canonical ScaleType enum + Color) and errors, but nothing from
// encode/ itself. The dispatch lives in encode/scale.go so back-compat
// callers continue to use encode.ResolveScale.
package scale

import (
	"github.com/frankbardon/prism/encode/scene"
)

// Scale resolves a single data value into a pixel coordinate. The
// concrete impls live one-per-file in this package.
type Scale interface {
	// Apply resolves a data value to its pixel coordinate. Returns
	// PRISM_ENCODE_001 on type / category mismatches.
	Apply(value any) (float64, error)
	// Domain returns the resolved input domain. Cast on read.
	Domain() []any
	// Range returns the [min,max] pixel range the scale maps into.
	Range() [2]float64
	// Type returns the canonical scene.ScaleType for this scale.
	Type() scene.ScaleType
}

// BandSizer is the optional capability returning a band's pixel width.
// Implemented by BandScale; bar / rect mark encoders ask for it.
type BandSizer interface {
	BandWidth() float64
}
