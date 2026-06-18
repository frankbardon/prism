package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// CrosstabPosition implements PRISM_SPEC_032 / PRISM_SPEC_033:
//
//   - PRISM_SPEC_032: a crosstab transform must declare rows[],
//     columns[], cell.aggregate (and cell.field unless aggregate is
//     "count"). The plan node enforces the same at build time; the
//     validate rule surfaces the problem statically before any I/O.
//   - PRISM_SPEC_033: a crosstab transform may only appear as the
//     first transform on a chain — Pulse has no in-memory cohort
//     constructor, so chaining it after a Prism filter / aggregate
//     / join is impossible.
type CrosstabPosition struct{}

// Code returns PRISM_SPEC_032 (the broader of the two; PRISM_SPEC_033
// fires only when the position rule trips).
func (CrosstabPosition) Code() string { return "PRISM_SPEC_032" }

// Check walks every spec node and reports crosstab transforms that
// fail position or shape rules.
func (CrosstabPosition) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	var out []*errors.AppError
	walkCrosstab(s, "", &out)
	return out
}

// crosstabDatePeriods mirrors Pulse's GROUP_DATE component set
// (processing/grouper.go). Empty period defaults to month.
var crosstabDatePeriods = map[string]bool{
	"year": true, "quarter": true, "month": true,
	"week": true, "day": true, "day_of_week": true,
}

// checkCrosstabGroupers statically validates grouper kind + period for
// one axis. Mirrors translateGroupers in plan/nodes/crosstab.go so the
// error surfaces before any I/O.
func checkCrosstabGroupers(groups []spec.CrosstabGroup, path string, out *[]*errors.AppError) {
	for i, g := range groups {
		switch g.Type {
		case "", "category":
			// ok — category groupers ignore period.
		case "date":
			if g.Period != "" && !crosstabDatePeriods[g.Period] {
				*out = append(*out, errors.New(
					"PRISM_SPEC_032",
					fmt.Sprintf("crosstab date grouper period %q at %s[%d] must be one of year/quarter/month/week/day/day_of_week.", g.Period, path, i),
					map[string]any{"Path": path, "Index": i, "Period": g.Period},
				))
			}
		default:
			*out = append(*out, errors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab grouper type %q at %s[%d] must be category or date.", g.Type, path, i),
				map[string]any{"Path": path, "Index": i, "Type": g.Type},
			))
		}
	}
}

// crosstabOverlayKinds lists the friendly overlay names the crosstab
// node supports (mirrors crosstabOverlayKinds in plan/nodes/crosstab.go).
// index_vs_margin additionally requires axis row|column.
var crosstabOverlayKinds = map[string]bool{
	"share_of_row": true, "share_of_col": true,
	"index_vs_margin": true, "zscore_vs_margin": true,
}

// crosstabOverlayUserAxis lists the overlay kinds that require an
// explicit axis (row|column); the rest bake the axis into the kind.
var crosstabOverlayUserAxis = map[string]bool{
	"index_vs_margin": true, "zscore_vs_margin": true,
}

// checkCrosstabOverlays statically validates overlay kind + axis.
func checkCrosstabOverlays(overlays []spec.CrosstabOverlay, path string, out *[]*errors.AppError) {
	for i, o := range overlays {
		if !crosstabOverlayKinds[o.Kind] {
			*out = append(*out, errors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab overlay kind %q at %s[%d] must be share_of_row, share_of_col, or index_vs_margin.", o.Kind, path, i),
				map[string]any{"Path": path, "Index": i, "Kind": o.Kind},
			))
			continue
		}
		if crosstabOverlayUserAxis[o.Kind] && o.Axis != "row" && o.Axis != "column" {
			*out = append(*out, errors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab overlay %s at %s[%d] requires axis row or column (got %q).", o.Kind, path, i, o.Axis),
				map[string]any{"Path": path, "Index": i, "Kind": o.Kind, "Axis": o.Axis},
			))
		}
	}
}

func walkCrosstab(s *spec.Spec, prefix string, out *[]*errors.AppError) {
	if s == nil {
		return
	}
	for i, t := range s.Transform {
		if t.Crosstab == nil {
			continue
		}
		path := fmt.Sprintf("%stransform[%d].crosstab", prefix, i)
		// Position: must be the first transform on the chain (or
		// reference a registered dataset via its `data` alias —
		// dataset-level crosstab is also leaf-bound).
		if i > 0 && t.Crosstab.Data == "" {
			*out = append(*out, errors.New(
				"PRISM_SPEC_033",
				fmt.Sprintf("crosstab at %s must be the first transform on the chain.", path),
				map[string]any{"Path": path, "Index": i},
			))
		}
		// Shape: rows + columns + cell.aggregate required.
		if len(t.Crosstab.Crosstab.Rows) == 0 {
			*out = append(*out, errors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab.rows at %s requires at least one grouper.", path),
				map[string]any{"Axis": "rows", "Path": path},
			))
		}
		if len(t.Crosstab.Crosstab.Columns) == 0 {
			*out = append(*out, errors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab.columns at %s requires at least one grouper.", path),
				map[string]any{"Axis": "columns", "Path": path},
			))
		}
		if t.Crosstab.Crosstab.Cell.Aggregate == "" {
			*out = append(*out, errors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab.cell.aggregate at %s is required.", path),
				map[string]any{"Path": path},
			))
		}
		if t.Crosstab.Crosstab.Cell.Field == "" && t.Crosstab.Crosstab.Cell.Aggregate != "count" {
			*out = append(*out, errors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab.cell.field at %s is required for aggregate %q.", path, t.Crosstab.Crosstab.Cell.Aggregate),
				map[string]any{"Path": path, "Aggregate": t.Crosstab.Crosstab.Cell.Aggregate},
			))
		}
		// Grouper kind / period enum check (rows + columns).
		checkCrosstabGroupers(t.Crosstab.Crosstab.Rows, path+".rows", out)
		checkCrosstabGroupers(t.Crosstab.Crosstab.Columns, path+".columns", out)
		// Overlay kind / axis enum check.
		checkCrosstabOverlays(t.Crosstab.Crosstab.Overlays, path+".overlays", out)
		// Normalize: enum check.
		switch t.Crosstab.Crosstab.Normalize {
		case "", "none", "row", "column", "total":
			// ok
		default:
			*out = append(*out, errors.New(
				"PRISM_SPEC_034",
				fmt.Sprintf("crosstab.normalize at %s must be one of none/row/column/total (got %q).", path, t.Crosstab.Crosstab.Normalize),
				map[string]any{"Path": path, "Normalize": t.Crosstab.Crosstab.Normalize},
			))
		}
	}
	for i, layer := range s.Layer {
		walkCrosstab(layer, fmt.Sprintf("%slayer[%d].", prefix, i), out)
	}
	for i, child := range s.Concat {
		walkCrosstab(child, fmt.Sprintf("%sconcat[%d].", prefix, i), out)
	}
	for i, child := range s.HConcat {
		walkCrosstab(child, fmt.Sprintf("%shconcat[%d].", prefix, i), out)
	}
	for i, child := range s.VConcat {
		walkCrosstab(child, fmt.Sprintf("%svconcat[%d].", prefix, i), out)
	}
	if s.ChildSpec != nil {
		walkCrosstab(s.ChildSpec, prefix+"spec.", out)
	}
}
