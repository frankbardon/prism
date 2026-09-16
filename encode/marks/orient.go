package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// Mark orientation (E9-S1).
//
// A baseline-anchored cartesian mark — bar today, area / tick / boxplot
// / violin next, and any future gauge-shaped mark — has two axis roles
// rather than two fixed axes:
//
//   - the **category** axis carries the discrete band the mark sits in
//     and sizes the mark across its thickness, and
//   - the **measure** axis carries the continuous value and is anchored
//     at the data-zero baseline.
//
// Which physical axis plays which role is the mark's *orientation*.
// `vertical` puts the category on x and the measure on y (the historic
// and still-default shape); `horizontal` swaps them, which is the only
// way to draw a horizontal bar chart.
//
// This file is the single place that decision is made. Every mark that
// grows from a baseline should call MarkOrientation once and then read
// its geometry through CategorySlots / MeasureSpans / OrientedRect,
// rather than re-deriving "x is the band axis" inline. Callers that
// need the anchor on its own (stacking, reference rules) use
// BaselinePixel.

// Orientation is the resolved cartesian direction of a
// baseline-anchored mark: which axis carries the discrete category and
// which carries the measured value.
type Orientation string

const (
	// OrientVertical puts the category on x and the measure on y —
	// the default, and the only shape Prism drew before E9-S1.
	OrientVertical Orientation = "vertical"
	// OrientHorizontal puts the category on y and the measure on x,
	// growing bars rightward from a baseline on the left.
	OrientHorizontal Orientation = "horizontal"
	// OrientRadial is named by the spec vocabulary but implemented by
	// no mark. It is rejected — at validate (PRISM_SPEC_044) and again
	// here — rather than silently drawing a cartesian mark.
	OrientRadial Orientation = "radial"
)

// CategoryAxis returns the axis name ("x" / "y") carrying the discrete
// band for this orientation.
func (o Orientation) CategoryAxis() string {
	if o == OrientHorizontal {
		return "y"
	}
	return "x"
}

// MeasureAxis returns the axis name ("x" / "y") carrying the
// continuous, baseline-anchored value for this orientation.
func (o Orientation) MeasureAxis() string {
	if o == OrientHorizontal {
		return "x"
	}
	return "y"
}

// MarkOrientation resolves the orientation of a baseline-anchored
// mark from its mark-def and its resolved scales.
//
// The rule, in order:
//
//  1. An explicit `mark.orient` wins. "radial" is rejected (no mark
//     implements it); any other unknown value is rejected too. A
//     supported value still has to be drawable — `horizontal` needs a
//     band scale on y, `vertical` needs one on x — and says so plainly
//     when it is not.
//  2. Otherwise orientation is **inferred** from which axis carries a
//     band scale: a band on y with a continuous x implies horizontal,
//     a band on x implies vertical.
//  3. A band on both axes is ambiguous and resolves to vertical, the
//     historic default.
//  4. A band on neither axis cannot be drawn at all, and errors naming
//     both axes rather than blaming x alone.
//
// This mirrors Vega-Lite, which also infers orientation from the
// discrete axis and lets an explicit `orient` override it.
func MarkOrientation(in Inputs, markName string) (Orientation, error) {
	_, xIsBand := bandOf(in.X)
	_, yIsBand := bandOf(in.Y)

	explicit := ""
	if in.Mark != nil {
		explicit = in.Mark.Orient
	}
	switch Orientation(explicit) {
	case "":
		// fall through to inference
	case OrientVertical, OrientHorizontal:
		o := Orientation(explicit)
		if (o == OrientVertical && !xIsBand) || (o == OrientHorizontal && !yIsBand) {
			return "", prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("Mark %q declares orient %q, which needs a band scale on %s.", markName, explicit, o.CategoryAxis()),
				map[string]any{
					"Field":     "<" + o.CategoryAxis() + ">",
					"Source":    "<scale>",
					"Available": "band",
				},
			)
		}
		return o, nil
	case OrientRadial:
		return "", prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Mark %q declares orient \"radial\", which no mark implements.", markName),
			map[string]any{"Field": "<orient>", "Source": "<mark>", "Available": "vertical, horizontal"},
		)
	default:
		return "", prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Mark %q declares unknown orient %q.", markName, explicit),
			map[string]any{"Field": "<orient>", "Source": "<mark>", "Available": "vertical, horizontal"},
		)
	}

	switch {
	case yIsBand && !xIsBand:
		return OrientHorizontal, nil
	case xIsBand:
		// Band on x (with or without a band on y) keeps the historic
		// vertical reading.
		return OrientVertical, nil
	}
	return "", prismerrors.New(
		"PRISM_ENCODE_001",
		fmt.Sprintf("Mark %q requires a band scale on x (vertical) or on y (horizontal); neither axis has one.", markName),
		map[string]any{"Field": "<x|y>", "Source": "<scale>", "Available": "band"},
	)
}

// bandOf reports whether ch resolved through a band-capable scale.
func bandOf(ch Channel) (BandScaler, bool) {
	if ch.Scale == nil {
		return nil, false
	}
	b, ok := ch.Scale.(BandScaler)
	return b, ok
}

// CategorySlots returns the per-row (start, length) pair along the
// category axis: the band slot the row's category lands in, already
// normalised to a positive length. A y band scale runs bottom-to-top
// and therefore has a negative step, so the normalisation is what
// makes a horizontal bar drawable at all.
//
// It delegates to rectAxisExtent (the span-channel normaliser added in
// E9-S3) with no span bound, so both orientations and both the ranged
// and baseline-anchored paths share one implementation.
func CategorySlots(in Inputs, o Orientation) ([][2]float64, error) {
	name := o.CategoryAxis()
	ch := in.X
	if name == "y" {
		ch = in.Y
	}
	return rectAxisExtent(in, name, ch, Channel{}, true)
}

// BaselinePixel returns the measure axis's data-zero anchor in pixels:
// the pixel the mark grows from. Falls back to the plot edge the
// measure axis starts at when the scale cannot map zero (a log scale,
// say), matching the pre-E9-S1 behaviour on the y axis.
//
// Stacking (E5-S2) replaces this anchor per group; keeping it a named
// helper is what lets a stacked encoder swap the baseline without
// duplicating the orientation switch.
func BaselinePixel(in Inputs, o Orientation) float64 {
	if o == OrientHorizontal {
		if p, err := in.X.Scale.Apply(float64(0)); err == nil {
			return p
		}
		return in.Layout.X
	}
	if p, err := in.Y.Scale.Apply(float64(0)); err == nil {
		return p
	}
	return in.Layout.Bottom()
}

// MeasureSpans returns the per-row (start, length) pair along the
// measure axis, anchored at BaselinePixel. Both directions are
// normalised to a positive length, so a value below the baseline
// yields a rect that starts at the value and ends at the baseline.
func MeasureSpans(in Inputs, o Orientation) ([][2]float64, error) {
	ch := in.Y
	if o == OrientHorizontal {
		ch = in.X
	}
	vals, err := readField(in.Table, ch.Field)
	if err != nil {
		return nil, err
	}
	baseline := BaselinePixel(in, o)
	out := make([][2]float64, len(vals))
	for i := range vals {
		p, err := ch.Scale.Apply(vals[i])
		if err != nil {
			return nil, err
		}
		out[i] = spanFromBaseline(p, baseline)
	}
	return out, nil
}

// spanFromBaseline normalises a (value pixel, baseline pixel) pair into
// a (start, positive length) span. Axis-agnostic: on y a positive value
// sits above the baseline, on x it sits to the right, and both reduce
// to the same two cases.
func spanFromBaseline(value, baseline float64) [2]float64 {
	if length := baseline - value; length >= 0 {
		return [2]float64{value, length}
	}
	return [2]float64{baseline, value - baseline}
}

// OrientedRect assembles a rect from a category slot and a measure
// span, putting each on the axis this orientation assigns it.
func OrientedRect(o Orientation, category, measure [2]float64) scene.RectGeom {
	if o == OrientHorizontal {
		return scene.RectGeom{
			X: measure[0],
			Y: category[0],
			W: measure[1],
			H: category[1],
		}
	}
	return scene.RectGeom{
		X: category[0],
		Y: measure[0],
		W: category[1],
		H: measure[1],
	}
}
