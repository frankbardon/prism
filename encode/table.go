package encode

import (
	"encoding/json"
	"fmt"

	"github.com/frankbardon/prism/encode/marks"
	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// tableCellSparkWidth / tableCellSparkHeight size the synthetic plot
// rect a sub-mark column's per-row cell is encoded against (E1-S3).
// The render backend (E1-S4) is free to scale the resulting <svg> to
// fit its own cell layout — these are just the coordinate space the
// nested Scene IR's geometry is expressed in, matching the compact
// sparkline footprint used elsewhere (encode/layout.go's
// ComputeSparkline uses the same order of magnitude). Aliased to the
// exported scene.TableCellSparkWidth/Height constants so the render
// backend that re-wraps a cell's Scene IR for standalone emission
// (render/html) shares the exact same coordinate-space contract.
const (
	tableCellSparkWidth  = scene.TableCellSparkWidth
	tableCellSparkHeight = scene.TableCellSparkHeight
)

// buildTableSceneDoc builds the full SceneDoc for a table-mark spec
// (E1). Unlike every other mark, a table has no cartesian/polar
// geometry — Encode dispatches here before any x/y scale resolution
// or mark-family encoding runs.
func buildTableSceneDoc(
	s *spec.Spec, tbl *table.Table, enc *spec.Encoding, fullTheme *theme.Theme,
	sceneTheme *scene.Theme, layout Layout, hasTitle bool,
) (*scene.SceneDoc, error) {
	tableIR, warnings, err := buildTable(s, tbl, enc, fullTheme)
	if err != nil {
		return nil, err
	}

	// Layers still carries one entry so render/svg's checkMarkSupport
	// guard (render/svg/mark_support.go) keeps rejecting a direct
	// svg-backend request for a table mark; Marks stays empty since a
	// table has no positioned geometry of its own.
	layer := scene.SceneLayer{ID: "layer-0", Mark: scene.MarkTable}

	sceneObj := scene.Scene{
		ID:         "scene-0",
		Frame:      layout.Frame,
		Plot:       layout.Plot,
		Layers:     []scene.SceneLayer{layer},
		Table:      tableIR,
		Selections: BuildSelections(s.Selection),
		Animation:  animationFromSpec(s),
	}
	if hasTitle {
		sceneObj.Title = &scene.TextElement{
			Content: titleText(s),
			X:       layout.Plot.CenterX(),
			Y:       20,
		}
	}
	doc := scene.NewDoc()
	doc.Theme = sceneTheme
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: sceneObj},
		},
	}
	doc.Warnings = warnings
	return doc, nil
}

// buildTable resolves a table-mark spec's encoding.columns[] against
// the upstream (filtered/sorted/limited/aggregated) table into a
// scene.Table: one TableColumn per declared column and one TableRow
// per upstream row. Columns.check has already run (PRISM_SPEC_040)
// so enc.Columns is non-empty by the time a validated spec reaches
// here; the length check below is defensive for callers that skip
// validation.
func buildTable(s *spec.Spec, tbl *table.Table, enc *spec.Encoding, fullTheme *theme.Theme) (*scene.Table, []scene.Warning, error) {
	if enc == nil || len(enc.Columns) == 0 {
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			`Mark "table" has no columns to encode; encoding.columns[] is required.`,
			map[string]any{"Field": "<encoding.columns>", "Source": "<spec>", "Available": ""},
		)
	}

	pageSize := spec.TablePageSizeDefault
	if s.Mark != nil && s.Mark.Def != nil && s.Mark.Def.PageSize != nil {
		pageSize = *s.Mark.Def.PageSize
	}

	cols := make([]scene.TableColumn, 0, len(enc.Columns))
	for _, c := range enc.Columns {
		if _, ok := tbl.Column(c.Field); !ok {
			return nil, nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("Table column field %q not present in upstream table.", c.Field),
				map[string]any{"Field": c.Field, "Source": "<table>", "Available": joinTableFields(tbl)},
			)
		}
		header := c.Title
		if header == "" {
			header = c.Field
		}
		cols = append(cols, scene.TableColumn{Field: c.Field, Header: header, Mark: c.Mark})
	}

	// Sub-mark cells reuse the standalone mark encoder's default
	// style so a sparkline column's line looks like any other
	// standalone sparkline; theme resolution follows the same
	// cascade defaultMarkStyle already applies elsewhere in encode.go.
	subMarkStyles := map[string]scene.Style{}

	n := tbl.NumRows()
	rows := make([]scene.TableRow, 0, n)
	var warnings []scene.Warning
	for i := 0; i < n; i++ {
		row := scene.TableRow{ID: int64(i), Values: make(map[string]any, len(cols))}
		for _, c := range enc.Columns {
			col, ok := tbl.Column(c.Field)
			if !ok {
				continue // already validated to exist above
			}
			raw := col.ValueAt(i)
			row.Values[c.Field] = raw
			if c.Mark == "" {
				continue
			}
			series, ok := parseNumericSeries(raw)
			if !ok {
				warnings = append(warnings, scene.Warning{
					Code:    scene.WarnTableCellUnparseable,
					Message: fmt.Sprintf("Table row %d column %q: value could not be parsed as a numeric series for sub-mark %q.", i, c.Field, c.Mark),
					Details: map[string]any{"Row": i, "Field": c.Field, "Mark": c.Mark},
				})
				continue
			}
			style, ok := subMarkStyles[c.Mark]
			if !ok {
				style = defaultMarkStyle(fullTheme, c.Mark)
				subMarkStyles[c.Mark] = style
			}
			cell, err := buildTableSubMarkCell(c.Mark, series, style)
			if err != nil {
				return nil, nil, err
			}
			if cell == nil {
				continue
			}
			if row.Cells == nil {
				row.Cells = make(map[string]*scene.TableCell, 1)
			}
			row.Cells[c.Field] = cell
		}
		rows = append(rows, row)
	}

	return &scene.Table{Columns: cols, Rows: rows, PageSize: pageSize}, warnings, nil
}

// buildTableSubMarkCell encodes one row's numeric series as a
// standalone sub-mark (e.g. "sparkline") — the same encode/marks
// dispatch a top-level chart of that mark type would use — over a
// synthetic two-column table (an index field "i" and the series
// field "v"). Returns (nil, nil) for an empty series (nothing to
// draw, not an error).
func buildTableSubMarkCell(markType string, series []float64, style scene.Style) (*scene.TableCell, error) {
	n := len(series)
	if n == 0 {
		return nil, nil
	}

	idxCol := make(table.FloatColumn, n)
	valCol := make(table.FloatColumn, n)
	idxVals := make([]any, n)
	valVals := make([]any, n)
	for i, v := range series {
		idxCol[i] = float64(i)
		valCol[i] = v
		idxVals[i] = float64(i)
		valVals[i] = v
	}
	cellSchema := &table.Schema{Fields: []table.Field{
		{Name: "i", Type: table.FieldTypeF64},
		{Name: "v", Type: table.FieldTypeF64},
	}}
	cellTable, err := table.NewTable(cellSchema, map[string]table.Column{"i": idxCol, "v": valCol}, n, "table-cell")
	if err != nil {
		return nil, err
	}

	xScale, _, err := ResolveScale("quantitative", table.KindFloat, idxVals, 0, tableCellSparkWidth)
	if err != nil {
		return nil, err
	}
	yScale, _, err := ResolveScale("quantitative", table.KindFloat, valVals, tableCellSparkHeight, 0)
	if err != nil {
		return nil, err
	}

	inputs := marks.Inputs{
		Table:  cellTable,
		X:      marks.Channel{Field: "i", Scale: toMarkScale(xScale)},
		Y:      marks.Channel{Field: "v", Scale: toMarkScale(yScale)},
		Layout: scene.Rect{X: 0, Y: 0, W: tableCellSparkWidth, H: tableCellSparkHeight},
		Style:  style,
	}
	markList, _, err := marks.Encode(markType, inputs)
	if err != nil {
		return nil, err
	}
	return &scene.TableCell{Marks: markList}, nil
}

// parseNumericSeries coerces a table cell's raw value into a numeric
// series for a sub-mark column. Handles the shapes a materialised
// row's field can realistically carry:
//   - []float64 / []any of numbers (already a decoded JSON array)
//   - a JSON-encoded array string (table.FromInline's fallback
//     representation for a nested-array inline value — see
//     table/inline.go's stringify — since Prism's native table model
//     has no first-class array column type)
//
// Returns ok=false for anything else (nil, non-array string, etc.)
// so the caller can warn and skip the cell rather than failing the
// whole encode.
func parseNumericSeries(raw any) ([]float64, bool) {
	switch v := raw.(type) {
	case []float64:
		return v, true
	case []any:
		out := make([]float64, 0, len(v))
		for _, x := range v {
			f, ok := scale.ToFloat(x)
			if !ok {
				return nil, false
			}
			out = append(out, f)
		}
		return out, true
	case string:
		var arr []float64
		if err := json.Unmarshal([]byte(v), &arr); err != nil {
			return nil, false
		}
		return arr, true
	default:
		return nil, false
	}
}
