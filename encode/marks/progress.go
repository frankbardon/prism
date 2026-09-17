package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// The progress mark (E10-S1) draws one metric row per table row: a
// value bar sitting on a full-scale track, where the visible remainder
// of the track reads as "distance still to go".
//
// It is the multi-row sibling of bullet. bullet collapses its measure
// to row 0 — a single KPI readout — so a four-metric layout needs a
// facet wrapper and inherits facet's hardcoded "<field> = <value>" row
// labels. progress reads every row, and the row labels are simply the
// category axis's tick labels.
//
// Two things make it more than a bar with a background rect:
//
//   - The measure domain is **mark-owned**. mark.total names the value
//     the track runs to, and encode widens the measure channel's domain
//     with it before the scale is built (progressMeasureExtras), so the
//     track ends at the plot edge instead of running past it.
//   - The track is a **separate, named scene mark** ("progress-track-N")
//     carrying its own Style (Inputs.TrackStyle), not a backdrop fused
//     into the bar's geometry, so it can be themed independently — via
//     theme.Marks["progress_track"], resolved by
//     encode.progressTrackStyle. The two halves also carry distinct CSS
//     classes (ProgressClass / ProgressTrackClass) so downstream CSS
//     can scope to one of them; both are scene.MarkRect, which the
//     renderer would otherwise class identically as prism-mark-bar.
//
// Orientation comes from the shared primitive in orient.go —
// MarkOrientation plus CategorySlots / BaselinePixel / MeasureSpans /
// OrientedRect — exactly as bar does. progress does NOT carry a
// per-mark orientation field: the canonical metric-row spec binds a
// nominal y against a quantitative x, which MarkOrientation already
// infers as horizontal, and `mark.orient` overrides that inference.

// ProgressClass / ProgressTrackClass are the CSS classes the two
// halves of a progress row carry (scene.Mark.Class). They are what
// makes theme.Marks["progress"] and theme.Marks["progress_track"]
// addressable from a stylesheet as well as from theme JSON, and they
// mirror those key names on purpose: a reader who has seen one knows
// the other. Without them both rects would render as prism-mark-bar.
const (
	ProgressClass      = "prism-mark-progress"
	ProgressTrackClass = "prism-mark-progress-track"
)

// encodeProgress emits 2N scene marks for an N-row table: N track
// rects first (so they paint behind), then N value bars.
func encodeProgress(in Inputs) ([]scene.Mark, error) {
	orient, err := MarkOrientation(in, "progress")
	if err != nil {
		return nil, err
	}
	measureCh := in.Y
	if orient == OrientHorizontal {
		measureCh = in.X
	}
	slots, err := CategorySlots(in, orient)
	if err != nil {
		return nil, err
	}
	bars, err := MeasureSpans(in, orient)
	if err != nil {
		return nil, err
	}
	if len(slots) != len(bars) {
		return nil, fmt.Errorf("encodeProgress: column length mismatch (%s=%d, %s=%d)",
			orient.CategoryAxis(), len(slots), orient.MeasureAxis(), len(bars))
	}
	totals, err := ProgressTotals(in.Mark, in.Table, measureCh.Scale, len(slots))
	if err != nil {
		return nil, err
	}

	var colorVals []any
	if in.Color != nil && in.Color.Field != "" {
		cv, cerr := readField(in.Table, in.Color.Field)
		if cerr != nil {
			return nil, cerr
		}
		colorVals = cv
	}

	thickness := progressThickness(in.Mark)
	cornerR := 0.0
	if in.Mark != nil && in.Mark.CornerRadius != nil {
		cornerR = *in.Mark.CornerRadius
	}
	baseline := BaselinePixel(in, orient)

	tracks := make([]scene.Mark, 0, len(slots))
	values := make([]scene.Mark, 0, len(slots))
	for i := range slots {
		slot := progressSlot(slots[i], thickness)

		totalPix, terr := measureCh.Scale.Apply(totals[i])
		if terr != nil {
			return nil, terr
		}
		track := OrientedRect(orient, slot, spanFromBaseline(totalPix, baseline))
		track.CornerR = cornerR
		tracks = append(tracks, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("progress-track-%d", i),
			Class: ProgressTrackClass,
			Style: in.TrackStyle,
			Rect:  &track,
		})

		style := in.Style
		if i < len(colorVals) {
			if cat, ok := colorVals[i].(string); ok {
				if c, v := resolveCategoryColor(in, cat); c != nil || v != "" {
					style.Fill = c
					style.FillVar = v
				}
			}
		}
		bar := OrientedRect(orient, slot, bars[i])
		bar.CornerR = cornerR
		values = append(values, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("progress-%d", i),
			Class: ProgressClass,
			Style: style,
			Rect:  &bar,
		})
	}

	// The dispatcher's post-encode per-row passes (datum / key /
	// tooltip) stamp the *leading* len(table) marks, which here are the
	// tracks. Run the same passes over the value bars so both halves of
	// a row carry the same back-references and a hover on the filled
	// part of a row behaves like a hover on its remainder.
	stampProgressRowRefs(in, values)

	return append(tracks, values...), nil
}

// progressThickness returns the band fraction a progress row occupies.
// Out-of-range values are rejected at validate (PRISM_SPEC_061); the
// clamp here is the encoder's own guard so a spec that reached this
// point by another path still draws something.
func progressThickness(def *spec.MarkDef) float64 {
	if def == nil || def.Thickness == nil {
		return spec.ProgressThicknessDefault
	}
	t := *def.Thickness
	if t <= 0 || t > 1 {
		return spec.ProgressThicknessDefault
	}
	return t
}

// progressSlot narrows a full category band slot to the mark's
// thickness, centred in the band.
func progressSlot(slot [2]float64, thickness float64) [2]float64 {
	length := slot[1] * thickness
	return [2]float64{slot[0] + (slot[1]-length)/2, length}
}

// ProgressTotals resolves the per-row measure ceiling a progress
// mark's track runs to.
//
//   - A literal number on mark.total is the ceiling for every row.
//   - A string names a data field, read **per row** — each metric may
//     carry its own maximum.
//   - Unset falls back to the measure scale's own domain maximum, so
//     the track spans the whole axis.
//
// Exported because encode resolves the same values a second time one
// stage earlier, to widen the measure channel's domain before the
// scale exists (see encode.progressMeasureExtras).
func ProgressTotals(def *spec.MarkDef, tbl *table.Table, measure Scale, n int) ([]float64, error) {
	fill := func(v float64) []float64 {
		out := make([]float64, n)
		for i := range out {
			out[i] = v
		}
		return out
	}
	var raw any
	if def != nil {
		raw = def.Total
	}
	switch t := raw.(type) {
	case nil:
		return fill(progressDomainMax(measure)), nil
	case string:
		if t == "" {
			return fill(progressDomainMax(measure)), nil
		}
		vals, err := readField(tbl, t)
		if err != nil {
			return nil, err
		}
		out := make([]float64, n)
		for i := range out {
			if i >= len(vals) {
				break
			}
			v, ok := toFloat64(vals[i])
			if !ok {
				return nil, prismerrors.New(
					"PRISM_ENCODE_001",
					fmt.Sprintf("progress total field %q holds a non-numeric value at row %d (got %T).", t, i, vals[i]),
					map[string]any{"Field": t, "Source": "<table>", "Available": "numeric"},
				)
			}
			out[i] = v
		}
		return out, nil
	default:
		v, ok := toFloat64(t)
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("progress total must be a number or field name (got %T).", raw),
				map[string]any{"Field": "<total>", "Source": "<mark>", "Available": "number|field"},
			)
		}
		return fill(v), nil
	}
}

// progressDomainMax reads the upper bound of a resolved measure
// scale's domain. Returns 0 when the scale reports no usable numeric
// bound, which collapses the track onto the baseline rather than
// drawing it somewhere arbitrary.
func progressDomainMax(measure Scale) float64 {
	if measure == nil {
		return 0
	}
	dom := measure.Domain()
	if len(dom) == 0 {
		return 0
	}
	v, ok := toFloat64(dom[len(dom)-1])
	if !ok {
		return 0
	}
	return v
}

// stampProgressRowRefs mirrors the dispatcher's per-row passes onto a
// trailing block of marks the leading-prefix rule would otherwise skip.
func stampProgressRowRefs(in Inputs, bars []scene.Mark) {
	if len(bars) == 0 || in.Table == nil {
		return
	}
	AttachDatum(bars, in.LayerID, len(bars))
	if in.KeyField != "" {
		AttachKeys(bars, in.Table, in.KeyField)
	}
	if in.Tooltip != nil {
		AttachTooltips(bars, BuildTooltips(in.Table, in.Tooltip, in.Table.NumRows()))
	}
}
