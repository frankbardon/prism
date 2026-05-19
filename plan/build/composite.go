package build

import (
	"fmt"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
)

// BuildComposite translates a composite *spec.Spec (layer / concat /
// hconcat / vconcat) into a *plan.CompositeDAG. Each child becomes
// one ChildDAG with its own sub-plan + tip; the executor runs each
// child independently via plan.Execute and the encoder assembles a
// SceneDoc from the collected per-child tables.
//
// Per D050, nested composition (a child that is itself composite) is
// rejected with PRISM_PLAN_002 (Kind="composition:nested") in v1 and
// deferred to P09. Single-level composition is fully supported.
//
// Per D049, layer children inherit the parent's `datasets` block (and
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

	kind, children := compositeChildren(s)
	if len(children) == 0 {
		return nil, prismerrors.New(
			"PRISM_PLAN_002",
			fmt.Sprintf("Composite %s has no children.", kind),
			map[string]any{"Kind": fmt.Sprintf("composition:empty-%s", kind), "Phase": "P08"},
		)
	}

	rows, cols := compositeShape(kind, len(children))
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
		if IsComposite(child) {
			return nil, prismerrors.New(
				"PRISM_PLAN_002",
				fmt.Sprintf("Nested composition is not supported in v1 (child %d of %s is itself composite).", i, kind),
				map[string]any{"Kind": "composition:nested", "Phase": "P09"},
			)
		}

		merged := mergeParentDatasets(s, child)
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
// hconcat is 1×N; vconcat is N×1; concat is 1×N per D053.
func compositeShape(kind plan.CompositeKind, n int) (rows, cols int) {
	switch kind {
	case plan.CompositeLayer:
		return 1, 1
	case plan.CompositeVConcat:
		return n, 1
	case plan.CompositeHConcat, plan.CompositeConcat:
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
