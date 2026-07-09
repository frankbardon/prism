package inmem

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// executeCrosstab builds a contingency table (rows × columns × cell
// aggregate → long-form table) over the single materialised input,
// pure-Go with no engine coupling. It reproduces the shape the former
// Pulse-backed leaf emitted — sorted composite axis keys, present-only
// body cells, recompute-only margin axes, row/column/total
// normalisation, and the cell-scoped overlay layers (share_of_row,
// share_of_col, index_vs_margin, zscore_vs_margin) — but keys carry
// their native typed values instead of stringified numerics.
//
// Cell + margin aggregates reuse computeAggregate (group_aggregate.go),
// so the numerical semantics are identical to every other aggregate
// transform. Margins are recompute-only: each row/column/grand margin
// is the aggregate over the marginalised record slice, not a fold of
// cell values (correct for mean/median/… as well as sum/count).
func executeCrosstab(_ context.Context, n *nodes.CrosstabNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}
	body := n.Body()
	cellAs := n.CellAs()

	rowAxis, err := buildCrosstabAxis(in, body.Rows, "rows")
	if err != nil {
		return nil, err
	}
	colAxis, err := buildCrosstabAxis(in, body.Columns, "columns")
	if err != nil {
		return nil, err
	}

	// Only records whose row AND column keys are both present land in a
	// cell (a null on either axis drops the record, matching the Pulse
	// grouper's ErrGrouperKeyNull contract).
	cellIdx := map[string]map[string][]int{}
	for i := 0; i < in.NumRows(); i++ {
		rk, rok := rowAxis.keyAt(i)
		ck, cok := colAxis.keyAt(i)
		if !rok || !cok {
			continue
		}
		byCol := cellIdx[rk]
		if byCol == nil {
			byCol = map[string][]int{}
			cellIdx[rk] = byCol
		}
		byCol[ck] = append(byCol[ck], i)
	}

	op := nodes.AggOp{Op: body.Cell.Aggregate, Field: body.Cell.Field, As: cellAs}
	shareTotals := crosstabShareTotals(in, op)

	// Cell values over present (row, col) partitions.
	cellVal := map[string]map[string]float64{}
	for _, rk := range rowAxis.keys {
		for _, ck := range colAxis.keys {
			idx := cellIdx[rk][ck]
			if len(idx) == 0 {
				continue
			}
			v, err := computeAggregate(in, op, idx, shareTotals)
			if err != nil {
				return nil, err
			}
			m := cellVal[rk]
			if m == nil {
				m = map[string]float64{}
				cellVal[rk] = m
			}
			m[ck] = v
		}
	}

	// Recompute-only margins over the marginalised record slices.
	needRowMargin, needColMargin, needGrand := crosstabMarginNeeds(body)
	rowMargin := map[string]float64{}
	rowMarginPresent := map[string]bool{}
	if needRowMargin {
		for _, rk := range rowAxis.keys {
			idx := rowAxis.idx[rk]
			if len(idx) == 0 {
				continue
			}
			v, err := computeAggregate(in, op, idx, shareTotals)
			if err != nil {
				return nil, err
			}
			rowMargin[rk] = v
			rowMarginPresent[rk] = true
		}
	}
	colMargin := map[string]float64{}
	colMarginPresent := map[string]bool{}
	if needColMargin {
		for _, ck := range colAxis.keys {
			idx := colAxis.idx[ck]
			if len(idx) == 0 {
				continue
			}
			v, err := computeAggregate(in, op, idx, shareTotals)
			if err != nil {
				return nil, err
			}
			colMargin[ck] = v
			colMarginPresent[ck] = true
		}
	}
	var grand float64
	grandPresent := false
	if needGrand {
		all := make([]int, in.NumRows())
		for i := range all {
			all[i] = i
		}
		v, err := computeAggregate(in, op, all, shareTotals)
		if err != nil {
			return nil, err
		}
		grand, grandPresent = v, true
	}

	// Normalisation divides each present cell by the matching margin;
	// a zero/absent denominator drops the cell (Present=false).
	if err := applyCrosstabNormalize(body.Normalize, rowAxis, colAxis, cellVal,
		rowMargin, rowMarginPresent, colMargin, colMarginPresent, grand, grandPresent); err != nil {
		return nil, err
	}

	var rows []crosstabRow
	if len(body.Overlays) > 0 {
		rows, err = buildOverlayRows(body, rowAxis, colAxis, cellVal,
			rowMargin, rowMarginPresent, colMargin, colMarginPresent, grand, grandPresent, cellAs)
		if err != nil {
			return nil, err
		}
	} else {
		rows = buildPlainRows(body, rowAxis, colAxis, cellVal,
			rowMargin, rowMarginPresent, colMargin, colMarginPresent, grand, grandPresent, cellAs)
	}

	hash := hashChain(in.Hash(), n.Fingerprint())
	outSchema, err := n.Schema([]*table.Schema{in.Schema()})
	if err != nil {
		return nil, err
	}
	return materializeCrosstab(rows, outSchema, hash)
}

// crosstabRow is one long-form output row: the typed axis-key values
// keyed by field name, plus the cell / overlay / margin columns.
type crosstabRow struct {
	values map[string]any
}

// crosstabAxis holds the per-record key strings for one axis plus the
// sorted distinct composite keys, the record indices per key, and the
// typed output value each grouper contributes for a given key.
type crosstabAxis struct {
	fields    []string
	keys      []string         // sorted distinct composite keys (present, non-null)
	idx       map[string][]int // composite key → record indices
	perRecord []crosstabRecKey // per input row: composite key + presence
	repr      map[string][]any // composite key → typed value per grouper (output)
}

// crosstabRecKey is one record's composite axis key.
type crosstabRecKey struct {
	key     string
	present bool
}

// keyAt returns the composite key for record i and whether it is
// present (all groupers non-null).
func (a *crosstabAxis) keyAt(i int) (string, bool) {
	rk := a.perRecord[i]
	return rk.key, rk.present
}

// grouperResolver computes one grouper's (key component, typed output
// value) for a record, or reports the value is null.
type grouperResolver struct {
	field  string
	kind   string // "category" | "date"
	period string // calendar component for date groupers
	col    table.Column
}

// buildCrosstabAxis precomputes the per-record composite keys, the
// sorted distinct key list, the record-index partition, and the typed
// output value per grouper per key.
func buildCrosstabAxis(in *table.Table, groups []spec.CrosstabGroup, axis string) (*crosstabAxis, error) {
	if len(groups) == 0 {
		return nil, prismerrors.New(
			"PRISM_SPEC_032",
			fmt.Sprintf("crosstab.%s requires at least one grouper.", axis),
			map[string]any{"Axis": axis},
		)
	}
	resolvers := make([]grouperResolver, len(groups))
	fields := make([]string, len(groups))
	for i, g := range groups {
		col, ok := in.Column(g.Field)
		if !ok {
			return nil, prismerrors.New(
				"PRISM_PLAN_003",
				fmt.Sprintf("crosstab.%s references missing field %q.", axis, g.Field),
				map[string]any{"Dataset": g.Field, "Available": strings.Join(in.FieldNames(), ", ")},
			)
		}
		kind := g.Type
		if kind == "" {
			kind = "category"
		}
		period := g.Period
		if kind == "date" && period == "" {
			period = "month"
		}
		if kind == "date" && !crosstabDateComponent(period) {
			return nil, prismerrors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab date grouper period %q must be one of year/quarter/month/week/day/day_of_week.", period),
				map[string]any{"Axis": axis, "Period": period},
			)
		}
		if kind != "category" && kind != "date" {
			return nil, prismerrors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab grouper type %q not supported (use category or date).", kind),
				map[string]any{"Axis": axis, "Type": kind},
			)
		}
		resolvers[i] = grouperResolver{field: g.Field, kind: kind, col: col}
		resolvers[i].period = period
		fields[i] = g.Field
	}

	rows := in.NumRows()
	a := &crosstabAxis{
		fields:    fields,
		idx:       map[string][]int{},
		perRecord: make([]crosstabRecKey, rows),
		repr:      map[string][]any{},
	}
	for i := 0; i < rows; i++ {
		parts := make([]string, len(resolvers))
		reprs := make([]any, len(resolvers))
		present := true
		for j := range resolvers {
			part, out, ok := resolvers[j].resolve(i)
			if !ok {
				present = false
				break
			}
			parts[j] = part
			reprs[j] = out
		}
		if !present {
			a.perRecord[i] = crosstabRecKey{present: false}
			continue
		}
		key := strings.Join(parts, "\x00")
		a.perRecord[i] = crosstabRecKey{key: key, present: true}
		if _, seen := a.idx[key]; !seen {
			a.keys = append(a.keys, key)
			a.repr[key] = reprs
		}
		a.idx[key] = append(a.idx[key], i)
	}
	// Deterministic output ordering: sort composite keys as strings
	// (the Pulse grouper contract).
	sort.Strings(a.keys)
	return a, nil
}

// resolve computes one grouper's (key component, typed output value)
// for record i. Reports ok=false when the value is null (the record is
// dropped from the axis, mirroring the grouper's null-key contract).
func (r grouperResolver) resolve(i int) (string, any, bool) {
	if r.col.IsNull(i) {
		return "", nil, false
	}
	v := r.col.ValueAt(i)
	if v == nil {
		return "", nil, false
	}
	if r.kind == "date" {
		days, ok := toInt64(v)
		if !ok {
			return "", nil, false
		}
		key := formatCrosstabDateKey(time.Unix(days*86400, 0).UTC(), r.period)
		return key, key, true
	}
	// Category grouper. Categorical fields already surface their string
	// label; numeric fields stringify via the rounded-numeric contract
	// but keep their native typed value in the output column.
	switch x := v.(type) {
	case string:
		return x, x, true
	default:
		f, ok := toFloat64(v)
		if !ok {
			return fmt.Sprintf("%v", v), v, true
		}
		return formatRoundedNumeric(f), v, true
	}
}

// buildPlainRows emits the long-form body rows (present cells only),
// then the row / column / grand margin rows when enabled — the exact
// order and _margin sentinels the former Pulse path produced.
func buildPlainRows(body spec.CrosstabBody, rowAxis, colAxis *crosstabAxis,
	cellVal map[string]map[string]float64,
	rowMargin map[string]float64, rowPresent map[string]bool,
	colMargin map[string]float64, colPresent map[string]bool,
	grand float64, grandPresent bool, cellAs string,
) []crosstabRow {
	var out []crosstabRow
	for _, rk := range rowAxis.keys {
		for _, ck := range colAxis.keys {
			v, ok := cellVal[rk][ck]
			if !ok {
				continue
			}
			row := map[string]any{}
			putAxisValues(row, rowAxis, rk)
			putAxisValues(row, colAxis, ck)
			row[cellAs] = v
			out = append(out, crosstabRow{values: row})
		}
	}
	margins := body.Margins
	if margins != nil && margins.Rows {
		for _, rk := range rowAxis.keys {
			if !rowPresent[rk] {
				continue
			}
			row := map[string]any{}
			putAxisValues(row, rowAxis, rk)
			row[cellAs] = rowMargin[rk]
			row["_margin"] = "row"
			out = append(out, crosstabRow{values: row})
		}
	}
	if margins != nil && margins.Columns {
		for _, ck := range colAxis.keys {
			if !colPresent[ck] {
				continue
			}
			row := map[string]any{}
			putAxisValues(row, colAxis, ck)
			row[cellAs] = colMargin[ck]
			row["_margin"] = "column"
			out = append(out, crosstabRow{values: row})
		}
	}
	if margins != nil && margins.Grand && grandPresent {
		out = append(out, crosstabRow{values: map[string]any{cellAs: grand, "_margin": "grand"}})
	}
	return out
}

// buildOverlayRows emits body cells only (present) with one column per
// overlay layer, index-aligned to body.Overlays. Margins never reach
// the overlay output.
func buildOverlayRows(body spec.CrosstabBody, rowAxis, colAxis *crosstabAxis,
	cellVal map[string]map[string]float64,
	rowMargin map[string]float64, rowPresent map[string]bool,
	colMargin map[string]float64, colPresent map[string]bool,
	grand float64, grandPresent bool, cellAs string,
) ([]crosstabRow, error) {
	overlays := make([]overlayLayer, len(body.Overlays))
	for i, o := range body.Overlays {
		layer, err := computeOverlay(o, rowAxis, colAxis, cellVal,
			rowMargin, rowPresent, colMargin, colPresent, grand, grandPresent)
		if err != nil {
			return nil, err
		}
		overlays[i] = layer
	}

	var out []crosstabRow
	for _, rk := range rowAxis.keys {
		for _, ck := range colAxis.keys {
			v, ok := cellVal[rk][ck]
			if !ok {
				continue
			}
			row := map[string]any{}
			putAxisValues(row, rowAxis, rk)
			putAxisValues(row, colAxis, ck)
			row[cellAs] = v
			for i, o := range body.Overlays {
				row[overlayColumnName(o)] = overlays[i].value(rk, ck)
			}
			out = append(out, crosstabRow{values: row})
		}
	}
	return out, nil
}

// putAxisValues writes one axis's typed grouper values onto the row map
// under their field names.
func putAxisValues(row map[string]any, axis *crosstabAxis, key string) {
	reprs := axis.repr[key]
	for i, f := range axis.fields {
		if i < len(reprs) {
			row[f] = reprs[i]
		}
	}
}

// crosstabShareTotals precomputes the whole-table denominator when the
// cell aggregate is "share" (computeAggregate consults it per group).
func crosstabShareTotals(in *table.Table, op nodes.AggOp) map[string]float64 {
	totals := map[string]float64{}
	if strings.ToLower(op.Op) != "share" || op.Field == "" {
		return totals
	}
	col, ok := in.Column(op.Field)
	if !ok {
		return totals
	}
	total := 0.0
	for _, v := range floatColumnValues(col) {
		total += v
	}
	totals[op.Field] = total
	return totals
}

// crosstabMarginNeeds reports which margin axes must be recomputed:
// explicit margin flags for the plain path, plus the denominators the
// normalisation mode and overlay layers require.
func crosstabMarginNeeds(body spec.CrosstabBody) (row, col, grand bool) {
	if body.Margins != nil {
		row = body.Margins.Rows
		col = body.Margins.Columns
		grand = body.Margins.Grand
	}
	switch body.Normalize {
	case "row":
		row = true
	case "column":
		col = true
	case "total":
		grand = true
	}
	for _, o := range body.Overlays {
		switch overlayAxisFor(o) {
		case "row":
			row = true
		case "column":
			col = true
		case "grand":
			grand = true
		}
	}
	return row, col, grand
}

// applyCrosstabNormalize divides each present cell by the matching
// margin denominator. A zero or absent denominator drops the cell.
func applyCrosstabNormalize(mode string, rowAxis, colAxis *crosstabAxis,
	cellVal map[string]map[string]float64,
	rowMargin map[string]float64, rowPresent map[string]bool,
	colMargin map[string]float64, colPresent map[string]bool,
	grand float64, grandPresent bool,
) error {
	switch mode {
	case "", "none":
		return nil
	case "row", "column", "total":
		// handled below
	default:
		return prismerrors.New(
			"PRISM_SPEC_034",
			fmt.Sprintf("crosstab.normalize must be one of none/row/column/total (got %q).", mode),
			map[string]any{"Normalize": mode},
		)
	}
	for _, rk := range rowAxis.keys {
		for _, ck := range colAxis.keys {
			v, ok := cellVal[rk][ck]
			if !ok {
				continue
			}
			var denom float64
			var present bool
			switch mode {
			case "row":
				denom, present = rowMargin[rk], rowPresent[rk]
			case "column":
				denom, present = colMargin[ck], colPresent[ck]
			case "total":
				denom, present = grand, grandPresent
			}
			if !present || denom == 0 {
				delete(cellVal[rk], ck)
				continue
			}
			cellVal[rk][ck] = v / denom
		}
	}
	return nil
}

// crosstabDateComponent reports whether the period is a supported
// calendar component.
func crosstabDateComponent(period string) bool {
	switch period {
	case "year", "quarter", "month", "week", "day", "day_of_week":
		return true
	}
	return false
}

// formatCrosstabDateKey renders a day-resolution timestamp into the
// bucket label for a calendar component. Mirrors Pulse's dateGrouper
// formatDateKey so labels stay identical ("2024", "2024-Q1", "2024-03",
// "2024-W05", "2024-03-14", "Monday").
func formatCrosstabDateKey(t time.Time, component string) string {
	switch component {
	case "year":
		return strconv.Itoa(t.Year())
	case "quarter":
		return fmt.Sprintf("%d-Q%d", t.Year(), (int(t.Month())-1)/3+1)
	case "month":
		return t.Format("2006-01")
	case "week":
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	case "day":
		return t.Format("2006-01-02")
	case "day_of_week":
		return t.Weekday().String()
	}
	return ""
}

// formatRoundedNumeric stringifies a numeric category-grouper key.
// Mirrors Pulse's processing.formatRoundedNumeric so integral values
// render without a trailing ".0".
func formatRoundedNumeric(v float64) string {
	if v == math.Trunc(v) && !math.IsInf(v, 0) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// toFloat64 coerces a column value to float64.
func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int64:
		return float64(x), true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// toInt64 coerces a column value to int64.
func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case float64:
		return int64(x), true
	}
	return 0, false
}

// materializeCrosstab drains typed long rows into a *table.Table with
// the pre-derived output schema. Absent map entries default to the
// column's zero value (matching the former long-row materialiser).
func materializeCrosstab(rows []crosstabRow, schema *table.Schema, hash string) (*table.Table, error) {
	if schema == nil {
		return nil, fmt.Errorf("crosstab: nil output schema")
	}
	cols := make(map[string]table.Column, len(schema.Fields))
	for i := range schema.Fields {
		f := &schema.Fields[i]
		switch table.KindFromFieldType(f.Type) {
		case table.KindString:
			cols[f.Name] = make(table.StringColumn, 0, len(rows))
		case table.KindFloat:
			cols[f.Name] = make(table.FloatColumn, 0, len(rows))
		case table.KindInt:
			cols[f.Name] = make(table.IntColumn, 0, len(rows))
		case table.KindBool:
			cols[f.Name] = make(table.BoolColumn, 0, len(rows))
		case table.KindDate:
			cols[f.Name] = make(table.DateColumn, 0, len(rows))
		default:
			return nil, fmt.Errorf("crosstab: unsupported field type %s for %q", f.Type, f.Name)
		}
	}
	for _, r := range rows {
		for i := range schema.Fields {
			f := &schema.Fields[i]
			raw, present := r.values[f.Name]
			switch table.KindFromFieldType(f.Type) {
			case table.KindString:
				s := ""
				if present && raw != nil {
					s = crosstabString(raw)
				}
				cols[f.Name] = append(cols[f.Name].(table.StringColumn), s)
			case table.KindFloat:
				v := 0.0
				if present && raw != nil {
					v, _ = toFloat64(raw)
				}
				cols[f.Name] = append(cols[f.Name].(table.FloatColumn), v)
			case table.KindInt:
				v := int64(0)
				if present && raw != nil {
					v, _ = toInt64(raw)
				}
				cols[f.Name] = append(cols[f.Name].(table.IntColumn), v)
			case table.KindBool:
				b := false
				if present && raw != nil {
					b, _ = raw.(bool)
				}
				cols[f.Name] = append(cols[f.Name].(table.BoolColumn), b)
			case table.KindDate:
				v := int64(0)
				if present && raw != nil {
					v, _ = toInt64(raw)
				}
				cols[f.Name] = append(cols[f.Name].(table.DateColumn), v)
			}
		}
	}
	return table.NewTable(schema, cols, len(rows), hash)
}

// crosstabString renders an axis value for a string output column.
func crosstabString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
