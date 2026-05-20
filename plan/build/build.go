// Package build translates a *spec.Spec into a *plan.DAG.
//
// The package lives separately from `plan` because the translator
// imports `plan/nodes` (which already imports `plan` itself) — Go
// disallows the resulting import cycle. Splitting Build into its own
// package keeps `plan` dependency-free of the concrete node
// implementations while still giving callers a single entry point.
//
// Translation rules:
//
//   - Each declared dataset (top-level `data` + each `datasets[name]`)
//     becomes one leaf node: SourceNode for `data.source`, InlineNode
//     for `data.values`. Name-only datasets register as aliases for
//     the already-registered dataset they point at.
//   - Each transform[] entry becomes one node, chained on top of the
//     previous transform's output (or the active dataset's leaf).
//   - If the leaf encoding declares an aggregate on any channel, a
//     synthetic GroupAggregateNode is injected at the tail.
//   - The tail node is marked as the DAG's sole sink and its id is
//     returned alongside the DAG so the encode stage knows which
//     table to consume. P03's synthetic SinkNode (D030) was retired
//     in P05 — see D040.
//
// Out of scope for P03 (raise PRISM_PLAN_002):
//
//   - Composition primitives (layer, concat, hconcat, vconcat, facet,
//     repeat) — P08/P09.
//   - Selections — P13.
//
// Validator-gate failures (PRISM_SPEC_*) are NOT re-checked here; the
// builder trusts that callers have already validated the spec. The
// builder only emits codes that belong to plan-time concerns:
// PRISM_PLAN_001 (cycle, via TopoLevels), PRISM_PLAN_002 (out of
// scope feature), PRISM_PLAN_003 (missing dataset reference).
package build

import (
	"fmt"
	"strconv"

	"github.com/spf13/afero"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// Options carries the runtime knobs the builder needs to build leaf
// nodes (Source needs an afero.Fs + Resolver) and to wire the compile
// backend that every linear node's Execute routes through. FS
// defaults to afero.NewOsFs(); Resolver defaults to resolve.New(nil);
// Backend is left nil so callers that don't care about execution
// (e.g. `prism plan`) skip the dependency on compile/. The CLI
// `execute` subcommand passes `inmem.New()`. See D033.
//
// DatasetRegistry (P07, optional) resolves name-only dataset references
// like `{"data": {"name": "current"}}` through the server-side / env-
// loaded alias map (see resolve/registry_dataset.go). Nil = no
// registry; the builder behaviour reverts to P03–P06 (name-only ref
// raises PRISM_PLAN_003 when the alias is undeclared inline).
type Options struct {
	FS              afero.Fs
	Resolver        resolve.Resolver
	Backend         plan.Backend
	DatasetRegistry resolve.DatasetRegistry
}

// Build translates a *spec.Spec into a *plan.DAG. The returned
// NodeID is the tip — the table the encode stage consumes. The
// builder marks this node as the DAG's sole sink so existing
// renderers (DOT / JSON / text) and the executor continue to find
// it via DAG.Sinks() without a code change.
func Build(s *spec.Spec, opts Options) (*plan.DAG, plan.NodeID, error) {
	if s == nil {
		return nil, "", fmt.Errorf("plan/build: nil spec")
	}
	if err := rejectOutOfScope(s); err != nil {
		return nil, "", err
	}
	if opts.FS == nil {
		opts.FS = afero.NewOsFs()
	}
	if opts.Resolver == nil {
		opts.Resolver = resolve.New(nil)
	}

	b := plan.NewBuilder()
	ctx := newBuildCtx(b, opts)

	// Register every declared dataset as a leaf. The top-level data
	// gets registered first so its leaf wins the "active" slot even
	// if a sibling named dataset has the same source ref.
	if s.Data != nil {
		if err := ctx.registerTopLevel(s.Data); err != nil {
			return nil, "", err
		}
	}
	for name, ds := range s.Datasets {
		if err := ctx.registerDataset(name, ds); err != nil {
			return nil, "", err
		}
	}

	// Resolve the active leaf — the one the top-level transform chain
	// consumes. The spec's top-level data wins; absent that, fall
	// back to the first registered dataset (deterministic by name).
	active, err := ctx.activeLeaf(s)
	if err != nil {
		return nil, "", err
	}

	// Walk the transform chain.
	tip, err := ctx.applyTransforms(active, s.Transform)
	if err != nil {
		return nil, "", err
	}

	// If the encoding declares an aggregate, inject a GroupAggregate.
	tip, err = ctx.injectEncodingAggregate(tip, s.Encoding)
	if err != nil {
		return nil, "", err
	}

	// Mark the tip as the DAG's sole sink. Encode stage consumes the
	// table at this id; renderers locate it via DAG.Sinks(). D040
	// retired the synthetic SinkNode that P03 wired here.
	if err := b.MarkSink(tip); err != nil {
		return nil, "", err
	}
	dag, err := b.Build()
	if err != nil {
		return nil, "", err
	}
	return dag, tip, nil
}

// rejectOutOfScope walks the spec for features the builder cannot yet
// represent and emits PRISM_PLAN_002 pointing at the landing phase.
//
// P08 landed layer + concat / hconcat / vconcat; P09 added facet +
// repeat. Callers building a composite spec must use BuildComposite
// (the rejection here points them at the right entry). P13 wires
// Selection so the planner pipes the block straight through to the
// encoder (no DAG nodes are required — selections are pure encoder /
// renderer state per D004).
func rejectOutOfScope(s *spec.Spec) error {
	switch {
	case len(s.Layer) > 0, len(s.Concat) > 0, len(s.HConcat) > 0, len(s.VConcat) > 0,
		s.Facet != nil, s.Repeat != nil:
		return prismerrors.New(
			"PRISM_PLAN_002",
			"Spec is a composite (layer / concat / hconcat / vconcat / facet / repeat); use BuildComposite, not Build.",
			map[string]any{"Kind": "composition:flat-build", "Phase": "P08-P09"},
		)
	}
	return nil
}

// IsComposite reports whether s carries any of the six composition
// primitives (layer / concat / hconcat / vconcat / facet / repeat).
// Callers route composite specs through BuildComposite; flat specs
// continue through Build.
func IsComposite(s *spec.Spec) bool {
	if s == nil {
		return false
	}
	return len(s.Layer) > 0 ||
		len(s.Concat) > 0 ||
		len(s.HConcat) > 0 ||
		len(s.VConcat) > 0 ||
		s.Facet != nil ||
		s.Repeat != nil
}

func outOfScopeErr(kind, phase string) error {
	return prismerrors.New(
		"PRISM_PLAN_002",
		fmt.Sprintf("Unknown or unsupported plan kind %s (deferred to %s).", kind, phase),
		map[string]any{"Kind": kind, "Phase": phase},
	)
}

// buildCtx carries the per-Build state through the helpers.
type buildCtx struct {
	b            *plan.Builder
	opts         Options
	leafByName   map[string]plan.NodeID // dataset name → leaf node id
	leafBySource map[string]plan.NodeID // source ref → SourceNode id (dedupe)
	topLeaf      plan.NodeID            // the top-level (anonymous) leaf, if any
	counter      int
}

func newBuildCtx(b *plan.Builder, opts Options) *buildCtx {
	return &buildCtx{
		b:            b,
		opts:         opts,
		leafByName:   map[string]plan.NodeID{},
		leafBySource: map[string]plan.NodeID{},
	}
}

func (c *buildCtx) nextID(prefix string) plan.NodeID {
	c.counter++
	return plan.NodeID(prefix + ":" + strconv.Itoa(c.counter))
}

// registerTopLevel registers s.Data and records its leaf id as the
// default active leaf for the top-level transform chain. Pure wrapper
// around registerDataset that captures the leaf id post-registration.
func (c *buildCtx) registerTopLevel(ds *spec.Data) error {
	before := snapshot(c)
	if err := c.registerDataset(ds.Name, ds); err != nil {
		return err
	}
	// Figure out which leaf was newly added. New SourceNodes appear
	// in leafBySource; new InlineNodes appear in leafByName under
	// ds.Name when non-empty, otherwise the most-recently-created
	// counter slot.
	if ds.Source != "" {
		if id, ok := c.leafBySource[ds.Source]; ok {
			c.topLeaf = id
			return nil
		}
	}
	if ds.Name != "" {
		if id, ok := c.leafByName[ds.Name]; ok {
			c.topLeaf = id
			return nil
		}
	}
	// Anonymous inline. The newest leaf is the one not in `before`.
	for id := range c.b.NodeIDs() {
		if _, was := before.ids[id]; !was {
			c.topLeaf = id
			return nil
		}
	}
	return nil
}

// snapshot captures the current builder's node ids so registerTopLevel
// can identify the newly added leaf for anonymous inline datasets.
type buildSnapshot struct {
	ids map[plan.NodeID]struct{}
}

func snapshot(c *buildCtx) buildSnapshot {
	out := buildSnapshot{ids: map[plan.NodeID]struct{}{}}
	for id := range c.b.NodeIDs() {
		out.ids[id] = struct{}{}
	}
	return out
}

// registerDataset turns one *spec.Data into a leaf node and records
// its id under name (if non-empty). Source refs are de-duped: two
// datasets pointing at the same .pulse share one SourceNode. The
// dataset registry (when configured) is consulted for name-only
// references before the empty-source path is taken; this lets specs
// say `{"data": {"name": "current"}}` and resolve via the
// server-side registry.
//
// Duplicate alias detection (P07): a non-empty `name` that already
// maps to a different leaf id raises PRISM_RESOLVE_DUPLICATE_DATASET.
// Aliasing the same physical leaf twice is a no-op; aliasing two
// different leaves under one name is a programmer error.
func (c *buildCtx) registerDataset(name string, ds *spec.Data) error {
	if ds == nil {
		return nil
	}
	// Resolve name-only references through the dataset registry so the
	// downstream Source path can build a real leaf. Mutates a local
	// copy; the caller's *spec.Data is untouched.
	if ds.Source == "" && ds.Name != "" && c.opts.DatasetRegistry != nil {
		if path, ok := c.opts.DatasetRegistry.Resolve(ds.Name); ok {
			dsCopy := *ds
			dsCopy.Source = path
			ds = &dsCopy
		}
	}
	switch {
	case ds.Source != "":
		if id, ok := c.leafBySource[ds.Source]; ok {
			if name != "" {
				if err := c.bindAlias(name, id); err != nil {
					return err
				}
			}
			return nil
		}
		src := nodes.New(ds.Source, c.opts.FS, c.opts.Resolver)
		if err := c.b.AddNode(src); err != nil {
			return err
		}
		if err := c.b.MarkRoot(src.ID()); err != nil {
			return err
		}
		c.leafBySource[ds.Source] = src.ID()
		if name != "" {
			if err := c.bindAlias(name, src.ID()); err != nil {
				return err
			}
		}
		if ds.Name != "" && ds.Name != name {
			if err := c.bindAlias(ds.Name, src.ID()); err != nil {
				return err
			}
		}
		return nil
	case len(ds.Values) > 0 || len(ds.Fields) > 0:
		id := c.nextID("inline")
		in := nodes.NewInline(id, name, ds.Values, ds.Fields)
		if err := c.b.AddNode(in); err != nil {
			return err
		}
		if err := c.b.MarkRoot(id); err != nil {
			return err
		}
		if name != "" {
			if err := c.bindAlias(name, id); err != nil {
				return err
			}
		}
		if ds.Name != "" && ds.Name != name {
			if err := c.bindAlias(ds.Name, id); err != nil {
				return err
			}
		}
		return nil
	case ds.Name != "":
		// Name-only reference: alias for an already-registered dataset.
		if name != "" && name != ds.Name {
			if id, ok := c.leafByName[ds.Name]; ok {
				if err := c.bindAlias(name, id); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return nil
}

// bindAlias publishes alias → id under c.leafByName. Binding the same
// alias to the same id is idempotent (allows the top-level data + a
// dataset map entry to share the same alias). Binding the alias to a
// different id raises PRISM_RESOLVE_DUPLICATE_DATASET.
func (c *buildCtx) bindAlias(alias string, id plan.NodeID) error {
	if existing, ok := c.leafByName[alias]; ok && existing != id {
		return prismerrors.New(
			"PRISM_RESOLVE_DUPLICATE_DATASET",
			fmt.Sprintf("Dataset alias %q is declared more than once (first at %s, again at %s).",
				alias, existing, id),
			map[string]any{"Alias": alias, "First": string(existing), "Second": string(id)},
		)
	}
	c.leafByName[alias] = id
	return nil
}

// activeLeaf picks the leaf the top-level transform chain consumes.
// Preference order: top-level leaf recorded at registerTopLevel time
// (handles anonymous inline data.values), top-level data.name alias,
// first named dataset alphabetically.
func (c *buildCtx) activeLeaf(s *spec.Spec) (plan.NodeID, error) {
	if c.topLeaf != "" {
		return c.topLeaf, nil
	}
	if s.Data != nil {
		if id, ok := c.lookupLeaf(s.Data); ok {
			return id, nil
		}
		if s.Data.Name != "" {
			if id, ok := c.leafByName[s.Data.Name]; ok {
				return id, nil
			}
			return "", c.missingDatasetErr(s.Data.Name)
		}
	}
	if len(c.leafByName) > 0 {
		var minName string
		for name := range c.leafByName {
			if minName == "" || name < minName {
				minName = name
			}
		}
		return c.leafByName[minName], nil
	}
	return "", fmt.Errorf("plan/build: spec has no data binding")
}

// lookupLeaf resolves a *spec.Data to its registered leaf id.
func (c *buildCtx) lookupLeaf(ds *spec.Data) (plan.NodeID, bool) {
	if ds.Source != "" {
		id, ok := c.leafBySource[ds.Source]
		return id, ok
	}
	if ds.Name != "" {
		id, ok := c.leafByName[ds.Name]
		return id, ok
	}
	return "", false
}

// applyTransforms walks the transform chain rooted at startID and
// returns the id of the tail node. Unknown transform variants raise
// PRISM_PLAN_002; missing data references raise PRISM_PLAN_003.
//
// Transform.As publishing (P07): each transform variant exposes an
// optional `as` field. After the corresponding node is constructed, the
// builder registers the new node id under that alias in leafByName so
// downstream transforms can reference it via `data: "<as>"`. Alias
// collision raises PRISM_RESOLVE_DUPLICATE_DATASET via bindAlias.
func (c *buildCtx) applyTransforms(startID plan.NodeID, ts []spec.Transform) (plan.NodeID, error) {
	tip := startID
	for _, t := range ts {
		nextTip, err := c.applyOneTransform(tip, t)
		if err != nil {
			return "", err
		}
		if alias := transformAsName(t); alias != "" {
			if err := c.bindAlias(alias, nextTip); err != nil {
				return "", err
			}
		}
		tip = nextTip
	}
	return tip, nil
}

// transformAsName returns the `as` field of whichever variant is set
// on t, or "" when the variant declares none.
func transformAsName(t spec.Transform) string {
	switch {
	case t.Filter != nil:
		return t.Filter.As
	case t.Calculate != nil:
		// Calculate.As is the *output column* name (always present),
		// not a dataset alias — do NOT publish.
		return ""
	case t.Aggregate != nil:
		return t.Aggregate.As
	case t.Bin != nil:
		// Bin.As is the output column name (always present), not an
		// alias.
		return ""
	case t.Window != nil:
		return t.Window.As
	case t.Join != nil:
		return t.Join.As
	case t.Union != nil:
		return t.Union.As
	case t.Pivot != nil:
		return t.Pivot.As
	case t.Sample != nil:
		return t.Sample.As
	case t.Sort != nil:
		return t.Sort.As
	case t.Limit != nil:
		return t.Limit.As
	}
	return ""
}

func (c *buildCtx) applyOneTransform(input plan.NodeID, t spec.Transform) (plan.NodeID, error) {
	resolveInput := func(dataRef string) (plan.NodeID, error) {
		if dataRef == "" {
			return input, nil
		}
		if id, ok := c.leafByName[dataRef]; ok {
			return id, nil
		}
		return "", c.missingDatasetErr(dataRef)
	}

	switch {
	case t.Filter != nil:
		in, err := resolveInput(t.Filter.Data)
		if err != nil {
			return "", err
		}
		id := c.nextID("filter")
		return c.addAndReturn(nodes.NewFilter(id, in, t.Filter.Filter))
	case t.Calculate != nil:
		in, err := resolveInput(t.Calculate.Data)
		if err != nil {
			return "", err
		}
		id := c.nextID("calc")
		return c.addAndReturn(nodes.NewCalculate(id, in, t.Calculate.Calculate, t.Calculate.As))
	case t.Aggregate != nil:
		in, err := resolveInput(t.Aggregate.Data)
		if err != nil {
			return "", err
		}
		id := c.nextID("ga")
		ops := make([]nodes.AggOp, len(t.Aggregate.Aggregate))
		for i, o := range t.Aggregate.Aggregate {
			ops[i] = nodes.AggOp{Op: o.Op, Field: o.Field, As: o.As}
		}
		return c.addAndReturn(nodes.NewGroupAggregate(id, in, t.Aggregate.Groupby, ops))
	case t.Bin != nil:
		in, err := resolveInput(t.Bin.Data)
		if err != nil {
			return "", err
		}
		id := c.nextID("bin")
		return c.addAndReturn(nodes.NewBin(id, in, t.Bin.Field, t.Bin.As, nodes.BinParams{Auto: true}))
	case t.Window != nil:
		in, err := resolveInput(t.Window.Data)
		if err != nil {
			return "", err
		}
		id := c.nextID("win")
		ops := make([]nodes.WindowOp, len(t.Window.Window))
		for i, o := range t.Window.Window {
			ops[i] = nodes.WindowOp{Op: o.Op, Field: o.Field, As: o.As, Param: o.Param}
		}
		sk := make([]nodes.SortKey, len(t.Window.Sort))
		for i, s := range t.Window.Sort {
			sk[i] = nodes.SortKey{Field: s.Field, Order: s.Order}
		}
		return c.addAndReturn(nodes.NewWindow(id, in, ops, t.Window.Partitionby, sk, t.Window.Frame))
	case t.Join != nil:
		left, err := resolveInput(t.Join.Data)
		if err != nil {
			return "", err
		}
		right, ok := c.leafByName[t.Join.With]
		if !ok {
			return "", c.missingDatasetErr(t.Join.With)
		}
		id := c.nextID("join")
		on := joinOnFields(t.Join.On)
		kind := nodes.JoinKind(t.Join.Join)
		if kind == "" {
			kind = nodes.JoinInner
		}
		return c.addAndReturn(nodes.NewJoin(id, left, right, on, kind, 0))
	case t.Union != nil:
		ins := make([]plan.NodeID, 0, len(t.Union.Union))
		for _, name := range t.Union.Union {
			id, ok := c.leafByName[name]
			if !ok {
				return "", c.missingDatasetErr(name)
			}
			ins = append(ins, id)
		}
		id := c.nextID("union")
		return c.addAndReturn(nodes.NewUnion(id, ins))
	case t.Pivot != nil:
		in, err := resolveInput(t.Pivot.Data)
		if err != nil {
			return "", err
		}
		id := c.nextID("pivot")
		return c.addAndReturn(nodes.NewPivot(id, in, t.Pivot.Pivot, t.Pivot.Value, t.Pivot.Groupby, t.Pivot.Op))
	case t.Unpivot != nil:
		in, err := resolveInput(t.Unpivot.Data)
		if err != nil {
			return "", err
		}
		id := c.nextID("unpivot")
		return c.addAndReturn(nodes.NewUnpivot(id, in, t.Unpivot.Unpivot, t.Unpivot.As))
	case t.Sample != nil:
		in, err := resolveInput(t.Sample.Data)
		if err != nil {
			return "", err
		}
		id := c.nextID("sample")
		return c.addAndReturn(nodes.NewSample(id, in, t.Sample.Sample, t.Sample.Seed))
	case t.Sort != nil:
		in, err := resolveInput(t.Sort.Data)
		if err != nil {
			return "", err
		}
		id := c.nextID("sort")
		sk := make([]nodes.SortKey, len(t.Sort.Sort))
		for i, s := range t.Sort.Sort {
			sk[i] = nodes.SortKey{Field: s.Field, Order: s.Order}
		}
		return c.addAndReturn(nodes.NewSort(id, in, sk))
	case t.Limit != nil:
		in, err := resolveInput(t.Limit.Data)
		if err != nil {
			return "", err
		}
		id := c.nextID("limit")
		off := 0
		if t.Limit.Offset != nil {
			off = *t.Limit.Offset
		}
		return c.addAndReturn(nodes.NewLimit(id, in, t.Limit.Limit, off))
	}
	return "", outOfScopeErr("transform:unknown", "P04")
}

// addAndReturn wraps b.AddNode + returns the new node's id. If the
// node implements a SetBackend(plan.Backend) hook AND the builder has
// a Backend configured, we wire it here so every linear node's
// Execute routes through the backend without callers having to
// remember to inject manually.
func (c *buildCtx) addAndReturn(n plan.Node) (plan.NodeID, error) {
	if err := c.b.AddNode(n); err != nil {
		return "", err
	}
	if c.opts.Backend != nil {
		if bw, ok := n.(backendWired); ok {
			bw.SetBackend(c.opts.Backend)
		}
	}
	return n.ID(), nil
}

// backendWired is the duck-typed interface every linear node
// satisfies (FilterNode, ProjectNode, etc.). Stub nodes that have
// not migrated (Join, Union, Pivot, Unpivot) do not satisfy it, so
// the type assertion skips them without effect.
type backendWired interface {
	SetBackend(plan.Backend)
}

// joinOnFields normalises the spec's polymorphic `on` value (string or
// []string) into a flat []string.
func joinOnFields(on any) []string {
	switch v := on.(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	}
	return nil
}

// injectEncodingAggregate looks at the encoding for any channel
// declaring an aggregate; if any does, append a GroupAggregateNode
// whose groupby = every non-aggregated field channel and whose aggs =
// every aggregated field channel.
func (c *buildCtx) injectEncodingAggregate(tip plan.NodeID, enc *spec.Encoding) (plan.NodeID, error) {
	if enc == nil {
		return tip, nil
	}
	type entry struct {
		field string
		agg   string
	}
	var entries []entry
	collectPos := func(ch *spec.PositionChannel) {
		if ch == nil || ch.Field == "" {
			return
		}
		entries = append(entries, entry{field: ch.Field, agg: ch.Aggregate})
	}
	collectMark := func(ch *spec.MarkChannel) {
		if ch == nil || ch.Field == "" {
			return
		}
		entries = append(entries, entry{field: ch.Field, agg: ch.Aggregate})
	}
	collectPos(enc.X)
	collectPos(enc.Y)
	collectPos(enc.X2)
	collectPos(enc.Y2)
	collectPos(enc.Theta)
	collectPos(enc.Radius)
	collectMark(enc.Color)
	collectMark(enc.Fill)
	collectMark(enc.Stroke)
	collectMark(enc.Opacity)
	collectMark(enc.Size)
	collectMark(enc.Shape)

	var hasAgg bool
	for _, e := range entries {
		if e.agg != "" {
			hasAgg = true
			break
		}
	}
	if !hasAgg {
		return tip, nil
	}

	var groupby []string
	var aggs []nodes.AggOp
	seen := map[string]bool{}
	for _, e := range entries {
		if e.agg == "" {
			if !seen[e.field] {
				groupby = append(groupby, e.field)
				seen[e.field] = true
			}
			continue
		}
		aggs = append(aggs, nodes.AggOp{Op: e.agg, Field: e.field, As: e.field})
	}
	id := c.nextID("enc-ga")
	return c.addAndReturn(nodes.NewGroupAggregate(id, tip, groupby, aggs))
}

// missingDatasetErr formats a PRISM_PLAN_003 with the available leaf
// names for an actionable diagnostic.
func (c *buildCtx) missingDatasetErr(name string) error {
	avail := ""
	first := true
	for k := range c.leafByName {
		if !first {
			avail += ", "
		}
		avail += k
		first = false
	}
	return prismerrors.New(
		"PRISM_PLAN_003",
		fmt.Sprintf("Transform references undeclared dataset %q (available: %s).", name, avail),
		map[string]any{"Dataset": name, "Available": avail},
	)
}
