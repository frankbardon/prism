package encode

import (
	"fmt"

	encresolve "github.com/frankbardon/prism/encode/resolve"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// encodeFacetComposite turns a facet CompositeDAG into a SceneDoc.
// Per D054 the builder returned exactly one ChildDAG carrying the
// shared upstream pipeline; this function partitions the resulting
// table by (row_value, col_value) tuples and emits one SceneCell per
// partition.
//
// Per D057 facet defaults resolve.scale.{x,y} to "shared" (matching
// Vega-Lite). Shared scales are computed across all partition tables
// once and applied to every cell via the existing flat Encode path
// with OverrideXScale / OverrideYScale hints; per-cell axes for the
// shared channels are stripped after the child Encode returns
// (D051: shared axes live once on SceneGrid.SharedAxes).
func encodeFacetComposite(s *spec.Spec, composite *plan.CompositeDAG, childTables []map[plan.NodeID]*table.Table, opts EncodeOpts) (*scene.SceneDoc, error) {
	if len(composite.Children) != 1 {
		return nil, fmt.Errorf("encode: facet composite expects exactly 1 ChildDAG (D054), got %d", len(composite.Children))
	}
	if s.Facet == nil {
		return nil, fmt.Errorf("encode: facet composite missing Facet block on parent spec")
	}
	child := composite.Children[0]
	upstream, ok := childTables[0][child.Tip]
	if !ok || upstream == nil {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Facet upstream table for tip %q absent from executor result.", child.Tip),
			map[string]any{"Field": string(child.Tip), "Source": "<executor>", "Available": joinNodeIDs(childTables[0])},
		)
	}

	rowField := facetField(s.Facet.Row)
	colField := facetField(s.Facet.Column)
	if rowField == "" && colField == "" {
		return nil, fmt.Errorf("encode: facet declares neither row nor column field")
	}

	partitions, err := partitionTable(upstream, rowField, colField)
	if err != nil {
		return nil, err
	}

	// Outer width/height and theme.
	outerW := opts.Width
	if outerW == 0 {
		outerW = 800
	}
	outerH := opts.Height
	if outerH == 0 {
		outerH = 600
	}
	sceneTheme, err := resolveTheme(opts, s.Theme)
	if err != nil {
		return nil, err
	}

	rows := len(partitions.RowValues)
	cols := len(partitions.ColValues)
	if rows == 0 {
		rows = 1
	}
	if cols == 0 {
		cols = 1
	}

	gap := 20.0
	// Reserve a fixed top strip for column headers + left strip for
	// row headers when those axes are populated. Keeps the cell grid
	// in a sensible space; v1 picks safe defaults.
	headerTop := 0.0
	if colField != "" {
		headerTop = 22.0
	}
	headerLeft := 0.0
	if rowField != "" {
		headerLeft = 60.0
	}
	cellW := (outerW - headerLeft - gap*float64(cols-1)) / float64(cols)
	cellH := (outerH - headerTop - gap*float64(rows-1)) / float64(rows)
	if cellW < 1 {
		cellW = outerW / float64(cols)
	}
	if cellH < 1 {
		cellH = outerH / float64(rows)
	}

	resolution := encresolve.FromSpec(composite.Resolve)
	// D057 default for facet: shared x/y unless the spec overrides.
	// FromSpec already returns shared for x/y when nothing overrides
	// (the package defaults), so we do nothing extra for the default
	// path.

	// Pre-compute shared scales when requested.
	cellLayout := Compute(cellW, cellH, false)
	xMode := resolution[scene.ChannelX]
	yMode := resolution[scene.ChannelY]

	var xShared Scale
	var yShared Scale
	if xMode.Scale == encresolve.ModeShared {
		xShared, err = buildSharedScaleForFacet(child.Spec, partitions, scene.ChannelX,
			cellLayout.Plot.X, cellLayout.Plot.Right())
		if err != nil {
			return nil, err
		}
	}
	if yMode.Scale == encresolve.ModeShared {
		yShared, err = buildSharedScaleForFacet(child.Spec, partitions, scene.ChannelY,
			cellLayout.Plot.Bottom(), cellLayout.Plot.Y)
		if err != nil {
			return nil, err
		}
	}

	// Walk partitions row-major. Each partition runs the per-cell
	// encode path: if the child spec is itself composite, recurse;
	// otherwise dispatch to flat Encode. The flat Encode would build
	// its own scales; we override the position channels' scales when
	// shared.
	var cells []scene.SceneCell
	var warnings []scene.Warning
	for ri := 0; ri < rows; ri++ {
		for ci := 0; ci < cols; ci++ {
			partTbl, ok := partitions.Tables[[2]int{ri, ci}]
			if !ok || partTbl == nil {
				// No rows for this (row, col) tuple — skip the cell
				// (matches Vega-Lite's behaviour for sparse facets).
				continue
			}
			cellOpts := opts
			cellOpts.Width = cellW
			cellOpts.Height = cellH
			cellOpts.OverrideXScale = xShared
			cellOpts.OverrideYScale = yShared

			cellDoc, err := encodeFacetCell(child.Spec, partTbl, cellOpts)
			if err != nil {
				return nil, fmt.Errorf("facet cell (%d,%d): %w", ri, ci, err)
			}
			if len(cellDoc.Grid.Cells) == 0 {
				continue
			}
			dx := headerLeft + float64(ci)*(cellW+gap)
			dy := headerTop + float64(ri)*(cellH+gap)

			// If the cell is itself a composite (D058 nested facet),
			// its SceneDoc carries multiple cells laid out within the
			// per-cell sub-grid. Carry all of them through into the
			// outer grid with the outer-cell offset applied. Single-
			// child cells (the common case) walk the same loop with
			// one iteration.
			for ii, inner := range cellDoc.Grid.Cells {
				innerScene := inner.Scene
				offsetScene(&innerScene, dx, dy)
				innerScene.ID = fmt.Sprintf("scene-r%d-c%d-%d", ri, ci, ii)

				// Strip per-cell axes for shared channels (D051).
				if xShared != nil {
					innerScene.Axes = stripAxisChannel(innerScene.Axes, scene.ChannelX)
				}
				if yShared != nil {
					innerScene.Axes = stripAxisChannel(innerScene.Axes, scene.ChannelY)
				}
				cells = append(cells, scene.SceneCell{
					Row:   ri,
					Col:   ci,
					Scene: innerScene,
				})
			}
			warnings = append(warnings, cellDoc.Warnings...)
		}
	}

	doc := scene.NewDoc()
	doc.Theme = sceneTheme
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{
			Rows:  rows,
			Cols:  cols,
			GapPx: int(gap),
			Headers: scene.GridHeaders{
				Left: rowHeaderTexts(rowField, partitions.RowValues),
				Top:  rowHeaderTexts(colField, partitions.ColValues),
			},
		},
		Cells: cells,
	}
	// Shared axes anchored to the first surviving cell's Plot rect.
	if xShared != nil && len(cells) > 0 {
		ax := BuildAxisWithOpts(xShared, scene.ChannelX, scene.AxisPositionBottom, cells[len(cells)-1].Scene.Plot,
			DefaultAxisOpts(facetFieldFromChildSpec(child.Spec, scene.ChannelX)))
		doc.Grid.Shared.X = &ax
	}
	if yShared != nil && len(cells) > 0 {
		ax := BuildAxisWithOpts(yShared, scene.ChannelY, scene.AxisPositionLeft, cells[0].Scene.Plot,
			DefaultAxisOpts(facetFieldFromChildSpec(child.Spec, scene.ChannelY)))
		doc.Grid.Shared.Y = &ax
	}
	doc.Warnings = warnings
	return doc, nil
}

// encodeFacetCell dispatches one partition through either the flat
// Encode path (common case) or recursively through EncodeComposite
// when the child spec is itself composite (D058 nested facet /
// repeat / layer).
func encodeFacetCell(childSpec *spec.Spec, partTbl *table.Table, opts EncodeOpts) (*scene.SceneDoc, error) {
	if isCompositeSpec(childSpec) {
		// Build the child as a fresh composite using the partition's
		// single-table executor result. We synthesise an in-memory
		// composite that mirrors what the build path would produce:
		// for facet/repeat children, recursion is the natural model;
		// for layer/concat children, each grand-child receives the
		// same partition table.
		return encodeNestedCompositeFromTable(childSpec, partTbl, opts)
	}
	// Flat case: feed the partition table directly through Encode by
	// wiring a synthetic tip id.
	const tip plan.NodeID = "facet-cell-tip"
	tables := map[plan.NodeID]*table.Table{tip: partTbl}
	return Encode(childSpec, tables, tip, opts)
}

// isCompositeSpec mirrors plan/build.IsComposite without the import
// (avoids encode → plan/build cycle).
func isCompositeSpec(s *spec.Spec) bool {
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

// encodeNestedCompositeFromTable handles the nested-facet recursion
// case (D058). It constructs a minimal CompositeDAG-equivalent for
// the inner composite and dispatches via the existing kind-specific
// encoders. The inner composite shares the partition table for all
// of its leaves; the recursion bottoms out at the inner-most flat
// Encode call.
//
// v1 supports two specific nesting shapes: facet-within-facet and
// facet-within-layer. Other shapes fall back with a clear error
// pointing at the not-yet-supported nesting combination.
func encodeNestedCompositeFromTable(childSpec *spec.Spec, partTbl *table.Table, opts EncodeOpts) (*scene.SceneDoc, error) {
	if childSpec.Facet != nil {
		// Build a synthetic CompositeDAG holding the shared partition
		// as the upstream. We recycle the partition table by stashing
		// it under a synthetic NodeID and pretending the upstream sub-
		// DAG produces it.
		const tip plan.NodeID = "facet-nest-tip"
		fake := &plan.CompositeDAG{
			Kind:     plan.CompositeFacet,
			Children: []plan.ChildDAG{{Tip: tip, Spec: childSpec.ChildSpec}},
			Resolve:  childSpec.Resolve,
		}
		childTables := []map[plan.NodeID]*table.Table{
			{tip: partTbl},
		}
		return encodeFacetComposite(childSpec, fake, childTables, opts)
	}
	return nil, fmt.Errorf("encode: nested composite kind not supported under facet cell yet (got %T)", childSpec)
}

// facetPartitions carries the partition mapping for a facet's
// upstream table. RowValues / ColValues are the distinct values in
// first-seen order on the row / col field. Tables is keyed by
// (rowIdx, colIdx) into the value slices.
type facetPartitions struct {
	RowValues []any
	ColValues []any
	Tables    map[[2]int]*table.Table
}

// partitionTable splits src by (rowField, colField) tuples. Empty
// rowField → single row partition holding every row (likewise for
// colField). Returns first-seen value ordering so the rendered grid
// matches source-row order.
func partitionTable(src *table.Table, rowField, colField string) (*facetPartitions, error) {
	if src == nil {
		return nil, fmt.Errorf("partitionTable: nil source")
	}
	out := &facetPartitions{
		Tables: map[[2]int]*table.Table{},
	}

	n := src.NumRows()
	rowVals := []any{nil}
	colVals := []any{nil}
	rowIdx := make([]int, n)
	colIdx := make([]int, n)

	if rowField != "" {
		col, ok := src.Column(rowField)
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("Facet row field %q absent from upstream table.", rowField),
				map[string]any{"Field": rowField, "Source": "<facet-upstream>", "Available": joinTableFields(src)},
			)
		}
		rowVals, rowIdx = enumerate(col)
	} else {
		for i := range rowIdx {
			rowIdx[i] = 0
		}
	}
	if colField != "" {
		col, ok := src.Column(colField)
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("Facet column field %q absent from upstream table.", colField),
				map[string]any{"Field": colField, "Source": "<facet-upstream>", "Available": joinTableFields(src)},
			)
		}
		colVals, colIdx = enumerate(col)
	} else {
		for i := range colIdx {
			colIdx[i] = 0
		}
	}

	out.RowValues = rowVals
	out.ColValues = colVals

	// Build a keep-bitmap per (row, col) tuple seen, then materialise
	// the partition table.
	for ri := 0; ri < len(rowVals); ri++ {
		for ci := 0; ci < len(colVals); ci++ {
			keep := make([]bool, n)
			anyKept := false
			for i := 0; i < n; i++ {
				if rowIdx[i] == ri && colIdx[i] == ci {
					keep[i] = true
					anyKept = true
				}
			}
			if !anyKept {
				continue
			}
			tag := fmt.Sprintf("facet:r%d:c%d", ri, ci)
			sub, err := table.Filter(src, keep, tag)
			if err != nil {
				return nil, err
			}
			out.Tables[[2]int{ri, ci}] = sub
		}
	}
	return out, nil
}

// enumerate scans col and returns (distinct values in first-seen
// order, per-row index into that slice).
func enumerate(col table.Column) ([]any, []int) {
	n := col.Len()
	values := []any{}
	indices := make([]int, n)
	seen := map[any]int{}
	for i := 0; i < n; i++ {
		v := col.ValueAt(i)
		idx, ok := seen[v]
		if !ok {
			idx = len(values)
			seen[v] = idx
			values = append(values, v)
		}
		indices[i] = idx
	}
	return values, indices
}

// facetField returns the binding's field name (empty when nil).
func facetField(ch *spec.FacetChannel) string {
	if ch == nil {
		return ""
	}
	return ch.Field
}

// buildSharedScaleForFacet computes one shared scale across every
// facet partition's contribution to the channel. Reuses the
// encode/resolve.Unify path indirectly via per-partition value
// collection.
func buildSharedScaleForFacet(childSpec *spec.Spec, parts *facetPartitions, channel scene.Channel, rmin, rmax float64) (Scale, error) {
	if childSpec == nil || childSpec.Encoding == nil {
		return nil, nil
	}
	var ch *spec.PositionChannel
	switch channel {
	case scene.ChannelX:
		ch = childSpec.Encoding.X
	case scene.ChannelY:
		ch = childSpec.Encoding.Y
	}
	if ch == nil || ch.Field == "" {
		return nil, nil
	}

	// Union the per-partition values into one big slice; resolve via
	// the regular per-channel resolver so type inference (band vs
	// linear vs time) lands consistently with the flat Encode path.
	var allValues []any
	for _, tbl := range parts.Tables {
		col, ok := tbl.Column(ch.Field)
		if !ok {
			continue
		}
		for i := 0; i < col.Len(); i++ {
			allValues = append(allValues, col.ValueAt(i))
		}
	}
	if len(allValues) == 0 {
		return nil, nil
	}
	// Synthesise a one-shot table-like wrapper via the per-channel
	// resolver. We do not have the Table here, only the values, so
	// pull the column-kind from the first partition + use
	// ResolveScale directly.
	for _, tbl := range parts.Tables {
		col, ok := tbl.Column(ch.Field)
		if !ok {
			continue
		}
		// All partitions share the same upstream schema (table.Filter
		// preserves kinds), so the first column's kind is canonical.
		if ch.Scale != nil && ch.Scale.Type != "" {
			opts := ScaleOpts{}
			if ch.Scale.Base != nil {
				opts.Base = *ch.Scale.Base
			}
			if ch.Scale.Exponent != nil {
				opts.Exp = *ch.Scale.Exponent
			}
			sc, _, err := ResolveScaleTyped(scene.ScaleType(ch.Scale.Type), allValues, rmin, rmax, opts)
			return sc, err
		}
		sc, _, err := ResolveScale(ch.Type, col.Kind(), allValues, rmin, rmax)
		return sc, err
	}
	return nil, nil
}

// facetFieldFromChildSpec returns the field bound on the child
// spec's encoding for the given channel; used as the shared-axis
// title.
func facetFieldFromChildSpec(childSpec *spec.Spec, channel scene.Channel) string {
	if childSpec == nil || childSpec.Encoding == nil {
		return ""
	}
	switch channel {
	case scene.ChannelX:
		if childSpec.Encoding.X != nil {
			return childSpec.Encoding.X.Field
		}
	case scene.ChannelY:
		if childSpec.Encoding.Y != nil {
			return childSpec.Encoding.Y.Field
		}
	}
	return ""
}

// stripAxisChannel returns the axes slice with any entry on the
// given channel removed. Used to honour D051 when a channel resolves
// shared.
func stripAxisChannel(axes []scene.Axis, channel scene.Channel) []scene.Axis {
	out := axes[:0]
	for _, ax := range axes {
		if ax.Channel == channel {
			continue
		}
		out = append(out, ax)
	}
	return out
}

// rowHeaderTexts assembles the per-row / per-col header strings the
// renderer emits at the grid edge. Empty field → no headers (single
// dimension facet). Each header label is "<field> = <value>".
func rowHeaderTexts(field string, values []any) []string {
	if field == "" {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, fmt.Sprintf("%s = %v", field, v))
	}
	return out
}
