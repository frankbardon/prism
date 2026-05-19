package encode

import (
	"fmt"

	"github.com/frankbardon/prism/encode/marks"
	encresolve "github.com/frankbardon/prism/encode/resolve"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// EncodeComposite turns a *plan.CompositeDAG + the executor's
// per-child tip tables into a SceneDoc. Layer kinds collapse into a
// single Scene whose Layers slice carries one entry per child; concat
// / hconcat / vconcat kinds emit one Scene per cell into the grid.
//
// childTables is one map per child (positional: childTables[i] holds
// the executor result for composite.Children[i]). Per-child maps
// avoid the NodeID-collision foot-gun where two sibling sub-DAGs
// independently allocate the same auto-counter id; the encoder reads
// `childTables[i][child.Tip]` rather than a merged map.
//
// Per D049 each child's tip table comes from `childTables[i][child.Tip]`;
// missing tips emit PRISM_WARN_LAYER_SKIPPED and that child is
// dropped from the output (the other children still render).
//
// Per D050 nested composites are rejected at BuildComposite time;
// EncodeComposite does not recurse into a child composite — children
// here are always flat (built via Build).
//
// The flat-chart Encode path is unchanged; existing P05 / P06 / P07
// goldens stay byte-identical because the 1×1 single-layer case is
// preserved.
func EncodeComposite(s *spec.Spec, composite *plan.CompositeDAG, childTables []map[plan.NodeID]*table.Table, opts EncodeOpts) (*scene.SceneDoc, error) {
	if s == nil {
		return nil, fmt.Errorf("encode: nil spec")
	}
	if composite == nil {
		return nil, fmt.Errorf("encode: nil CompositeDAG")
	}
	if len(childTables) != len(composite.Children) {
		return nil, fmt.Errorf("encode: childTables length %d != composite.Children length %d",
			len(childTables), len(composite.Children))
	}

	switch composite.Kind {
	case plan.CompositeLayer:
		return encodeLayerComposite(s, composite, childTables, opts)
	case plan.CompositeHConcat, plan.CompositeVConcat, plan.CompositeConcat:
		return encodeConcatComposite(s, composite, childTables, opts)
	case plan.CompositeFacet:
		return encodeFacetComposite(s, composite, childTables, opts)
	case plan.CompositeRepeat:
		return encodeRepeatComposite(s, composite, childTables, opts)
	}
	return nil, prismerrors.New(
		"PRISM_PLAN_002",
		fmt.Sprintf("Unknown composite kind %q.", composite.Kind),
		map[string]any{"Kind": string(composite.Kind), "Phase": "P08"},
	)
}

// encodeLayerComposite assembles N layers into one Scene. Cross-layer
// scale resolution: shared scales unify the domain across surviving
// layers; independent scales resolve once per layer. Shared axes live
// on SceneGrid.SharedAxes; per-layer cell axes hold the independent
// ones. The output grid is always 1×1 (one Scene with N layers).
func encodeLayerComposite(s *spec.Spec, composite *plan.CompositeDAG, childTables []map[plan.NodeID]*table.Table, opts EncodeOpts) (*scene.SceneDoc, error) {
	width := opts.Width
	if width == 0 {
		width = 800
	}
	height := opts.Height
	if height == 0 {
		height = 600
	}
	sceneTheme, err := resolveTheme(opts, s.Theme)
	if err != nil {
		return nil, err
	}

	hasTitle := s.Title != nil
	layout := Compute(width, height, hasTitle)

	var warnings []scene.Warning

	// First pass: collect surviving children (tables present), warn
	// on skipped layers.
	var live []liveChild
	for i, child := range composite.Children {
		tbl, ok := childTables[i][child.Tip]
		if !ok || tbl == nil {
			warnings = append(warnings, scene.Warning{
				Code:    scene.WarnLayerSkipped,
				Layer:   fmt.Sprintf("layer-%d", i),
				Message: fmt.Sprintf("layer %d skipped: tip table %q absent from executor result.", i, child.Tip),
				Details: map[string]any{"layer": i, "tip": string(child.Tip)},
			})
			continue
		}
		live = append(live, liveChild{idx: i, child: child, tbl: tbl})
	}
	if len(live) == 0 {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"Layer composite has no surviving layers (every child tip absent).",
			map[string]any{"Field": "<layers>", "Source": "<executor>", "Available": ""},
		)
	}

	resolution := encresolve.FromSpec(composite.Resolve)

	// Second pass: per-channel domain collection for shared resolution.
	// Build LayerDomain entries per surviving layer for X and Y.
	xLayerDomains, err := collectLayerDomains(live, scene.ChannelX)
	if err != nil {
		return nil, err
	}
	yLayerDomains, err := collectLayerDomains(live, scene.ChannelY)
	if err != nil {
		return nil, err
	}

	// Resolve shared scales when requested. xSharedScale / ySharedScale
	// are non-nil only when the channel is shared AND at least one
	// layer contributes a domain.
	var (
		xSharedScale Scale
		ySharedScale Scale
		xSharedTitle string
		ySharedTitle string
	)
	if resolution[scene.ChannelX].Scale == encresolve.ModeShared && len(xLayerDomains) > 0 {
		xSharedScale, err = resolveSharedScale(xLayerDomains, live, scene.ChannelX,
			layout.Plot.X, layout.Plot.Right())
		if err != nil {
			return nil, err
		}
		xSharedTitle = firstFieldName(live, scene.ChannelX)
	}
	if resolution[scene.ChannelY].Scale == encresolve.ModeShared && len(yLayerDomains) > 0 {
		// Y axis is pixel-inverted: pass (bottom, top).
		ySharedScale, err = resolveSharedScale(yLayerDomains, live, scene.ChannelY,
			layout.Plot.Bottom(), layout.Plot.Y)
		if err != nil {
			return nil, err
		}
		ySharedTitle = firstFieldName(live, scene.ChannelY)
	}

	// Third pass: encode each layer's marks using either the shared
	// scale or a freshly resolved per-layer scale. Independent-axis
	// layers append their axes into per-cell axes; shared axes are
	// emitted once on SceneGrid.SharedAxes (D051).
	var sceneLayers []scene.SceneLayer
	var perCellAxes []scene.Axis
	var legends []scene.Legend
	seenIndependentX := false
	seenIndependentY := false
	for _, lc := range live {
		childEnc := lc.child.Spec.Encoding
		if childEnc == nil {
			warnings = append(warnings, scene.Warning{
				Code:    scene.WarnNoDataForLayer,
				Layer:   fmt.Sprintf("layer-%d", lc.idx),
				Message: fmt.Sprintf("layer %d has no encoding; skipped.", lc.idx),
			})
			continue
		}
		markType := ""
		if lc.child.Spec.Mark != nil {
			markType = lc.child.Spec.Mark.TypeName()
		}
		if markType == "" {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("Layer %d has no mark type; encoder cannot dispatch.", lc.idx),
				map[string]any{"Field": "<mark>", "Source": "<layer-spec>", "Available": "bar|line|area|point|rule"},
			)
		}

		// Per-layer X scale: shared one when present, else resolve per
		// channel.
		var xScale Scale
		if xSharedScale != nil {
			xScale = xSharedScale
		} else if childEnc.X != nil && childEnc.X.Field != "" {
			sc, wn, err := resolveChannel(childEnc.X, lc.tbl, layout.Plot.X, layout.Plot.Right())
			if err != nil {
				return nil, err
			}
			if wn != nil {
				warnings = append(warnings, *wn)
			}
			xScale = sc
		}
		var yScale Scale
		if ySharedScale != nil {
			yScale = ySharedScale
		} else if childEnc.Y != nil && childEnc.Y.Field != "" {
			sc, wn, err := resolveChannel(childEnc.Y, lc.tbl, layout.Plot.Bottom(), layout.Plot.Y)
			if err != nil {
				return nil, err
			}
			if wn != nil {
				warnings = append(warnings, *wn)
			}
			yScale = sc
		}

		// Add per-layer independent axes; emit once per channel so
		// stacking N layers does not produce N visually-identical axes.
		if xScale != nil && xSharedScale == nil && !seenIndependentX {
			perCellAxes = append(perCellAxes, BuildAxisWithOpts(
				xScale, scene.ChannelX, scene.AxisPositionBottom, layout.Plot,
				axisOptsFor(childEnc.X)))
			seenIndependentX = true
		}
		if yScale != nil && ySharedScale == nil && !seenIndependentY {
			perCellAxes = append(perCellAxes, BuildAxisWithOpts(
				yScale, scene.ChannelY, scene.AxisPositionLeft, layout.Plot,
				axisOptsFor(childEnc.Y)))
			seenIndependentY = true
		}

		// Color channel (per-layer; cross-layer legend sharing is a
		// future feature alongside facet shared legends in P09).
		var colorChannel *marks.ColorChannel
		if childEnc.Color != nil && childEnc.Color.Field != "" {
			col, ok := lc.tbl.Column(childEnc.Color.Field)
			if !ok {
				return nil, prismerrors.New(
					"PRISM_ENCODE_001",
					fmt.Sprintf("Layer %d color field %q not in upstream table.", lc.idx, childEnc.Color.Field),
					map[string]any{"Field": childEnc.Color.Field, "Source": "<layer-table>", "Available": joinTableFields(lc.tbl)},
				)
			}
			cats := []string{}
			seenCat := map[string]bool{}
			for i := 0; i < col.Len(); i++ {
				s, ok := col.ValueAt(i).(string)
				if !ok || seenCat[s] {
					continue
				}
				seenCat[s] = true
				cats = append(cats, s)
			}
			colorChannel = &marks.ColorChannel{
				Field:      childEnc.Color.Field,
				Categories: cats,
				Palette:    DefaultPalette(),
			}
			if len(cats) > 1 {
				legend := BuildSymbolLegend(LegendInputs{
					Channel:    scene.ChannelColor,
					Title:      fmt.Sprintf("layer-%d: %s", lc.idx, childEnc.Color.Field),
					Categories: cats,
					Palette:    colorChannel.Palette,
					Position:   scene.LegendTopRight,
				}, layout.Plot)
				if legend != nil {
					legends = append(legends, *legend)
				}
			}
		}

		style := defaultMarkStyle(markType)
		if lc.child.Spec.Mark != nil && lc.child.Spec.Mark.Def != nil {
			applyMarkDef(lc.child.Spec.Mark.Def, &style)
		}

		markInputs := marks.Inputs{
			Table:   lc.tbl,
			X:       marks.Channel{Field: fieldOf(childEnc.X), Scale: toMarkScale(xScale)},
			Y:       marks.Channel{Field: fieldOf(childEnc.Y), Scale: toMarkScale(yScale)},
			Color:   colorChannel,
			Layout:  layout.Plot,
			Style:   style,
			Tooltip: childEnc.Tooltip,
		}
		if lc.child.Spec.Mark != nil {
			markInputs.Mark = lc.child.Spec.Mark.Def
		}

		markList, markWarn, err := marks.Encode(markType, markInputs)
		if err != nil {
			return nil, err
		}
		if markWarn != nil {
			warnings = append(warnings, *markWarn)
		}

		sceneLayers = append(sceneLayers, scene.SceneLayer{
			ID:     fmt.Sprintf("layer-%d", lc.idx),
			Source: layerSourceLabel(lc.child.Spec),
			Mark:   specMarkToScene(markType),
			Marks:  markList,
			ZIndex: lc.idx,
		})
	}

	sceneObj := scene.Scene{
		ID:      "scene-0",
		Frame:   layout.Frame,
		Plot:    layout.Plot,
		Axes:    perCellAxes,
		Legends: legends,
		Layers:  sceneLayers,
	}
	if hasTitle {
		sceneObj.Title = &scene.TextElement{
			Content: titleText(s),
			X:       layout.Plot.CenterX(),
			Y:       20,
		}
	}

	doc := scene.NewDoc()
	doc.Theme = sceneTheme
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: sceneObj},
		},
	}
	// Shared axes (D051): emit once on the grid, not per cell. Only
	// populated when the axis resolves shared AND we built a shared
	// scale for the channel.
	if xSharedScale != nil && resolution[scene.ChannelX].Axis == encresolve.ModeShared {
		ax := BuildAxisWithOpts(xSharedScale, scene.ChannelX, scene.AxisPositionBottom, layout.Plot,
			DefaultAxisOpts(xSharedTitle))
		doc.Grid.Shared.X = &ax
	}
	if ySharedScale != nil && resolution[scene.ChannelY].Axis == encresolve.ModeShared {
		ax := BuildAxisWithOpts(ySharedScale, scene.ChannelY, scene.AxisPositionLeft, layout.Plot,
			DefaultAxisOpts(ySharedTitle))
		doc.Grid.Shared.Y = &ax
	}
	doc.Warnings = warnings
	return doc, nil
}

// liveChild bundles one surviving layer's index, child plan, and
// tip table so the composite-encoder helpers can iterate without
// reaching back into the executor's table map.
type liveChild struct {
	idx   int
	child plan.ChildDAG
	tbl   *table.Table
}

// collectLayerDomains returns one LayerDomain per surviving layer for
// the given channel. Layers whose encoding lacks the channel skip.
// Layers whose declared field is absent from the upstream table
// raise PRISM_ENCODE_001 (defensive — the executor should have
// caught it earlier).
func collectLayerDomains(live []liveChild, channel scene.Channel) ([]encresolve.LayerDomain, error) {
	out := make([]encresolve.LayerDomain, 0, len(live))
	for _, lc := range live {
		enc := lc.child.Spec.Encoding
		if enc == nil {
			continue
		}
		var ch *spec.PositionChannel
		switch channel {
		case scene.ChannelX:
			ch = enc.X
		case scene.ChannelY:
			ch = enc.Y
		case scene.ChannelX2:
			ch = enc.X2
		case scene.ChannelY2:
			ch = enc.Y2
		}
		if ch == nil || ch.Field == "" {
			continue
		}
		col, ok := lc.tbl.Column(ch.Field)
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("Layer %d channel %s field %q not present in upstream table.", lc.idx, channel, ch.Field),
				map[string]any{"Field": ch.Field, "Source": "<layer-table>", "Available": joinTableFields(lc.tbl)},
			)
		}
		values := make([]any, col.Len())
		for i := 0; i < col.Len(); i++ {
			values[i] = col.ValueAt(i)
		}
		ty := scaleTypeForChannel(ch, col.Kind())
		out = append(out, encresolve.LayerDomain{
			LayerID: fmt.Sprintf("layer-%d", lc.idx),
			Channel: channel,
			Type:    ty,
			Values:  values,
		})
	}
	return out, nil
}

// firstFieldName returns the field name on the first live layer that
// declares the given channel. Used as the shared axis title.
func firstFieldName(live []liveChild, channel scene.Channel) string {
	for _, lc := range live {
		enc := lc.child.Spec.Encoding
		if enc == nil {
			continue
		}
		var ch *spec.PositionChannel
		switch channel {
		case scene.ChannelX:
			ch = enc.X
		case scene.ChannelY:
			ch = enc.Y
		}
		if ch != nil && ch.Field != "" {
			return ch.Field
		}
	}
	return ""
}

// layerSourceLabel returns a debug-friendly label for a layer's
// source binding. Used in SceneLayer.Source for cache + diagnostic
// purposes.
func layerSourceLabel(s *spec.Spec) string {
	if s == nil || s.Data == nil {
		return ""
	}
	if s.Data.Name != "" {
		return s.Data.Name
	}
	if s.Data.Source != "" {
		return s.Data.Source
	}
	return ""
}

// scaleTypeForChannel infers the scale family for a PositionChannel
// + table-column kind. If the spec declares scale.type explicitly, it
// wins; otherwise channel.type + column.kind drive the choice (mirrors
// encode/scale.go's inference).
func scaleTypeForChannel(ch *spec.PositionChannel, kind table.Kind) scene.ScaleType {
	if ch.Scale != nil && ch.Scale.Type != "" {
		return scene.ScaleType(ch.Scale.Type)
	}
	switch ch.Type {
	case "quantitative":
		return scene.ScaleLinear
	case "nominal", "ordinal":
		return scene.ScaleBand
	case "temporal":
		return scene.ScaleTime
	}
	switch kind {
	case table.KindString:
		return scene.ScaleBand
	case table.KindDate:
		return scene.ScaleTime
	default:
		return scene.ScaleLinear
	}
}

// resolveSharedScale routes shared-scale resolution through Unify
// when multiple layers contribute domains; for a single surviving
// layer it short-circuits to the same per-channel resolver the flat
// encoder uses (scaleFromUnified does not know how to coerce raw
// temporal strings — only the per-channel resolver does).
func resolveSharedScale(domains []encresolve.LayerDomain, live []liveChild, channel scene.Channel, rmin, rmax float64) (Scale, error) {
	if len(domains) == 1 {
		// Pass-through: find the live layer that contributed this
		// domain and resolve directly via the per-channel path.
		for _, lc := range live {
			enc := lc.child.Spec.Encoding
			if enc == nil {
				continue
			}
			var ch *spec.PositionChannel
			switch channel {
			case scene.ChannelX:
				ch = enc.X
			case scene.ChannelY:
				ch = enc.Y
			}
			if ch == nil || ch.Field == "" {
				continue
			}
			sc, _, err := resolveChannel(ch, lc.tbl, rmin, rmax)
			return sc, err
		}
	}
	ty, dom, err := encresolve.Unify(domains)
	if err != nil {
		return nil, err
	}
	return scaleFromUnified(ty, dom, rmin, rmax)
}

// scaleFromUnified converts a (type, domain) pair from
// encode/resolve.Unify back into an encode.Scale ready for axis +
// mark wiring. Numeric / temporal domains arrive as []any{min, max}
// already in the right shape; categorical domains arrive as the full
// ordered list of categories.
func scaleFromUnified(ty scene.ScaleType, domain []any, rmin, rmax float64) (Scale, error) {
	switch ty {
	case scene.ScaleLinear, scene.ScaleLog, scene.ScalePow, scene.ScaleSqrt:
		if len(domain) < 2 {
			return nil, fmt.Errorf("scaleFromUnified: numeric domain needs [min,max], got %v", domain)
		}
		mn, ok1 := domain[0].(float64)
		mx, ok2 := domain[1].(float64)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("scaleFromUnified: numeric domain values not float64: %T %T", domain[0], domain[1])
		}
		if mn > 0 {
			mn = 0
		}
		if mx < 0 {
			mx = 0
		}
		return &LinearScale{
			DomainMin: mn,
			DomainMax: mx,
			RangeMin:  rmin,
			RangeMax:  rmax,
		}, nil
	case scene.ScaleBand, scene.ScalePoint, scene.ScaleOrdinal:
		cats := make([]string, 0, len(domain))
		for _, v := range domain {
			if s, ok := v.(string); ok {
				cats = append(cats, s)
			}
		}
		return &BandScale{
			Categories: cats,
			RangeMin:   rmin,
			RangeMax:   rmax,
			Padding:    0.1,
		}, nil
	case scene.ScaleTime:
		if len(domain) < 2 {
			return nil, fmt.Errorf("scaleFromUnified: time domain needs [min,max]")
		}
		mn, ok1 := domain[0].(float64)
		mx, ok2 := domain[1].(float64)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("scaleFromUnified: time domain values not float64: %T %T", domain[0], domain[1])
		}
		lin := &LinearScale{DomainMin: mn, DomainMax: mx, RangeMin: rmin, RangeMax: rmax}
		return &TimeScale{Linear: lin}, nil
	}
	return nil, fmt.Errorf("scaleFromUnified: unknown scale type %q", ty)
}

// encodeConcatComposite assembles a multi-cell SceneGrid (concat /
// hconcat / vconcat). Each child encodes via the standard Encode
// path; its produced Scene is offset into a per-cell sub-frame and
// stitched into SceneGrid.Cells in row-major order.
func encodeConcatComposite(s *spec.Spec, composite *plan.CompositeDAG, childTables []map[plan.NodeID]*table.Table, opts EncodeOpts) (*scene.SceneDoc, error) {
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

	rows := composite.Rows
	cols := composite.Cols
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	gap := 20.0
	cellW := (outerW - gap*float64(cols-1)) / float64(cols)
	cellH := (outerH - gap*float64(rows-1)) / float64(rows)
	if cellW < 1 {
		cellW = outerW / float64(cols)
	}
	if cellH < 1 {
		cellH = outerH / float64(rows)
	}

	var cells []scene.SceneCell
	var warnings []scene.Warning
	for i, child := range composite.Children {
		row, col := rowColForIndex(composite.Kind, i, rows, cols)
		offsetX := float64(col) * (cellW + gap)
		offsetY := float64(row) * (cellH + gap)

		childOpts := opts
		childOpts.Width = cellW
		childOpts.Height = cellH

		// Each concat child is a flat chart (D050 forbids nested
		// composition in v1); call Encode directly.
		childDoc, err := Encode(child.Spec, childTables[i], child.Tip, childOpts)
		if err != nil {
			return nil, fmt.Errorf("concat child %d: %w", i, err)
		}
		if len(childDoc.Grid.Cells) == 0 {
			continue
		}
		childScene := childDoc.Grid.Cells[0].Scene
		offsetScene(&childScene, offsetX, offsetY)
		childScene.ID = fmt.Sprintf("scene-%d", i)
		cells = append(cells, scene.SceneCell{
			Row:   row,
			Col:   col,
			Scene: childScene,
		})
		// Inherit child warnings into the outer document.
		warnings = append(warnings, childDoc.Warnings...)
	}

	doc := scene.NewDoc()
	doc.Theme = sceneTheme
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: rows, Cols: cols, GapPx: int(gap)},
		Cells:  cells,
	}
	doc.Warnings = warnings
	return doc, nil
}

// rowColForIndex maps a child index to its (row, col) cell position
// based on the composite kind. vconcat fills the column top-to-bottom;
// hconcat / concat fill the row left-to-right.
func rowColForIndex(kind plan.CompositeKind, i, rows, cols int) (row, col int) {
	switch kind {
	case plan.CompositeVConcat:
		return i, 0
	default:
		return 0, i
	}
}

// offsetScene shifts the Scene's frame / plot / axis / mark / legend
// coordinates by (dx, dy) so cells co-exist in a single SVG viewBox
// without per-cell transform groups in the renderer.
func offsetScene(s *scene.Scene, dx, dy float64) {
	s.Frame.X += dx
	s.Frame.Y += dy
	s.Plot.X += dx
	s.Plot.Y += dy
	if s.Title != nil {
		s.Title.X += dx
		s.Title.Y += dy
	}
	for i := range s.Axes {
		offsetAxis(&s.Axes[i], dx, dy)
	}
	for i := range s.Layers {
		for j := range s.Layers[i].Marks {
			offsetMark(&s.Layers[i].Marks[j], dx, dy)
		}
	}
	for i := range s.Legends {
		s.Legends[i].Frame.X += dx
		s.Legends[i].Frame.Y += dy
	}
}

func offsetAxis(a *scene.Axis, dx, dy float64) {
	a.Domain.X1 += dx
	a.Domain.Y1 += dy
	a.Domain.X2 += dx
	a.Domain.Y2 += dy
	for i := range a.Grid {
		a.Grid[i].X1 += dx
		a.Grid[i].Y1 += dy
		a.Grid[i].X2 += dx
		a.Grid[i].Y2 += dy
	}
	for i := range a.Ticks {
		switch a.Channel {
		case scene.ChannelX:
			a.Ticks[i].Pixel += dx
		case scene.ChannelY:
			a.Ticks[i].Pixel += dy
		}
	}
}

func offsetMark(m *scene.Mark, dx, dy float64) {
	if m.Rect != nil {
		m.Rect.X += dx
		m.Rect.Y += dy
	}
	if m.Line != nil {
		for i := range m.Line.Points {
			m.Line.Points[i][0] += dx
			m.Line.Points[i][1] += dy
		}
	}
	if m.Point != nil {
		m.Point.Cx += dx
		m.Point.Cy += dy
	}
	if m.Text != nil {
		m.Text.X += dx
		m.Text.Y += dy
	}
	if m.Area != nil {
		for i := range m.Area.Upper {
			m.Area.Upper[i][0] += dx
			m.Area.Upper[i][1] += dy
		}
		for i := range m.Area.Lower {
			m.Area.Lower[i][0] += dx
			m.Area.Lower[i][1] += dy
		}
	}
	if m.Rule != nil {
		m.Rule.X1 += dx
		m.Rule.Y1 += dy
		m.Rule.X2 += dx
		m.Rule.Y2 += dy
	}
	if m.Image != nil {
		m.Image.X += dx
		m.Image.Y += dy
	}
	if m.Arc != nil {
		m.Arc.Cx += dx
		m.Arc.Cy += dy
	}
}
