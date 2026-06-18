package spec

// CrosstabTransform builds a contingency table. Wraps Pulse v0.13's
// Request.Crosstab section: the engine composes the cell aggregation
// across the row × column grouper grid, applies the configured
// normalisation, and returns long-form rows ready for a heatmap.
//
// Constraint: must be the first transform in the chain (or the only
// transform). Because Pulse has no in-memory cohort constructor,
// crosstab can only run against the source ref — not on the output
// of a prior transform. The plan builder enforces this via
// PRISM_PLAN_CROSSTAB_REQUIRES_SOURCE; the validate rule signals it
// statically via PRISM_SPEC_032.
type CrosstabTransform struct {
	Crosstab CrosstabBody `json:"crosstab"`
	Data     string       `json:"data,omitempty"`
	As       string       `json:"as,omitempty"`
}

// CrosstabBody is the body of the crosstab transform.
type CrosstabBody struct {
	// Rows is the list of grouper specs that form the row axis. v1
	// supports a single category grouper per axis (Field only).
	Rows []CrosstabGroup `json:"rows"`
	// Columns mirrors Rows for the column axis.
	Columns []CrosstabGroup `json:"columns"`
	// Cell is the cell aggregation. Required.
	Cell CrosstabCell `json:"cell"`
	// Margins toggles emission of row / column / grand-total rows.
	// Margin rows carry a `_margin` sentinel column so consumers can
	// filter them out for the body-only heatmap path.
	Margins *CrosstabMargins `json:"margins,omitempty"`
	// Normalize is one of "none" (default), "row", "column", "total".
	// Maps to pulse.CrosstabNormalize* directly.
	Normalize string `json:"normalize,omitempty"`
	// Shape is "matrix" or "long". Defaults to "long" (the heatmap-
	// friendly form). "matrix" returns the structured payload on
	// Response.Crosstab — reserved for future encoders that consume
	// the dense matrix.
	Shape string `json:"shape,omitempty"`
	// Overlays attaches Pulse post-result overlay layers to the cell
	// grid. Each overlay adds one F64 column to the long-form output,
	// index-aligned to the base cell, so it can drive a `color` /
	// `opacity` channel. When any overlay is present the node runs the
	// crosstab in matrix shape internally (overlays decorate body
	// cells) — user `margins` flags are ignored for the visual output.
	Overlays []CrosstabOverlay `json:"overlays,omitempty"`
}

// CrosstabOverlay is one post-result overlay layer on the cell grid.
// v1 supports the cell-scoped, matrix-payload kinds that align
// one-to-one with heatmap cells: share_of_row, share_of_col, and
// index_vs_margin. Group/series-scoped kinds (index_vs_total,
// share_of_total) land in a follow-up.
type CrosstabOverlay struct {
	// Kind is the friendly overlay name: share_of_row, share_of_col,
	// index_vs_margin.
	Kind string `json:"kind"`
	// Axis selects the margin axis ("row" or "column") for
	// index_vs_margin. Ignored by share_of_row/col (the axis is
	// implied by the kind).
	Axis string `json:"axis,omitempty"`
	// As is the output column name. Defaults to the kind.
	As string `json:"as,omitempty"`
}

// CrosstabGroup is one row / column grouper. Type defaults to
// "category" (GROUP_CATEGORY, Field only). Type "date" buckets a
// temporal field by calendar Period (GROUP_DATE) — the bucket keys are
// string labels ("2024", "2024-Q1", "2024-03", ...). Range / rounded /
// quantile groupers remain a follow-up.
type CrosstabGroup struct {
	Field string `json:"field"`
	Type  string `json:"type,omitempty"`
	// Period selects the calendar component for a date grouper: one of
	// year, quarter, month (default), week, day, day_of_week. Ignored
	// for category groupers.
	Period string `json:"period,omitempty"`
}

// CrosstabCell describes the per-cell aggregation. Reuses the
// AggregateOp field names so spec authors only learn one vocabulary.
type CrosstabCell struct {
	Aggregate string `json:"aggregate"`
	Field     string `json:"field,omitempty"`
	As        string `json:"as,omitempty"`
}

// CrosstabMargins toggles emission of row / column / grand-total rows.
type CrosstabMargins struct {
	Rows    bool `json:"rows,omitempty"`
	Columns bool `json:"columns,omitempty"`
	Grand   bool `json:"grand,omitempty"`
}
