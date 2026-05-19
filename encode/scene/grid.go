package scene

// SceneGrid wraps one or more Scenes in an N×M layout. Flat charts
// use a 1×1 grid; composition (P08+) populates more cells.
type SceneGrid struct {
	Layout GridLayout  `json:"layout"`
	Cells  []SceneCell `json:"cells"`
	Shared SharedAxes  `json:"shared,omitempty"`
}

// GridLayout carries the grid's dimensions and per-cell sizing.
type GridLayout struct {
	Rows     int         `json:"rows"`
	Cols     int         `json:"cols"`
	GapPx    int         `json:"gap_px,omitempty"`
	RowSizes []float64   `json:"row_sizes,omitempty"`
	ColSizes []float64   `json:"col_sizes,omitempty"`
	Headers  GridHeaders `json:"headers,omitempty"`
}

// GridHeaders carries header/title rows or columns for facet grids.
type GridHeaders struct {
	Top    []string `json:"top,omitempty"`
	Left   []string `json:"left,omitempty"`
	Right  []string `json:"right,omitempty"`
	Bottom []string `json:"bottom,omitempty"`
}

// SceneCell is one Scene plus its row/col span in the grid.
type SceneCell struct {
	Row     int   `json:"row"`
	Col     int   `json:"col"`
	RowSpan int   `json:"row_span,omitempty"`
	ColSpan int   `json:"col_span,omitempty"`
	Scene   Scene `json:"scene"`
}

// SharedAxes carries grid-level axes that are shared across cells.
// nil = per-cell axes.
type SharedAxes struct {
	X *Axis `json:"x,omitempty"`
	Y *Axis `json:"y,omitempty"`
}
