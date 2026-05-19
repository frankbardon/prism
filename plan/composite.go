package plan

import (
	"github.com/frankbardon/prism/spec"
)

// CompositeKind discriminates the six composition primitives Prism
// supports as of P09. Selection-rooted composition stays deferred to
// P13.
type CompositeKind string

const (
	// CompositeLayer is `spec.Layer[]` — N layers in one chart with
	// (optional) cross-layer scale resolution.
	CompositeLayer CompositeKind = "layer"
	// CompositeConcat is `spec.Concat[]` — flat array of side-by-side
	// panels. In v1 this is functionally identical to CompositeHConcat
	// (D053); the `columns` wrap parameter lands in a future phase.
	CompositeConcat CompositeKind = "concat"
	// CompositeHConcat is `spec.HConcat[]` — 1 row × N cols.
	CompositeHConcat CompositeKind = "hconcat"
	// CompositeVConcat is `spec.VConcat[]` — N rows × 1 col.
	CompositeVConcat CompositeKind = "vconcat"
	// CompositeFacet is `spec.Facet{row, column}` — small multiples
	// driven by distinct values of the row / column field(s) (D054).
	// The builder returns a single shared sub-DAG (the parent's
	// pipeline); the encoder partitions the resulting Table by
	// `(row_value, col_value)` tuples and emits one SceneCell per
	// partition. `len(Children) == 1` is the convention "single
	// pipeline, encoder fans out" for facet.
	CompositeFacet CompositeKind = "facet"
	// CompositeRepeat is `spec.Repeat{row, column}` — small multiples
	// driven by a field-list. Each cell substitutes its field name
	// into the child spec and builds an independent sub-DAG (D056),
	// so `len(Children) == rows * cols` for repeat.
	CompositeRepeat CompositeKind = "repeat"
)

// ChildDAG carries one child's plan + the encoder-facing spec. The
// spec is forwarded so the encoder can read the child's mark,
// encoding, title etc. without re-walking the parent.
type ChildDAG struct {
	// DAG is the sub-plan for this child (one layer or one panel).
	DAG *DAG
	// Tip is the sub-plan's sole sink — the table the encoder reads.
	Tip NodeID
	// Spec is the merged child spec (parent datasets / data already
	// folded in by BuildComposite).
	Spec *spec.Spec
}

// CompositeDAG is the plan-stage representation of a composite spec
// (layer / concat / hconcat / vconcat). Per D049 + D050 each child
// owns its own sub-DAG; the executor handles each child
// independently via the existing plan.Execute entry point.
//
// Layout metadata (Rows, Cols) is normalised by BuildComposite based
// on the composition kind:
//   - Layer:   Rows=1, Cols=1 (cells flatten into one Scene's layers).
//   - HConcat: Rows=1, Cols=len(Children).
//   - VConcat: Rows=len(Children), Cols=1.
//   - Concat:  treated as HConcat in v1 (D053).
//   - Facet:   Rows=0, Cols=0 placeholders (D054); the encoder fills
//     in concrete dimensions after partitioning the upstream table.
//   - Repeat:  Rows=len(repeat.Row) or 1, Cols=len(repeat.Column) or
//     1 (D056); per-cell sub-DAGs land in Children in row-major order.
//
// Resolve carries cross-layer scale / axis resolution and is only
// meaningful when Kind == CompositeLayer; concat ignores it (cross-
// panel shared scales arrive with facet in P09).
type CompositeDAG struct {
	Kind     CompositeKind
	Rows     int
	Cols     int
	Children []ChildDAG
	Resolve  *spec.Resolve
}
