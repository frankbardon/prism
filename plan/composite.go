package plan

import (
	"github.com/frankbardon/prism/spec"
)

// CompositeKind discriminates the four composition primitives P08
// supports. Facet / repeat (P09) and nested composition (also P09)
// are not represented here.
type CompositeKind string

const (
	// CompositeLayer is `spec.Layer[]` — N layers in one chart with
	// (optional) cross-layer scale resolution.
	CompositeLayer CompositeKind = "layer"
	// CompositeConcat is `spec.Concat[]` — flat array of side-by-side
	// panels. In v1 this is functionally identical to CompositeHConcat
	// (D053); the `columns` wrap parameter lands in P09.
	CompositeConcat CompositeKind = "concat"
	// CompositeHConcat is `spec.HConcat[]` — 1 row × N cols.
	CompositeHConcat CompositeKind = "hconcat"
	// CompositeVConcat is `spec.VConcat[]` — N rows × 1 col.
	CompositeVConcat CompositeKind = "vconcat"
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
