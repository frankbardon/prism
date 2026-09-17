package marks

import (
	"fmt"
	"sort"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeArea partitions rows by the bound discrete grouping channels
// — color and/or detail (Vega-Lite semantics: either one on an area
// mark splits it into one ribbon per distinct value — mirrors
// encodeLine) — and emits one scene.Mark per group, each carrying its
// own resolved fill color via groupRows / lookupCategoryColor — the
// same palette resolution the legend uses. A detail-only split leaves
// in.Style's fill untouched: detail groups series without consuming a
// palette slot or producing a legend. Within each group, points are
// sorted by resolved x pixel ascending so the ribbon traces
// left-to-right rather than upstream row order — unless the encoding
// binds `order` (E5-S4), which hands the point sequence to the
// author: the plan has already sorted the rows and each group traces
// in table order.
//
// Each group's Upper is its row-by-row points and Lower is the y=0
// baseline edge (one point per Upper x, snapped to the pixel where
// the data value is 0). The baseline is the scale's zero, so
// positive-only domains fill down to the plot bottom and
// zero-crossing domains fill above and below the mid-plot zero line.
//
// Stacking (E5-S2) needs no code here: the StackNode's bounds columns
// reach this encoder as an ordinary y / y2 pair (encode/stack.go
// rebinds the channels before scales resolve), so a stacked area is
// just the y2 path below with a per-series lower edge. The centred
// streamgraph offset (E5-S3) is the same arrangement with signed
// bounds — the accumulation and the inside-out segment ordering both
// happen upstream in the plan node, and this encoder only ever sees a
// series with two edges.
//
// Binding y2 (E9-S3) replaces that implicit baseline with an explicit
// lower edge read per row from the y2 column and resolved through the
// y scale — the band shape behind a confidence interval. The
// geometry is otherwise identical, so grouping, sorting and curve
// interpolation all carry over unchanged. x2 is not expressible on an
// area and is rejected at validate (PRISM_SPEC_041).
//
// When neither channel is bound, behavior is unchanged from before
// grouping existed: a single scene.Mark ("area-0") carrying every
// row's points in raw upstream order.
//
// Orientation (E9-S2): an area has the same category/measure split a
// bar does, except its category axis is usually continuous or temporal
// rather than banded — so it resolves through MarkOrientationOr
// (orient.go) with a vertical fallback. `"orient": "horizontal"` moves
// the series axis to y and the baseline to x = 0, which is the shape a
// horizontal ribbon needs. Everything else — grouping, ordering along
// the series axis, curve interpolation — is orientation-agnostic.
func encodeArea(in Inputs) ([]scene.Mark, error) {
	orient, err := MarkOrientationOr(in, "area", OrientVertical)
	if err != nil {
		return nil, err
	}
	category := CategoryChannel(in, orient)
	measure := MeasureChannel(in, orient)
	cats, err := readField(in.Table, category.Field)
	if err != nil {
		return nil, err
	}
	vals, err := readField(in.Table, measure.Field)
	if err != nil {
		return nil, err
	}
	if len(cats) != len(vals) {
		return nil, fmt.Errorf("encodeArea: column length mismatch (%s=%d, %s=%d)",
			orient.CategoryAxis(), len(cats), orient.MeasureAxis(), len(vals))
	}
	if len(cats) == 0 {
		return nil, nil
	}
	// Baseline = the measure axis's data-zero pixel (mirrors bar.go).
	// Positive-only domains snap this to the plot edge; zero-crossing
	// domains land it mid-plot.
	baseline := BaselinePixel(in, orient)
	// y2 (E9-S3), when bound, supplies the lower edge per row instead
	// of the baseline. Left nil otherwise, which keeps the baseline
	// path byte-identical. y2 is the *measure* companion only while the
	// area is vertical; on a horizontal area y2 would be a second
	// position on the series axis, which the geometry cannot express —
	// rejected rather than silently ignored.
	var lows []any
	if spanBound(in.Y2) {
		if orient == OrientHorizontal {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				"A horizontal area measures along x, so y2 has no lower edge to supply.",
				map[string]any{"Field": in.Y2.Field, "Source": "<y2>", "Available": "vertical"},
			)
		}
		lows, err = readField(in.Table, in.Y2.Field)
		if err != nil {
			return nil, err
		}
		if len(lows) != len(cats) {
			return nil, fmt.Errorf("encodeArea: column length mismatch (x=%d, y2=%d)", len(cats), len(lows))
		}
	}
	// seriesPos is the pixel along the category (series) axis; it is
	// what the grouped path sorts on, in either orientation.
	seriesPos := make([]float64, len(cats))
	upperAll := make([][2]float64, len(cats))
	lowerAll := make([][2]float64, len(cats))
	for i := range cats {
		// Masked by mark.invalid:"break": the row carries a null in a
		// scale-bound channel and has no pixel. Its slots stay zero
		// and segmentRuns keeps it out of every run.
		if skipRow(in, i) {
			continue
		}
		c, err := PointPixel(category, cats[i])
		if err != nil {
			return nil, err
		}
		m, err := measure.Scale.Apply(vals[i])
		if err != nil {
			return nil, err
		}
		low := baseline
		if lows != nil {
			if low, err = in.Y2.Scale.Apply(lows[i]); err != nil {
				return nil, err
			}
		}
		seriesPos[i] = c
		ux, uy := OrientedPoint(orient, c, m)
		lx, ly := OrientedPoint(orient, c, low)
		upperAll[i] = [2]float64{ux, uy}
		lowerAll[i] = [2]float64{lx, ly}
	}

	// An `order` binding (E5-S4) means the plan already sequenced the
	// rows; honour that sequence instead of the default left-to-right
	// x-sort, which is the whole point of the channel's path sense.
	sortByX := len(groupChannels(in)) > 0 && !in.Ordered
	groups, err := groupRows(in, len(cats))
	if err != nil {
		return nil, err
	}

	curve, tension := curveFor(in)

	marks := make([]scene.Mark, 0, len(groups))
	for gi, g := range groups {
		idxs := append([]int(nil), g.indices...)
		if sortByX {
			sort.SliceStable(idxs, func(a, b int) bool {
				return seriesPos[idxs[a]] < seriesPos[idxs[b]]
			})
		}
		style := in.Style
		if g.color != nil || g.varName != "" {
			style.Fill = g.color
			style.FillVar = g.varName
		}
		// One filled band per unbroken run. With no mask there is
		// exactly one run per group and the ID keeps its historic
		// "area-<group>" spelling, so nothing moves.
		runs := segmentRuns(in, idxs)
		for si, run := range runs {
			upper := make([][2]float64, len(run))
			lower := make([][2]float64, len(run))
			for j, idx := range run {
				upper[j] = upperAll[idx]
				lower[j] = lowerAll[idx]
			}
			id := fmt.Sprintf("area-%d", gi)
			if len(runs) > 1 {
				id = fmt.Sprintf("area-%d-%d", gi, si)
			}
			marks = append(marks, scene.Mark{
				Type:  scene.MarkArea,
				ID:    id,
				Style: style,
				Area: &scene.AreaGeom{
					Upper:   upper,
					Lower:   lower,
					Curve:   curve,
					Tension: tension,
				},
			})
		}
	}
	return marks, nil
}
