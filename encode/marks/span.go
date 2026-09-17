package marks

import (
	"fmt"

	prismerrors "github.com/frankbardon/prism/errors"
)

// Span channels (E9-S3).
//
// `x2` / `y2` are the secondary position channels. They never resolve
// a scale of their own: the encoder binds each one to the *same*
// resolved Scale as its base channel (see encode.spanChannel), so a
// span is always measured in the base channel's units and lands on
// the base channel's axis. The base channel's domain is widened with
// the companion column's values before the scale is built, so an
// interval reaching past the base column's own range is never clipped.
//
// Marks that draw a span: bar and rect (both axes), rule (both axes),
// area (y2 only, as the explicit lower edge). Every other mark rejects
// a span channel at validate (PRISM_SPEC_042) rather than ignoring it.

// spanBound reports whether ch carries a usable span binding — a field
// name plus the scale its base channel resolved through. A span
// channel with no scale (polar / specialty / geo marks resolve none)
// counts as unbound.
func spanBound(ch Channel) bool { return ch.Field != "" && ch.Scale != nil }

// spanPixels resolves per-row [lo, hi] pixel intervals from a base
// channel and its `2` companion. lo is the smaller pixel and hi the
// larger, so a reversed pair (x2 < x, or any y pair — the SVG y axis
// grows downward) still yields a positive extent.
func spanPixels(in Inputs, base, upper Channel) ([][2]float64, error) {
	baseVals, err := readField(in.Table, base.Field)
	if err != nil {
		return nil, err
	}
	upperVals, err := readField(in.Table, upper.Field)
	if err != nil {
		return nil, err
	}
	if len(baseVals) != len(upperVals) {
		return nil, fmt.Errorf("spanPixels: column length mismatch (%s=%d, %s=%d)",
			base.Field, len(baseVals), upper.Field, len(upperVals))
	}
	upperScale := upper.Scale
	if upperScale == nil {
		upperScale = base.Scale
	}
	out := make([][2]float64, len(baseVals))
	for i := range baseVals {
		lo, err := base.Scale.Apply(baseVals[i])
		if err != nil {
			return nil, err
		}
		hi, err := upperScale.Apply(upperVals[i])
		if err != nil {
			return nil, err
		}
		if hi < lo {
			lo, hi = hi, lo
		}
		out[i] = [2]float64{lo, hi}
	}
	return out, nil
}

// rectAxisExtent resolves one axis of a ranged rect into per-row
// (start, length) pairs.
//
//   - A bound span channel wins: the rect runs from the lower pixel to
//     the upper one.
//   - Otherwise the base channel positions the rect in its band slot
//     and the band width sizes it. When an offset channel is bound on
//     this axis (E1-S4) the row takes the SUB-band its offset value
//     names inside that slot rather than the whole slot.
//   - With neither, bandRequired decides: a bar hard-fails (it needs a
//     categorical slot to sit in), while a rect falls back to the 1-px
//     cell it has always drawn for a fully quantitative pair.
//
// The sub-band subdivision lands here, and nowhere else, because this
// is the one place a signed band step is normalised: the sub-band's
// displacement and its width carry the parent's sign, and folding
// them in before the correction means one rule covers both
// orientations. A span channel on the same axis still wins outright —
// an offset alongside a span is incoherent and validate rejects it
// (PRISM_SPEC_066) rather than letting the two fight over one axis.
func rectAxisExtent(in Inputs, name string, base, upper Channel, bandRequired bool) ([][2]float64, error) {
	if base.Field == "" || base.Scale == nil {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("A ranged mark requires %s to be bound alongside %s2.", name, name),
			map[string]any{"Field": "<" + name + ">", "Source": "<encoding>", "Available": name},
		)
	}
	if spanBound(upper) {
		spans, err := spanPixels(in, base, upper)
		if err != nil {
			return nil, err
		}
		out := make([][2]float64, len(spans))
		for i, s := range spans {
			out[i] = [2]float64{s[0], s[1] - s[0]}
		}
		return out, nil
	}
	vals, err := readField(in.Table, base.Field)
	if err != nil {
		return nil, err
	}
	band, isBand := base.Scale.(BandScaler)
	if !isBand && bandRequired {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("A ranged bar requires a band scale on %s when %s2 is not bound.", name, name),
			map[string]any{"Field": base.Field, "Source": "<scale>", "Available": "band"},
		)
	}
	// A bound offset channel (E1-S4) turns each slot into a row of
	// sub-bands. Resolve the column and the nested scale once, outside
	// the loop; both stay nil when nothing is bound on this axis,
	// which is what leaves the arithmetic below untouched.
	var (
		offsetVals []any
		offsetBand BandScaler
	)
	if isBand && in.Offset.On(name) {
		if b, ok := in.Offset.band(); ok {
			offsetVals, err = readField(in.Table, in.Offset.Field)
			if err != nil {
				return nil, err
			}
			offsetBand = b
		}
	}
	out := make([][2]float64, len(vals))
	for i := range vals {
		p, err := base.Scale.Apply(vals[i])
		if err != nil {
			return nil, err
		}
		if isBand {
			// A y band scale runs bottom-to-top, so its step — and
			// therefore its band width — is negative and Apply returns
			// the band's *lower* pixel edge. Normalise to (top-left
			// start, positive length) so both orientations produce a
			// drawable rect.
			start, w := p, band.BandWidth()
			if offsetBand != nil && i < len(offsetVals) {
				// The nested scale's range is (0, parent BandWidth()),
				// so its Apply is a signed displacement from the
				// slot's leading edge and its BandWidth() carries the
				// parent's sign. Both ride the same correction below.
				d, derr := in.Offset.Scale.Apply(offsetVals[i])
				if derr != nil {
					return nil, derr
				}
				start, w = p+d, offsetBand.BandWidth()
			}
			if w < 0 {
				out[i] = [2]float64{start + w, -w}
				continue
			}
			out[i] = [2]float64{start, w}
			continue
		}
		// Fully quantitative and unranged: the historic 1-px cell.
		out[i] = [2]float64{p - 0.5, 1}
	}
	return out, nil
}
