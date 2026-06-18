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
}

// FilterTransform: row predicate.
type FilterTransform struct {
	Filter string `json:"filter"`
	Data   string `json:"data,omitempty"`
	As     string `json:"as,omitempty"`
}

// CalculateTransform: compute new column.
type CalculateTransform struct {
	Calculate string `json:"calculate"`
	As        string `json:"as"`
	Data      string `json:"data,omitempty"`
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
