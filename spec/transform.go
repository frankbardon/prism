package spec

// Transform is the discriminated transform union. Exactly one variant
// pointer is non-nil after unmarshal. UnmarshalJSON is implemented in
// transform_union.go (T01.14).
type Transform struct {
	Filter     *FilterTransform
	Calculate  *CalculateTransform
	Aggregate  *AggregateTransform
	Bin        *BinTransform
	Window     *WindowTransform
	Join       *JoinTransform
	Union      *UnionTransform
	Pivot      *PivotTransform
	Unpivot    *UnpivotTransform
	Sample     *SampleTransform
	Sort       *SortTransform
	Limit      *LimitTransform
	Crosstab   *CrosstabTransform
	Regression *RegressionTransform
	TimeUnit   *TimeUnitTransform
	Stack      *StackTransform
}

// FilterTransform: structured row predicate (E2-S1). The `filter` value
// is a Predicate tree, not a free-form expression string.
type FilterTransform struct {
	Filter Predicate `json:"filter"`
	Data   string    `json:"data,omitempty"`
	As     string    `json:"as,omitempty"`
}

// CalculateTransform: compute a new column from a structured derived-
// column expression (E2-S2). The `calculate` value is a CalcExpr tree,
// not a free-form expression string.
type CalculateTransform struct {
	Calculate CalcExpr `json:"calculate"`
	As        string   `json:"as"`
	Data      string   `json:"data,omitempty"`
}

// AggregateTransform: group-by aggregate.
type AggregateTransform struct {
	Aggregate []AggregateOp `json:"aggregate"`
	Groupby   []string      `json:"groupby,omitempty"`
	Data      string        `json:"data,omitempty"`
	As        string        `json:"as,omitempty"`
}

// AggregateOp is one aggregate calculation.
type AggregateOp struct {
	Op    string `json:"op"`
	Field string `json:"field,omitempty"`
	As    string `json:"as"`
}

// BinSpec is either a bool (auto) or an object with bin params.
type BinSpec struct {
	Auto    *bool
	Maxbins *int      `json:"maxbins,omitempty"`
	Step    *float64  `json:"step,omitempty"`
	Extent  []float64 `json:"extent,omitempty"`
}

// BinTransform: numeric bin.
type BinTransform struct {
	Bin   any    `json:"bin"`
	Field string `json:"field"`
	As    string `json:"as"`
	Data  string `json:"data,omitempty"`
}

// StackTransform: cumulative stacking (E5-S2).
//
// Computes two new columns holding the lower and upper bound of each
// row's segment within its stack, so a bar / area mark can draw the
// segment as a span rather than from the axis baseline. `stack` names
// the quantitative field to accumulate; `groupby` names the fields
// that define one stack (typically the dimension position channel);
// `offset` selects the accumulation mode.
//
// There is intentionally no `sort` key: `sort` is itself a transform
// discriminator, so a transform object carrying both would be rejected
// as ambiguous at decode. Order the segments with a preceding
// `{"sort": …}` transform — the stack preserves upstream row order
// inside each group.
//
// `as` is the [start, end] output column pair, NOT a dataset alias —
// like BinTransform.As and CalculateTransform.As it is never published
// to `leafByName`. Defaults to ["<field>_start", "<field>_end"].
type StackTransform struct {
	Stack   string   `json:"stack"`
	Groupby []string `json:"groupby,omitempty"`
	Offset  string   `json:"offset,omitempty"`
	As      []string `json:"as,omitempty"`
	Data    string   `json:"data,omitempty"`
}

// WindowTransform: windowed aggregate / rank.
type WindowTransform struct {
	Window      []WindowOp     `json:"window"`
	Partitionby []string       `json:"partitionby,omitempty"`
	Sort        []SortFieldDef `json:"sort,omitempty"`
	Frame       []any          `json:"frame,omitempty"`
	Data        string         `json:"data,omitempty"`
	As          string         `json:"as,omitempty"`
}

// WindowOp is one window operation.
type WindowOp struct {
	Op    string   `json:"op"`
	Field string   `json:"field,omitempty"`
	As    string   `json:"as"`
	Param *float64 `json:"param,omitempty"`
}

// SortFieldDef is a per-field sort entry.
type SortFieldDef struct {
	Field string `json:"field"`
	Order string `json:"order,omitempty"`
}

// JoinTransform: equality join.
type JoinTransform struct {
	Join string `json:"join"`
	With string `json:"with"`
	On   any    `json:"on"`
	Data string `json:"data,omitempty"`
	As   string `json:"as,omitempty"`
}

// UnionTransform: vertical concatenation.
type UnionTransform struct {
	Union []string `json:"union"`
	Data  string   `json:"data,omitempty"`
	As    string   `json:"as,omitempty"`
}

// PivotTransform: long → wide.
type PivotTransform struct {
	Pivot   string   `json:"pivot"`
	Value   string   `json:"value"`
	Groupby []string `json:"groupby,omitempty"`
	Op      string   `json:"op,omitempty"`
	Data    string   `json:"data,omitempty"`
	As      string   `json:"as,omitempty"`
}

// UnpivotTransform: wide → long.
type UnpivotTransform struct {
	Unpivot []string `json:"unpivot"`
	As      []string `json:"as,omitempty"`
	Data    string   `json:"data,omitempty"`
}

// SampleTransform: random subsample.
type SampleTransform struct {
	Sample int    `json:"sample"`
	Seed   *int64 `json:"seed,omitempty"`
	Data   string `json:"data,omitempty"`
	As     string `json:"as,omitempty"`
}

// SortTransform: order rows by fields.
type SortTransform struct {
	Sort []SortFieldDef `json:"sort"`
	Data string         `json:"data,omitempty"`
	As   string         `json:"as,omitempty"`
}

// LimitTransform: head with optional offset.
type LimitTransform struct {
	Limit  int    `json:"limit"`
	Offset *int   `json:"offset,omitempty"`
	Data   string `json:"data,omitempty"`
	As     string `json:"as,omitempty"`
}
