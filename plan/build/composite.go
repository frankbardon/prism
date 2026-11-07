package build

import (
	"fmt"
	"strconv"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/vfs"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// BuildComposite translates a composite *spec.Spec (layer / concat /
// hconcat / vconcat / facet / repeat) into a *plan.CompositeDAG.
// Layer + concat / hconcat / vconcat have each child build its own
// sub-plan + tip (P08). Facet builds the parent's shared upstream
// pipeline once and returns a single ChildDAG (D054); the encoder
// fans out into per-partition SceneCells. Repeat builds one
// sub-plan per cell after applying the field-name substitution
// (D055 / D056). The executor runs every child independently via
// plan.Execute and the encoder assembles a SceneDoc from the
// collected per-child tables.
//
// Nested composition: a layer's children stay flat (D050). A concat /
// hconcat / vconcat child may itself be a layer or a concat (a column
// of layered panels, a wrapped grid of layers): the whole nested
// subtree is staged on one shared plan.Builder with namespaced node
// ids, so the child carries one DAG every caller executes as before,
// plus a Composite describing the subtree (buildNestedInto). Facet /
// repeat children may themselves be composite, and the encoder
// recurses through EncodeComposite per cell (D058).
//
// Per D049, children inherit the parent's `datasets` block (and
// top-level `data` when the child has none) via mergeParentDatasets;
// per-child `data` overrides win.
func BuildComposite(s *spec.Spec, opts Options) (*plan.CompositeDAG, error) {
	if s == nil {
		return nil, fmt.Errorf("plan/build: nil spec")
	}
	if !IsComposite(s) {
		return nil, prismerrors.New(
			"PRISM_PLAN_002",
			"Spec is not a composite; use Build for flat charts.",
			map[string]any{"Kind": "composition:flat-spec", "Phase": "P08"},
		)
	}

	// Facet + repeat have bespoke build paths because their children
	// are not literal array entries on the parent spec.
	if s.Facet != nil {
		return buildFacetComposite(s, opts)
	}
	if s.Repeat != nil {
		return buildRepeatComposite(s, opts)
	}

	kind, children := compositeChildren(s)
	if len(children) == 0 {
		return nil, prismerrors.New(
			"PRISM_PLAN_002",
			fmt.Sprintf("Composite %s has no children.", kind),
			map[string]any{"Kind": fmt.Sprintf("composition:empty-%s", kind), "Phase": "P08"},
		)
	}

	rows, cols := compositeShape(kind, len(children), s.Columns)
	out := &plan.CompositeDAG{
		Kind:    kind,
		Rows:    rows,
		Cols:    cols,
		Resolve: s.Resolve,
	}

	for i, child := range children {
		if child == nil {
			return nil, prismerrors.New(
				"PRISM_PLAN_002",
				fmt.Sprintf("Composite %s child %d is nil.", kind, i),
				map[string]any{"Kind": "composition:nil-child", "Phase": "P08"},
			)
		}
		merged := mergeParentDatasets(s, child)
		if IsComposite(child) {
			if err := nestable(kind, i, child); err != nil {
				return nil, err
			}
			nested, err := buildNested(merged, opts)
			if err != nil {
				return nil, err
			}
			out.Children = append(out.Children, nested)
			continue
		}
		dag, tip, err := Build(merged, opts)
		if err != nil {
			return nil, err
		}
		out.Children = append(out.Children, plan.ChildDAG{
			DAG:  dag,
			Tip:  tip,
			Spec: merged,
		})
	}

	return out, nil
}

// nestable reports whether child (composite, at index i of a kind
// composition) may nest there: a concat-family parent takes a layer or
// another concat-family child; a layer takes no composite child, and
// a facet / repeat never nests under a concat.
func nestable(kind plan.CompositeKind, i int, child *spec.Spec) error {
	concatParent := kind == plan.CompositeConcat || kind == plan.CompositeHConcat || kind == plan.CompositeVConcat
	if concatParent && child.Facet == nil && child.Repeat == nil {
		return nil
	}
	return prismerrors.New(
		"PRISM_PLAN_002",
		fmt.Sprintf("Nested composition is not supported for %s children (child %d is itself composite): a concat / hconcat / vconcat child may be a layer or a concat, a layer child must be flat.", kind, i),
		map[string]any{"Kind": "composition:nested", "Phase": "P10"},
	)
}

// buildNested plans a composite child of a concat. Every leaf of the
// child's subtree is staged on one Builder under its own id namespace,
// so the returned ChildDAG holds a single DAG with one sink per leaf:
// callers that execute child.DAG (the CLI, RPC, WASM and prism.Compile
// loops) get every leaf's table in one map and need no change.
// Composite mirrors the subtree; each of its ChildDAGs shares that DAG
// and names its own leaf's tip. Tip is the first leaf's, for callers
// that print one table per child.
func buildNested(s *spec.Spec, opts Options) (plan.ChildDAG, error) {
	if opts.FS == nil {
		opts.FS = vfs.OsFs()
	}
	if opts.Resolver == nil {
		opts.Resolver = resolve.New(nil)
	}
	b := plan.NewBuilder()
	sub, err := buildNestedInto(b, "", s, opts)
	if err != nil {
		return plan.ChildDAG{}, err
	}
	dag, err := b.Build()
	if err != nil {
		return plan.ChildDAG{}, err
	}
	shareDAG(sub, dag)
	return plan.ChildDAG{DAG: dag, Tip: firstTip(sub), Spec: s, Composite: sub}, nil
}

// buildNestedInto stages composite s's leaves on b, child i's under
// ns + "i/", and returns the subtree's CompositeDAG (DAGs unset until
// shareDAG).
func buildNestedInto(b *plan.Builder, ns string, s *spec.Spec, opts Options) (*plan.CompositeDAG, error) {
	kind, children := compositeChildren(s)
	if len(children) == 0 {
		return nil, prismerrors.New(
			"PRISM_PLAN_002",
			"Nested composition must be a layer or a concat with children.",
			map[string]any{"Kind": "composition:nested", "Phase": "P10"},
		)
	}
	rows, cols := compositeShape(kind, len(children), s.Columns)
	out := &plan.CompositeDAG{Kind: kind, Rows: rows, Cols: cols, Resolve: s.Resolve}
	for i, child := range children {
		if child == nil {
			return nil, prismerrors.New(
				"PRISM_PLAN_002",
				fmt.Sprintf("Composite %s child %d is nil.", kind, i),
				map[string]any{"Kind": "composition:nil-child", "Phase": "P08"},
			)
		}
		merged := mergeParentDatasets(s, child)
		childNS := ns + strconv.Itoa(i) + "/"
		if IsComposite(child) {
			if err := nestable(kind, i, child); err != nil {
				return nil, err
			}
			sub, err := buildNestedInto(b, childNS, merged, opts)
			if err != nil {
				return nil, err
			}
			out.Children = append(out.Children, plan.ChildDAG{Spec: merged, Composite: sub})
			continue
		}
		tip, err := buildInto(b, childNS, merged, opts)
		if err != nil {
			return nil, err
		}
		out.Children = append(out.Children, plan.ChildDAG{Tip: tip, Spec: merged})
	}
	return out, nil
}

// shareDAG points every ChildDAG of the subtree at the one DAG its
// leaves were staged on, and sets each nested child's Tip to its
// first leaf's.
func shareDAG(c *plan.CompositeDAG, dag *plan.DAG) {
	for i := range c.Children {
		ch := &c.Children[i]
		ch.DAG = dag
		if ch.Composite != nil {
			shareDAG(ch.Composite, dag)
			ch.Tip = firstTip(ch.Composite)
		}
	}
}

// firstTip is the tip of the subtree's first leaf, depth first.
func firstTip(c *plan.CompositeDAG) plan.NodeID {
	for _, ch := range c.Children {
		if ch.Composite != nil {
			if t := firstTip(ch.Composite); t != "" {
				return t
			}
			continue
		}
		if ch.Tip != "" {
			return ch.Tip
		}
	}
	return ""
}

// compositeChildren returns the (kind, children) pair for s. Exactly
// one of the four composition arrays is non-empty when IsComposite(s)
// is true; callers should have already gated.
func compositeChildren(s *spec.Spec) (plan.CompositeKind, []*spec.Spec) {
	switch {
	case len(s.Layer) > 0:
		return plan.CompositeLayer, s.Layer
	case len(s.HConcat) > 0:
		return plan.CompositeHConcat, s.HConcat
	case len(s.VConcat) > 0:
		return plan.CompositeVConcat, s.VConcat
	case len(s.Concat) > 0:
		// D053: concat treated as hconcat in v1.
		return plan.CompositeConcat, s.Concat
	}
	return "", nil
}

// compositeShape normalises rows × cols for the composition kind.
// Layer always renders into a single cell (one Scene with N layers);
// hconcat is 1×N; vconcat is N×1; concat is 1×N per D053 unless the
// spec sets `columns`, which wraps it into rows of that many cells
// (the last row may be short). The schema rejects `columns` without
// `concat` (dependentRequired); it never changes another shape.
func compositeShape(kind plan.CompositeKind, n int, columns *int) (rows, cols int) {
	switch kind {
	case plan.CompositeLayer:
		return 1, 1
	case plan.CompositeVConcat:
		return n, 1
	case plan.CompositeConcat:
		if columns != nil && *columns > 0 && *columns < n {
			c := *columns
			return (n + c - 1) / c, c
		}
		return 1, n
	case plan.CompositeHConcat:
		return 1, n
	}
	return 1, n
}

// mergeParentDatasets returns a shallow copy of child augmented with
// parent.Datasets entries the child does not already declare. The
// parent's top-level `data` is inherited only when the child has no
// `data` block of its own — per-child overrides win (design/04-multi-
// source.md). Transform chains, mark, encoding, title etc. are NOT
// inherited; each layer / panel is a self-contained chart.
func mergeParentDatasets(parent, child *spec.Spec) *spec.Spec {
	out := *child

	// Inherit datasets entries the child has not redeclared. Always
	// allocate a fresh map so the merge never mutates the child in
	// place.
	merged := map[string]*spec.Data{}
	for name, ds := range parent.Datasets {
		merged[name] = ds
	}
	for name, ds := range child.Datasets {
		merged[name] = ds
	}
	if len(merged) > 0 {
		out.Datasets = merged
	}

	// Inherit top-level data only when the child has none. Avoids the
	// surprising behaviour where a child that explicitly says
	// `"data": {"name": "x"}` gets silently swapped for the parent's.
	if out.Data == nil && parent.Data != nil {
		out.Data = parent.Data
	}

	// Inherit $schema so child spec validation downstream still finds it.
	if out.Schema == "" {
		out.Schema = parent.Schema
	}

	return &out
}
