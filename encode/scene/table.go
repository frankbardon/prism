package scene

// TableCellSparkWidth / TableCellSparkHeight are the pixel dimensions
// of the coordinate space a table cell's sub-mark (e.g. a "sparkline"
// column) is encoded against — see encode/table.go's
// buildTableSubMarkCell, which resolves the sub-mark's X/Y scales
// against exactly this box. A render backend that re-wraps
// TableCell.Marks in its own SceneDoc to emit standalone geometry for
// one cell (E1-S4's inline-<svg>-per-cell HTML path) must size that
// wrapper Scene's Frame/Plot to match, or the sub-mark's
// already-resolved coordinates land outside the visible viewBox.
const (
	TableCellSparkWidth  = 120.0
	TableCellSparkHeight = 24.0
)

// Table is the Scene IR node for a table mark (E1). Unlike Mark, a
// Table carries no positioned geometry — its rows and columns render
// as DOM/CSS markup downstream (E1-S4), not SVG primitives. It hangs
// off Scene.Table as a sibling to Layers rather than a Mark geometry
// variant, so Mark.Validate()'s "exactly one geometry pointer"
// invariant is untouched. Additive to the Scene IR stability
// contract — no existing node type changes shape.
type Table struct {
	// Columns is the resolved column list, in declaration order
	// (mirrors spec.Encoding.Columns).
	Columns []TableColumn `json:"columns"`
	// Rows is the resolved row list — already filtered / sorted /
	// limited / aggregated by whatever transform chain fed the table
	// mark; encode never re-derives that here, it only reads the
	// upstream table the standard plan/compile pipeline produced.
	Rows []TableRow `json:"rows"`
	// PageSize is the number of rows a render backend should show
	// per page. Mirrors spec.MarkDef.PageSize
	// (spec.TablePageSizeDefault when the spec left it unset).
	PageSize int `json:"page_size,omitempty"`
}

// TableColumn is one resolved column definition: the field it reads
// from the upstream table, the header label to display, and — when
// the spec bound a sub-mark (e.g. "sparkline") — the mark type each
// row's cell for this column renders as, instead of formatted text.
type TableColumn struct {
	Field  string `json:"field"`
	Header string `json:"header"`
	// Mark names the sub-mark rendering this column's cells (e.g.
	// "sparkline"). Empty means the column renders as formatted
	// text — see TableRow.Values for the plain scalar in that case.
	Mark string `json:"mark,omitempty"`
}

// TableRow is one data row. ID is the row's ordinal index in the
// resolved upstream table (stable for a single encode call; a
// render/client-side sort reorders how rows are displayed, not this
// id). Values carries the plain scalar value for every column keyed
// by field name — the default text-rendering path, and also the
// value a client-side sort should read regardless of how the column
// visually renders (FR-11). Cells carries, for columns bound to a
// sub-mark, that column's own encoded Scene IR subtree for this row
// (e.g. a sparkline column's per-row line geometry), keyed by the
// same field name; a row with no sub-mark columns leaves Cells nil.
type TableRow struct {
	ID     int64                 `json:"id"`
	Values map[string]any        `json:"values"`
	Cells  map[string]*TableCell `json:"cells,omitempty"`
}

// TableCell carries one sub-mark's encoded Scene IR for a single
// row/column intersection — e.g. a sparkline column's per-row line
// geometry. Marks reuses the same Mark type every standalone mark
// encoder produces, encoded the same way encode/ would encode that
// sub-mark standalone, so a render backend can walk/emit it with the
// existing per-mark drawing code (see render/svg's mark emitters).
type TableCell struct {
	Marks []Mark `json:"marks"`
}
