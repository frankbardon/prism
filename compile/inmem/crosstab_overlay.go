package inmem

import (
	"fmt"
	"math"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
)

// overlayLayer holds the computed per-cell overlay value keyed by
// (row-key, col-key). Absent host cells and cells whose denominator /
// slice was degenerate carry no entry — value() returns 0 for them,
// matching the former matrix→long materialiser's zero-fill.
type overlayLayer struct {
	cells map[string]map[string]float64
}

// value returns the overlay value at (rk, ck), or 0 when absent.
func (l overlayLayer) value(rk, ck string) float64 {
	if m := l.cells[rk]; m != nil {
		return m[ck]
	}
	return 0
}

// overlayColumnName returns the output column name for an overlay,
// defaulting to the kind when As is empty.
func overlayColumnName(o spec.CrosstabOverlay) string {
	if o.As != "" {
		return o.As
	}
	return o.Kind
}

// overlayAxisFor resolves the margin axis an overlay reads. share_of_row
// / share_of_col are structurally axis-locked; index_vs_margin /
// zscore_vs_margin take the user-supplied axis.
func overlayAxisFor(o spec.CrosstabOverlay) string {
	switch o.Kind {
	case "share_of_row":
		return "row"
	case "share_of_col":
		return "column"
	case "index_vs_margin", "zscore_vs_margin":
		return o.Axis
	}
	return ""
}

// computeOverlay evaluates one overlay layer over the (already
// normalised) cell grid. Formulas mirror Pulse's cell-scoped,
// margin-referenced overlay handlers:
//
//   - share_of_row  : cell / row_margin
//   - share_of_col  : cell / col_margin
//   - index_vs_margin: cell / axis_margin * 100
//   - zscore_vs_margin: (cell - axis_margin) / sd(axis slice)
//
// Margins are the recompute-only aggregate margins the host produced
// (mean for AGG_MEAN, sum for AGG_SUM, …); the z-score sd is the
// population standard deviation of the present cell values in the same
// axis slice. Degenerate denominators (missing / zero margin, zero sd)
// yield an absent overlay cell.
func computeOverlay(o spec.CrosstabOverlay, rowAxis, colAxis *crosstabAxis,
	cellVal map[string]map[string]float64,
	rowMargin map[string]float64, rowPresent map[string]bool,
	colMargin map[string]float64, colPresent map[string]bool,
	grand float64, grandPresent bool,
) (overlayLayer, error) {
	axis := overlayAxisFor(o)
	if (o.Kind == "index_vs_margin" || o.Kind == "zscore_vs_margin") &&
		axis != "row" && axis != "column" {
		return overlayLayer{}, prismerrors.New(
			"PRISM_SPEC_032",
			fmt.Sprintf("crosstab overlay %q requires axis row or column (got %q).", o.Kind, o.Axis),
			map[string]any{"Kind": o.Kind, "Axis": o.Axis},
		)
	}
	if o.Kind != "share_of_row" && o.Kind != "share_of_col" &&
		o.Kind != "index_vs_margin" && o.Kind != "zscore_vs_margin" {
		return overlayLayer{}, prismerrors.New(
			"PRISM_SPEC_032",
			fmt.Sprintf("crosstab overlay kind %q not supported (use share_of_row/share_of_col/index_vs_margin/zscore_vs_margin).", o.Kind),
			map[string]any{"Kind": o.Kind},
		)
	}

	// Per-axis standard deviations for the z-score kind.
	rowSD := map[string]float64{}
	colSD := map[string]float64{}
	if o.Kind == "zscore_vs_margin" {
		if axis == "row" {
			for _, rk := range rowAxis.keys {
				rowSD[rk] = populationStdDev(rowSlice(rowAxis, colAxis, cellVal, rk))
			}
		} else {
			for _, ck := range colAxis.keys {
				colSD[ck] = populationStdDev(colSlice(rowAxis, colAxis, cellVal, ck))
			}
		}
	}

	out := overlayLayer{cells: map[string]map[string]float64{}}
	for _, rk := range rowAxis.keys {
		for _, ck := range colAxis.keys {
			cell, ok := cellVal[rk][ck]
			if !ok {
				continue
			}
			var margin float64
			var present bool
			switch axis {
			case "row":
				margin, present = rowMargin[rk], rowPresent[rk]
			case "column":
				margin, present = colMargin[ck], colPresent[ck]
			case "grand":
				margin, present = grand, grandPresent
			}
			var score float64
			switch o.Kind {
			case "share_of_row", "share_of_col":
				if !present || margin == 0 {
					continue
				}
				score = cell / margin
			case "index_vs_margin":
				if !present || margin == 0 {
					continue
				}
				score = cell / margin * 100.0
			case "zscore_vs_margin":
				if !present {
					continue
				}
				sd := rowSD[rk]
				if axis == "column" {
					sd = colSD[ck]
				}
				if sd == 0 {
					continue
				}
				score = (cell - margin) / sd
			}
			if math.IsNaN(score) || math.IsInf(score, 0) {
				continue
			}
			m := out.cells[rk]
			if m == nil {
				m = map[string]float64{}
				out.cells[rk] = m
			}
			m[ck] = score
		}
	}
	return out, nil
}

// rowSlice collects the present cell values across a row (for the
// z-score sd denominator).
func rowSlice(rowAxis, colAxis *crosstabAxis, cellVal map[string]map[string]float64, rk string) []float64 {
	var out []float64
	for _, ck := range colAxis.keys {
		if v, ok := cellVal[rk][ck]; ok {
			out = append(out, v)
		}
	}
	return out
}

// colSlice collects the present cell values down a column.
func colSlice(rowAxis, colAxis *crosstabAxis, cellVal map[string]map[string]float64, ck string) []float64 {
	var out []float64
	for _, rk := range rowAxis.keys {
		if v, ok := cellVal[rk][ck]; ok {
			out = append(out, v)
		}
	}
	return out
}

// populationStdDev returns the population standard deviation of the
// slice (sqrt of the population variance). Slices with fewer than two
// values return 0 — the degenerate contract Pulse's WelfordStdDev
// honours, which the z-score handler treats as a missing denominator.
func populationStdDev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	return math.Sqrt(variance(vals))
}
