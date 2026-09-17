package encode

import (
	"fmt"
	"strconv"

	"github.com/frankbardon/prism/encode/marks"
	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/geodata"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// EncodeOpts controls the encoder's per-call layout knobs. Width
// and Height default to 800×600 when zero. ThemeName selects a
// registered theme (light/dark/print + user-loaded); Theme is the
// resolved scene-IR theme override (wins over ThemeName).
//
// OverrideXScale / OverrideYScale (P09) let a composite caller hand
// the flat Encode path pre-computed shared scales. When non-nil,
// Encode skips its per-channel resolver for that axis and uses the
// override verbatim. Drives the shared-axis facet path (D057) and
// any future composite that wants to share a scale across cells
// without restating the spec.
type EncodeOpts struct {
	Width  float64
	Height float64
	Theme  *scene.Theme
	// FullTheme carries the resolved *theme.Theme (per-mark blocks,
	// scheme registry, range slots) so composite cells reuse the
	// parent's full theme rather than rebuilding from ThemeName.
	// When nil, resolveThemeFull rebuilds from ThemeName / spec.Theme.
	FullTheme      *theme.Theme
	ThemeName      string
	OverrideXScale Scale
	OverrideYScale Scale
}

// flatSceneID is the id the flat encoder stamps on the single scene
// it builds. A composite caller renumbers it through renameScene,
// which also re-keys anything filed under it in Defs (the plot clip,
// and a gradient legend's <linearGradient>).
const flatSceneID = "scene-0"

// sparkMarks is the single source of truth for "spark" marks —
// compact marks that render without axes, legend, or title and use
// the tight 4-px-pad layout (ComputeSparkline). Adding a spark mark
// is a one-line edit here; the three chrome-suppression sites in
// Encode consult it via isSparkMark. `bullet` is intentionally absent
// (it keeps its scale axis). The later spark variants (sparkbar,
// winloss, sparkarea) are listed ahead of their encoders so the
// chrome behavior lands with the mark in its own story.
var sparkMarks = map[string]bool{
	"sparkline": true,
	"sparkbar":  true,
	"winloss":   true,
	"sparkarea": true,
}

// isSparkMark reports whether markType is a chrome-suppressed spark
// mark (no axis/legend/title, ComputeSparkline layout).
func isSparkMark(markType string) bool {
	return sparkMarks[markType]
}

// Encode turns a validated *spec.Spec plus the executor's output
// tables into a SceneDoc ready for any Renderer. The tipID is the
// node id whose Table feeds the encoder (returned by
// plan/build.Build alongside the DAG).
//
// Pipeline (per design/02-architecture.md § Stage 5):
//  1. Pull the tip table.
//  2. Compute layout.
//  3. Resolve x / y scales from the upstream column values.
//  4. Build axes from the resolved scales.
//  5. Dispatch the encoded mark to encode/marks for geometry.
//  6. Wrap one SceneLayer → Scene → 1×1 SceneGrid → SceneDoc
//     (full nesting always; no flat-chart special case).
//
// All warnings collected along the way attach to SceneDoc.Warnings.
//
// Encode is the top-of-tree entry point: it delegates the encoding to
// encodeLeaf and then appends the spec-wide inert-field warnings
// (E7-S1). Composition cells call encodeLeaf directly, which is what
// keeps InertFieldWarnings a single reporter — a layer child is
// walked once, from the root spec, never once per cell.
func Encode(s *spec.Spec, tables map[plan.NodeID]*table.Table, tipID plan.NodeID, opts EncodeOpts) (*scene.SceneDoc, error) {
	doc, err := encodeLeaf(s, tables, tipID, opts)
	if err != nil {
		return nil, err
	}
	doc.Warnings = append(doc.Warnings, InertFieldWarnings(s)...)
	doc.Warnings = collapseOffsetCollisions(doc.Warnings)
	narrowDocCSS(doc, s, opts)
	return doc, nil
}

// encodeLeaf is Encode without the inert-field pass — the entry every
// composition cell uses.
func encodeLeaf(s *spec.Spec, tables map[plan.NodeID]*table.Table, tipID plan.NodeID, opts EncodeOpts) (*scene.SceneDoc, error) {
	if s == nil {
		return nil, fmt.Errorf("encode: nil spec")
	}
	width := opts.Width
	if width == 0 {
		width = 800
	}
	height := opts.Height
	if height == 0 {
		height = 600
	}
	sceneTheme, fullTheme, err := resolveThemeFull(opts, s.Theme)
	if err != nil {
		return nil, err
	}
	// E4-S3 auto-dark mark colors: this Encode call "owns" the theme
	// (opts.Theme == nil — a genuine top-of-tree resolve, not a
	// composite-cell reuse of an already-resolved *scene.Theme). Only
	// the owner is allowed to create the registry and, at the end of
	// this function, regenerate sceneTheme.CSS to include the
	// resolved-var declarations — composite cells (opts.Theme != nil)
	// keep colorReg/darkTheme nil and every mark color call site below
	// falls back to the pre-E4-S3 baked-hex path unchanged. See
	// finalizeAutoDarkCSS.
	isThemeOwner := opts.Theme == nil
	var colorReg *marks.ColorVarRegistry
	var darkTheme *theme.Theme
	if isThemeOwner && fullTheme != nil && fullTheme.DarkVariant != "" {
		if dv, ok := theme.Get(fullTheme.DarkVariant); ok {
			darkTheme = dv
			colorReg = marks.NewColorVarRegistry()
		}
	}

	tbl, ok := tables[tipID]
	if !ok || tbl == nil {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Encoder asked for tip node %q but executor returned no table for it.", tipID),
			map[string]any{"Field": string(tipID), "Source": "<executor>", "Available": joinNodeIDs(tables)},
		)
	}

	// Stacking (E5-S2): when the plan injected a StackNode, repoint the
	// stacked position channel at its bounds columns before anything
	// reads the encoding. Everything downstream — domain resolution,
	// axis building, the span-aware bar / area encoders — then treats
	// the stack as an ordinary x→x2 / y→y2 interval. No-op for every
	// spec that does not stack. See encode/stack.go.
	s = rebindStack(s, tbl)

	enc := s.Encoding
	if enc == nil {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"Spec has no encoding block; encoder cannot resolve channels.",
			map[string]any{"Field": "<encoding>", "Source": "<spec>", "Available": "x|y|color"},
		)
	}
	markType := ""
	if s.Mark != nil {
		markType = s.Mark.TypeName()
	}
	if markType == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"Spec has no mark type; encoder cannot dispatch.",
			map[string]any{"Field": "<mark>", "Source": "<spec>", "Available": "bar|line|area|point|rule"},
		)
	}

	hasTitle := s.Title != nil
	// Axis placement drives both the layout reservation and the
	// scene.Axis.Position stamped below, so padding can never disagree
	// with where the axis actually renders. The reservation is keyed on
	// the placement, not on whether the channel turned out to be bound:
	// an unbound channel still reserves its side, as it always has.
	// A channel that hides its axis with `"axis": null` is the one
	// exception — it claims no side, so the plot expands into the
	// padding the axis would have reserved.
	placement := placementFor(enc)
	// Legend placement (E1-S3) resolves before the layout, because a
	// side orient (left / right / top / bottom) reserves a margin band
	// the plot rect has to shrink for. Corner orients (the default
	// top-right included) overlay the plot and reserve nothing, which
	// is what keeps default placement byte-identical.
	//
	// Legend content (E3-S3) resolves first, because the band a side
	// orient reserves is measured from the *shown* entries and the
	// label budget — both of which legend.values, legend.title and
	// legend.label_limit move — and because legend.type, read here,
	// decides which KIND of legend is coming (E3-S4). The kind in
	// turn picks the default anchor: a symbol legend keeps the
	// top-right corner, a gradient bar defaults to the right side so
	// it reserves margin instead of lying over the cells it describes.
	legendContent := ResolveLegendContent(legendSpecOf(enc))
	legendKind := ResolveLegendKind(enc.Color, legendContent)
	legendPl, legendEnabled := ResolveLegendPlacement(legendSpecOf(enc), DefaultLegendPosition(legendKind))
	sides := placement.Sides()
	reservedLegendSide := false
	if legendEnabled && !isSparkMark(markType) && IsSideLegend(legendPl.Position) {
		// The box is measured up front: a top/bottom band's depth
		// grows with the entry count, and a channel that builds no
		// legend at all (fewer than two categories, or a gradient
		// over a column with no numeric cell) must not reserve an
		// empty band either.
		if box, ok := legendReserveBox(legendKind, enc, tbl, legendPl, legendContent, colorLegendTitle(enc)); ok {
			sides.MarkLegend(legendPl.Position, box.SideExtent(legendPl.Position, legendPl.Offset))
			reservedLegendSide = true
		}
	}
	// Sparkline (D067): 4-px-padded plot rect, no axis/legend/title
	// reservation; the title block, axes, and legends are suppressed
	// at scene-assembly time below.
	var layout Layout
	if isSparkMark(markType) {
		layout = ComputeSparkline(width, height)
		hasTitle = false
	} else {
		layout = Compute(LayoutOpts{
			Width:  width,
			Height: height,
			Title:  hasTitle,
			Sides:  sides,
		})
	}
	if reservedLegendSide {
		legendPl.Reserve = layout.Padding.LegendBand(legendPl.Position, hasTitle)
	}

	var warnings []scene.Warning

	// Table (E1) has no positioned geometry at all — encoding.columns[]
	// is its entire visual contract, resolved against the upstream
	// table (already filtered/sorted/limited/aggregated by the
	// standard transform pipeline) into a scene.Table node. Dispatch
	// here, before any cartesian/polar scale resolution runs.
	if markType == "table" {
		return buildTableSceneDoc(s, tbl, enc, fullTheme, sceneTheme, layout, hasTitle)
	}

	// Custom (E2) is likewise freeform/document-flow — no cartesian/
	// polar scale resolution applies — but unlike table it IS
	// consumed by the SVG backend directly (render/svg/custom.go).
	if markType == "custom" {
		var markDef *spec.MarkDef
		if s.Mark != nil {
			markDef = s.Mark.Def
		}
		return buildCustomSceneDoc(s, tbl, markDef, sceneTheme, layout, hasTitle)
	}

	// Polar marks (arc / pie / donut) consume theta + (optional) color;
	// they do not need cartesian x / y scales. Histogram builds its
	// own synthetic x/y scales inside the encoder (D060) so the
	// standard x/y resolution is skipped here too. The arc / histogram
	// encoders return their own axes when relevant.
	//
	// P11 marks that bring their own geometry:
	//   - sankey: source/target/value channels, no axes (D064/D065).
	//   - funnel: stacked trapezoids, no cartesian axes (D066).
	//   - path:   raw SVG d-string, no axes.
	// Image mark uses x/y when bound, otherwise skips — let the
	// standard path run; the image encoder is forgiving on missing
	// scales.
	polarMark := isPolarMark(markType)
	selfScaleMark := isSelfScaleMark(markType)
	specialtyMark := isSpecialtyMark(markType)
	geoMark := isGeoMark(markType)

	// Drop rows carrying a null in a scale-bound channel before any
	// scale resolution runs, so the domains, the color categories and
	// every per-row consumer downstream (marks, tooltips, datum
	// back-references, category styles, conditions) agree on one row
	// set. Only the cartesian x / y channels are scale-bound — the
	// marks that skip that resolution below (polar / histogram /
	// specialty / geo) bring their own geometry and never hand a raw
	// field value to Scale.Apply. A null in a non-scale-bound channel
	// (tooltip, text, color, …) is left alone.
	// mark.invalid picks between the two: "filter" (the default)
	// removes the rows, so the table shortens and the category leaves
	// the scale domain with them; "break" keeps every row and carries
	// a draw mask instead, so the category holds its slot on the axis
	// and a path mark splits at the gap. Exactly one of the two is
	// ever in play, which is what keeps a single row set in flight.
	var skipRows []bool
	if usesCartesianScales(markType) {
		if invalidMode(s) == spec.MarkInvalidBreak {
			mask, nullWarn, nerr := breakNullRows(tbl, "layer-0", scaleBoundChannels(enc)...)
			if nerr != nil {
				return nil, nerr
			}
			skipRows = mask
			if nullWarn != nil {
				warnings = append(warnings, *nullWarn)
			}
		} else {
			filtered, nullWarn, nerr := marks.DropNullRows(tbl, "layer-0", scaleBoundChannels(enc)...)
			if nerr != nil {
				return nil, nerr
			}
			tbl = filtered
			if nullWarn != nil {
				warnings = append(warnings, *nullWarn)
			}
		}
	}

	// Resolve x / y scales (composite caller may supply pre-computed
	// shared overrides per P09 / D057; honour them when present so
	// every cell in a faceted grid lands on the same domain).
	var (
		xScale Scale
		yScale Scale
		xWarn  *scene.Warning
		yWarn  *scene.Warning
	)
	// A bullet mark's bands / target / comparative live on the mark-def,
	// not the data column, so the data-derived measure-axis domain would
	// clip any of them that reach past the data range. Inject them as
	// extra domain values on the measure channel (x when horizontal, y
	// when vertical) so the axis spans the full bullet.
	var xExtra, yExtra []any
	if markType == "bullet" && s.Mark != nil && s.Mark.Def != nil {
		ext := bulletMeasureExtras(s.Mark.Def, tbl)
		if s.Mark.Def.Orientation == "vertical" {
			yExtra = ext
		} else {
			xExtra = ext
		}
	}
	// A progress mark owns its measure domain: mark.total names the
	// value the track runs to, and the track would overflow the plot
	// if the domain stopped at the data max. Inject the totals as
	// extra domain values on the measure axis — x when the mark reads
	// horizontally (a nominal y against a quantitative x, the
	// canonical metric-row shape), y when it reads vertically.
	if markType == "progress" && s.Mark != nil && s.Mark.Def != nil {
		ext := progressMeasureExtras(s.Mark.Def, tbl)
		if progressMeasureIsX(s.Mark.Def, enc) {
			xExtra = append(xExtra, ext...)
		} else {
			yExtra = append(yExtra, ext...)
		}
	}
	// A bound span channel (E9-S3) shares its base channel's scale, so
	// its values have to widen that channel's domain before the scale
	// is built — otherwise an interval reaching past the base column's
	// range would resolve outside the plot. No-ops when x2 / y2 is
	// absent.
	xExtra = append(xExtra, spanDomainValues(enc.X2, tbl)...)
	yExtra = append(yExtra, spanDomainValues(enc.Y2, tbl)...)
	if !polarMark && !selfScaleMark && !specialtyMark && !geoMark {
		if opts.OverrideXScale != nil {
			xScale = opts.OverrideXScale
		} else {
			xScale, xWarn, err = resolveChannel(enc.X, tbl, layout.Plot.X, layout.Plot.Right(), xExtra...)
			if err != nil {
				return nil, err
			}
			if xWarn != nil {
				warnings = append(warnings, *xWarn)
			}
		}
		// Y is inverted: low data → high pixel (the SVG y-axis grows
		// downward). Pass (rangeMax, rangeMin) so the linear interpolation
		// flips naturally.
		if opts.OverrideYScale != nil {
			yScale = opts.OverrideYScale
		} else {
			yScale, yWarn, err = resolveChannel(enc.Y, tbl, layout.Plot.Bottom(), layout.Plot.Y, yExtra...)
			if err != nil {
				return nil, err
			}
			if yWarn != nil {
				warnings = append(warnings, *yWarn)
			}
		}
	}

	// Build axes (only when the channel was bound). Sparkline (D067)
	// suppresses axes entirely — leave axes empty. A channel with
	// `"axis": null` is suppressed the same way: no domain line,
	// ticks, labels, title or grid.
	axes := make([]scene.Axis, 0, 2)
	if !isSparkMark(markType) && !geoMark {
		if xScale != nil && !placement.XHidden {
			axes = append(axes, BuildAxisWithOpts(xScale, scene.ChannelX, placement.X, layout.Plot, axisOptsFor(enc.X).withWarnings(&warnings)))
		}
		if yScale != nil && !placement.YHidden {
			axes = append(axes, BuildAxisWithOpts(yScale, scene.ChannelY, placement.Y, layout.Plot, axisOptsFor(enc.Y).withWarnings(&warnings)))
		}
	}

	// Resolve color channel (P05 supports nominal only).
	var colorChannel *marks.ColorChannel
	if enc.Color != nil && enc.Color.Field != "" {
		col, ok := tbl.Column(enc.Color.Field)
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("Color channel field %q not present in upstream table.", enc.Color.Field),
				map[string]any{"Field": enc.Color.Field, "Source": "<table>", "Available": joinTableFields(tbl)},
			)
		}
		cats := distinctStringValues(col)
		colorChannel = &marks.ColorChannel{
			Field:             enc.Color.Field,
			Categories:        cats,
			Palette:           ResolveCategoricalPaletteWithOpts(fullTheme, colorScaleOpts(enc.Color)),
			SequentialPalette: ResolveSequentialPaletteWithOpts(fullTheme, colorScaleOpts(enc.Color)),
		}
		if darkTheme != nil {
			colorChannel.DarkPalette = ResolveCategoricalPaletteWithOpts(darkTheme, colorScaleOpts(enc.Color))
			colorChannel.DarkSequentialPalette = ResolveSequentialPaletteWithOpts(darkTheme, colorScaleOpts(enc.Color))
		}
	}

	// Field-driven opacity channel (per-cell opacity from a numeric
	// column; consumed by the heatmap encoder today).
	var opacityChannel *marks.OpacityChannel
	if enc.Opacity != nil && enc.Opacity.Field != "" {
		opacityChannel = &marks.OpacityChannel{Field: enc.Opacity.Field}
	}

	// Mark-level style overrides.
	style := defaultMarkStyleAuto(fullTheme, darkTheme, colorReg, markType)
	if s.Mark != nil && s.Mark.Def != nil {
		applyMarkDef(s.Mark.Def, &style)
	}
	if err := applyMarkChannelBaseValues(&style, enc); err != nil {
		return nil, err
	}

	// For polar marks (arc/pie/donut), the theta channel field flows
	// in via marks.Channel.X.Field — the arc encoder builds its own
	// share-based geometry without an x/y scale (D059).
	markX := marks.Channel{Field: fieldOf(enc.X), Scale: toMarkScale(xScale)}
	markY := marks.Channel{Field: fieldOf(enc.Y), Scale: toMarkScale(yScale)}
	if polarMark && enc.Theta != nil && enc.Theta.Field != "" {
		markX = marks.Channel{Field: enc.Theta.Field}
	}

	// Offset position channels (E1-S4): the nested band scale that
	// subdivides a category's slot into sub-bands. The zero binding —
	// which is what an offset-free spec gets — leaves the band
	// arithmetic exactly as it was.
	offsetBind, err := resolveOffsetBinding(enc, tbl, toMarkScale(xScale), toMarkScale(yScale))
	if err != nil {
		return nil, err
	}
	// Rows sharing both a category and an offset value land on the
	// same sub-band and still overlap (E2-S3). Name it; draw it
	// unchanged.
	if offWarn := offsetCollisionWarning(enc, offsetBind, tbl, skipRows, "layer-0"); offWarn != nil {
		warnings = append(warnings, *offWarn)
	}

	markInputs := marks.Inputs{
		Table:         tbl,
		X:             markX,
		Y:             markY,
		X2:            spanChannel(enc.X2, toMarkScale(xScale)),
		Y2:            spanChannel(enc.Y2, toMarkScale(yScale)),
		Offset:        offsetBind,
		Color:         colorChannel,
		Detail:        detailFields(enc),
		Ordered:       spec.ResolveOrder(enc) != nil,
		Opacity:       opacityChannel,
		Layout:        layout.Plot,
		Style:         style,
		LabelStyle:    defaultMarkStyleAuto(fullTheme, darkTheme, colorReg, "text"),
		TrackStyle:    progressTrackStyle(fullTheme),
		MedianStyle:   boxplotMedianStyle(fullTheme, style),
		Skip:          skipRows,
		Tooltip:       enc.Tooltip,
		Text:          enc.Text,
		KeyField:      keyFieldFromEncoding(enc),
		ColorRegistry: colorReg,
	}
	if s.Mark != nil {
		markInputs.Mark = s.Mark.Def
	}
	// Geo marks (P18): build projection from spec + plot rect bbox.
	// Feature / Longitude / Latitude channels carry field names only.
	if geoMark {
		proj, perr := buildProjection(s.Projection, layout.Plot)
		if perr != nil {
			return nil, perr
		}
		markInputs.Projection = proj
		if s.Projection != nil && s.Projection.Tier != "" {
			markInputs.GeoTier = geodata.Tier(s.Projection.Tier)
		}
		if enc.Feature != nil {
			markInputs.Feature = marks.Channel{Field: enc.Feature.Field}
		}
		if enc.Longitude != nil {
			markInputs.Longitude = marks.Channel{Field: enc.Longitude.Field}
		}
		if enc.Latitude != nil {
			markInputs.Latitude = marks.Channel{Field: enc.Latitude.Field}
		}
	}
	// Tree / dendrogram / network (tier1-04): forward source / target
	// channels (the parent / child identity fields). Field names
	// only; the encoder computes positions via the layout subpkg.
	if markType == "tree" || markType == "dendrogram" || markType == "network" {
		if enc.Source != nil {
			markInputs.Source = marks.Channel{Field: enc.Source.Field}
		}
		if enc.Target != nil {
			markInputs.Target = marks.Channel{Field: enc.Target.Field}
		}
		if enc.Value != nil {
			markInputs.Value = marks.Channel{Field: enc.Value.Field}
		}
	}
	// Sankey: forward source/target/value channels (D064). These are
	// field names only; sankey computes positions internally without
	// per-axis scales.
	if markType == "sankey" {
		if enc.Source != nil {
			markInputs.Source = marks.Channel{Field: enc.Source.Field}
		}
		if enc.Target != nil {
			markInputs.Target = marks.Channel{Field: enc.Target.Field}
		}
		if enc.Value != nil {
			markInputs.Value = marks.Channel{Field: enc.Value.Field}
		}
		// Sankey color channel binds the source node field — build a
		// categorical palette over the unique source values when
		// color isn't already bound.
		if colorChannel == nil && enc.Source != nil && enc.Source.Field != "" {
			col, ok := tbl.Column(enc.Source.Field)
			if ok {
				cats := []string{}
				seen := map[string]bool{}
				for i := 0; i < col.Len(); i++ {
					sv, ok := col.ValueAt(i).(string)
					if !ok || seen[sv] {
						continue
					}
					seen[sv] = true
					cats = append(cats, sv)
				}
				// Also fold in target categories so any target-only node
				// gets a colour mapping too.
				if tcol, tok := tbl.Column(enc.Target.Field); tok && enc.Target != nil {
					for i := 0; i < tcol.Len(); i++ {
						sv, ok := tcol.ValueAt(i).(string)
						if !ok || seen[sv] {
							continue
						}
						seen[sv] = true
						cats = append(cats, sv)
					}
				}
				markInputs.Color = &marks.ColorChannel{
					Field:      enc.Source.Field,
					Categories: cats,
					Palette:    ResolveCategoricalPalette(fullTheme, ""),
				}
				if darkTheme != nil {
					markInputs.Color.DarkPalette = ResolveCategoricalPalette(darkTheme, "")
				}
				// Update local colorChannel ref so legend builder picks it up later.
				colorChannel = markInputs.Color
			}
		}
	}

	// Histogram: route via EncodeHistogram so axes can be built from
	// the synthetic bin scales (D060).
	if markType == "histogram" {
		hr, herr := marks.EncodeHistogram(markInputs)
		if herr != nil {
			return nil, herr
		}
		// Attach tooltips per bin if requested (one TooltipLine per
		// bin with the bin index — simple but functional).
		if enc.Tooltip != nil && len(hr.Marks) > 0 {
			tooltips := marks.BuildTooltips(tbl, enc.Tooltip, tbl.NumRows())
			marks.AttachTooltips(hr.Marks, tooltips)
		}
		applyCategoryStyles(enc, tbl, fullTheme, hr.Marks)
		if err := applyConditions(enc, tbl, hr.Marks); err != nil {
			return nil, err
		}
		if hr.XScale != nil && !placement.XHidden {
			axes = append(axes, BuildAxisWithOpts(hr.XScale, scene.ChannelX, placement.X, layout.Plot, axisOptsFor(enc.X).withWarnings(&warnings)))
		}
		if hr.YScale != nil && !placement.YHidden {
			// E3-S5: the synthetic bin-count axis honours channel.axis
			// config exactly like the histogram's x axis; "count" is only
			// the title fallback when the channel names no field and sets
			// no explicit axis.title.
			axes = append(axes, BuildAxisWithOpts(hr.YScale, scene.ChannelY, placement.Y, layout.Plot,
				axisOptsForTitled(enc.Y, "count").withWarnings(&warnings)))
		}
		finalizeAutoDarkCSS(sceneTheme, fullTheme, colorReg, isThemeOwner)
		return buildSceneDoc(s, layout, axes, hr.Marks, markType, colorChannel, enc, sceneTheme, warnings, hasTitle, legendPl, legendEnabled, legendContent, legendKind, tbl), nil
	}

	markList, markWarn, err := marks.Encode(markType, markInputs)
	if err != nil {
		return nil, err
	}
	if markWarn != nil {
		warnings = append(warnings, *markWarn)
	}

	applyCategoryStyles(enc, tbl, fullTheme, markList)
	if err := applyConditions(enc, tbl, markList); err != nil {
		return nil, err
	}

	// Wrap into the full nesting. Map spec mark type ("bar", "line"…)
	// to the canonical scene.MarkType (MarkRect, MarkLine…).
	layer := scene.SceneLayer{
		ID:    "layer-0",
		Mark:  specMarkToScene(markType),
		Marks: markList,
	}
	// Build legends for non-trivial mark channels. Sparkline (D067)
	// suppresses legends entirely, and so does an explicit
	// `"legend": null` on the color channel.
	var legends []scene.Legend
	var legendGradient *scene.Gradient
	legendGradientKey := ""
	if legendEnabled && !isSparkMark(markType) && !legendHidden(enc.Color) && colorChannel != nil &&
		(len(colorChannel.Categories) > 1 || legendKind == LegendKindGradient) {
		// Sankey populates colorChannel from source ∪ target nodes when
		// no explicit color binding exists (D064); use colorChannel.Field
		// as the legend title in that case.
		title := colorChannel.Field
		if enc.Color != nil && enc.Color.Field != "" {
			title = enc.Color.Field
		}
		legendGradientKey = LegendGradientID(flatSceneID, scene.ChannelColor)
		legend, grad := buildColorLegend(colorLegendInputs{
			Kind:       legendKind,
			SceneID:    flatSceneID,
			Channel:    scene.ChannelColor,
			Title:      title,
			Field:      colorChannel.Field,
			Format:     colorChannelFormat(enc),
			Categories: colorChannel.Categories,
			Palette:    colorChannel.Palette,
			Sequential: colorChannel.SequentialPalette,
			Table:      tbl,
			Placement:  legendPl,
			Content:    legendContent,
			Plot:       layout.Plot,
		})
		if legend != nil {
			legends = append(legends, *legend)
			legendGradient = grad
		}
	}

	sceneObj := scene.Scene{
		ID:         flatSceneID,
		Frame:      layout.Frame,
		Plot:       layout.Plot,
		Axes:       axes,
		Legends:    legends,
		Layers:     []scene.SceneLayer{layer},
		Selections: BuildSelections(s.Selection),
		Animation:  animationFromSpec(s),
	}
	if hasTitle {
		sceneObj.Title = &scene.TextElement{
			Content: titleText(s),
			X:       layout.Plot.CenterX(),
			Y:       20,
		}
	}
	registerSceneGradient(&sceneObj, legendGradientKey, legendGradient)
	// Plot-region clip (E2-S2). Armed only when a position scale pins
	// an explicit domain, or `mark_def.clip` asks for it outright.
	armPlotClip(&sceneObj, wantsPlotClip(s))
	finalizeAutoDarkCSS(sceneTheme, fullTheme, colorReg, isThemeOwner)
	doc := scene.NewDoc()
	doc.Theme = sceneTheme
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: sceneObj},
		},
	}
	doc.Warnings = warnings
	return doc, nil
}

// buildSceneDoc wraps a list of marks + axes into the full nesting
// (SceneDoc → SceneGrid → SceneCell → Scene → SceneLayer → marks).
// Used by special-case mark paths (histogram) that build their own
// scales + axes before reaching the standard wrap step.
func buildSceneDoc(
	s *spec.Spec, layout Layout, axes []scene.Axis, markList []scene.Mark,
	markType string, colorChannel *marks.ColorChannel, enc *spec.Encoding,
	sceneTheme *scene.Theme, warnings []scene.Warning, hasTitle bool,
	legendPl LegendPlacement, legendEnabled bool, legendContent LegendContent,
	legendKind LegendKind, tbl *table.Table,
) *scene.SceneDoc {
	layer := scene.SceneLayer{
		ID:    "layer-0",
		Mark:  specMarkToScene(markType),
		Marks: markList,
	}
	var legends []scene.Legend
	var legendGradient *scene.Gradient
	legendGradientKey := LegendGradientID(flatSceneID, scene.ChannelColor)
	if legendEnabled && !legendHidden(enc.Color) && colorChannel != nil &&
		(len(colorChannel.Categories) > 1 || legendKind == LegendKindGradient) {
		legend, grad := buildColorLegend(colorLegendInputs{
			Kind:       legendKind,
			SceneID:    flatSceneID,
			Channel:    scene.ChannelColor,
			Title:      enc.Color.Field,
			Field:      colorChannel.Field,
			Format:     colorChannelFormat(enc),
			Categories: colorChannel.Categories,
			Palette:    colorChannel.Palette,
			Sequential: colorChannel.SequentialPalette,
			Table:      tbl,
			Placement:  legendPl,
			Content:    legendContent,
			Plot:       layout.Plot,
		})
		if legend != nil {
			legends = append(legends, *legend)
			legendGradient = grad
		}
	}
	sceneObj := scene.Scene{
		ID:         flatSceneID,
		Frame:      layout.Frame,
		Plot:       layout.Plot,
		Axes:       axes,
		Legends:    legends,
		Layers:     []scene.SceneLayer{layer},
		Selections: BuildSelections(s.Selection),
		Animation:  animationFromSpec(s),
	}
	if hasTitle {
		sceneObj.Title = &scene.TextElement{
			Content: titleText(s),
			X:       layout.Plot.CenterX(),
			Y:       20,
		}
	}
	registerSceneGradient(&sceneObj, legendGradientKey, legendGradient)
	armPlotClip(&sceneObj, wantsPlotClip(s))
	doc := scene.NewDoc()
	doc.Theme = sceneTheme
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: 1, Cols: 1},
		Cells: []scene.SceneCell{
			{Row: 0, Col: 0, Scene: sceneObj},
		},
	}
	doc.Warnings = warnings
	return doc
}

// resolveChannel turns a PositionChannel + table into a Scale.
// Returns (nil, nil, nil) when the channel is nil or has no field
// binding — the encoder skips axis creation in that case.
//
// When the channel carries an explicit scale.type (and scale.base /
// scale.exponent for log / pow), the typed dispatch ResolveScaleTyped
// takes over. Otherwise the channel-type / column-kind inference
// path runs.
func resolveChannel(ch *spec.PositionChannel, tbl *table.Table, rmin, rmax float64, extra ...any) (Scale, *scene.Warning, error) {
	if ch == nil || ch.Field == "" {
		return nil, nil, nil
	}
	col, ok := tbl.Column(ch.Field)
	if !ok {
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Channel field %q not present in upstream table.", ch.Field),
			map[string]any{"Field": ch.Field, "Source": "<table>", "Available": joinTableFields(tbl)},
		)
	}
	values := make([]any, col.Len(), col.Len()+len(extra))
	for i := 0; i < col.Len(); i++ {
		values[i] = col.ValueAt(i)
	}
	// extra carries mark-supplied domain values (bullet bands / target /
	// comparative) that must widen the data-derived domain.
	values = append(values, extra...)
	opts := ScaleOptsFromSpec(ch.Scale)
	if ch.Scale != nil && ch.Scale.Type != "" {
		return ResolveScaleTyped(scene.ScaleType(ch.Scale.Type), values, rmin, rmax, opts)
	}
	return ResolveScaleWithOpts(ch.Type, col.Kind(), values, rmin, rmax, opts)
}

// bulletMeasureExtras returns the extra measure-axis domain values a
// bullet mark needs so its bands, target, and comparative never clip
// past the data-derived domain. Band bounds are used verbatim; target /
// comparative resolve like the encoder does — a literal number as-is, a
// string as row 0 of the named field.
func bulletMeasureExtras(def *spec.MarkDef, tbl *table.Table) []any {
	if def == nil {
		return nil
	}
	extras := make([]any, 0, len(def.Bands)+2)
	for _, b := range def.Bands {
		extras = append(extras, b)
	}
	for _, ref := range []any{def.Target, def.Comparative} {
		if v, ok := bulletRefDomainValue(ref, tbl); ok {
			extras = append(extras, v)
		}
	}
	return extras
}

// bulletRefDomainValue resolves a bullet target / comparative reference
// to a numeric domain value: a literal number is coerced directly; a
// non-empty string names a field whose row-0 value is read.
func bulletRefDomainValue(raw any, tbl *table.Table) (float64, bool) {
	switch t := raw.(type) {
	case nil:
		return 0, false
	case string:
		if t == "" {
			return 0, false
		}
		col, ok := tbl.Column(t)
		if !ok || col.Len() == 0 {
			return 0, false
		}
		return scale.ToFloat(col.ValueAt(0))
	default:
		return scale.ToFloat(raw)
	}
}

// toMarkScale lifts an encode.Scale into the marks.Scale interface
// (structural; same method set, just a separate package boundary).
func toMarkScale(s Scale) marks.Scale {
	if s == nil {
		return nil
	}
	return s
}

// fieldOf returns the channel's field name, or "" when the channel
// is nil.
func fieldOf(ch *spec.PositionChannel) string {
	if ch == nil {
		return ""
	}
	return ch.Field
}

// isPolarMark reports whether markType consumes theta + colour and
// builds its own share-based geometry (D059) instead of cartesian
// x / y scales.
func isPolarMark(markType string) bool {
	return markType == "arc" || markType == "pie" || markType == "donut"
}

// isSelfScaleMark reports whether markType builds its own synthetic
// x / y scales inside the encoder (D060).
func isSelfScaleMark(markType string) bool {
	return markType == "histogram"
}

// isSpecialtyMark reports whether markType brings its own geometry
// and needs no cartesian axes (P11 marks + the graph family).
func isSpecialtyMark(markType string) bool {
	switch markType {
	case "sankey", "funnel", "path", "tree", "dendrogram", "network":
		return true
	}
	return false
}

// isGeoMark reports whether markType projects lon/lat rather than
// resolving cartesian scales (P18).
func isGeoMark(markType string) bool {
	return markType == "geoshape" || markType == "geopoint"
}

// usesCartesianScales reports whether markType goes through the
// standard x / y scale resolution — and therefore whether a raw field
// value from those channels ever reaches Scale.Apply. It is the one
// predicate both the flat encoder and the layer-composite encoder
// consult before dropping null rows, so the two can never disagree
// about which channels are scale-bound.
func usesCartesianScales(markType string) bool {
	return !isPolarMark(markType) && !isSelfScaleMark(markType) &&
		!isSpecialtyMark(markType) && !isGeoMark(markType)
}

// scaleBoundChannels lists the encoding channels whose raw field
// values are handed to a resolved Scale.Apply — the only channels
// where an upstream null becomes a hard PRISM_ENCODE_001 rather than
// a cosmetic default. Today that is exactly the cartesian x / y pair;
// every other channel (color, opacity, tooltip, text, detail, the
// sankey / geo bindings) either has no scale or tolerates a null.
//
// Feeding marks.DropNullRows from one helper keeps the flat and
// layer-composite encoders on the same definition of "scale-bound".
func scaleBoundChannels(enc *spec.Encoding) []marks.NullChannel {
	if enc == nil {
		return nil
	}
	var out []marks.NullChannel
	if f := fieldOf(enc.X); f != "" {
		out = append(out, marks.NullChannel{Channel: "x", Field: f})
	}
	if f := fieldOf(enc.Y); f != "" {
		out = append(out, marks.NullChannel{Channel: "y", Field: f})
	}
	return out
}

// detailFields (E5-S1) flattens encoding.detail — which decodes as
// either a single entry or an array (spec.DetailChannel) — into the
// ordered list of table field names marks group on. Entries without a
// field are skipped; a nil channel yields nil, which marks.Inputs
// treats as "no detail bound".
//
// Detail is a pure grouping channel: unlike color it resolves no
// scale and no palette here, so nothing beyond the field names needs
// to travel to the mark encoders.
func detailFields(enc *spec.Encoding) []string {
	if enc == nil || enc.Detail == nil {
		return nil
	}
	entries := enc.Detail.Multi
	if enc.Detail.Single != nil {
		entries = append([]spec.DetailChannelEntry{*enc.Detail.Single}, entries...)
	}
	var out []string
	for _, e := range entries {
		if e.Field == "" {
			continue
		}
		out = append(out, e.Field)
	}
	return out
}

// defaultMarkStyle returns the resolved default style for a mark
// type. Cascade order:
//
//  1. Hardcoded fallback (matches the P05 palette so a nil theme
//     still renders a usable chart).
//  2. theme.Mark — global default for all marks.
//  3. theme.Marks[markType] — per-type override.
//
// Theme values shadow the hardcoded fallback per field; spec.MarkDef
// applies on top via applyMarkDef.
//
// Thin wrapper over defaultMarkStyleAuto with darkTheme/reg both nil —
// the pre-E4-S3 behavior, and the one every caller outside this
// file's own Encode() keeps using unchanged (composite/table paths
// don't participate in auto-dark mark colors; see EncodeOpts.Theme's
// doc comment on resolveThemeFull's ownership rule).
func defaultMarkStyle(t *theme.Theme, markType string) scene.Style {
	return defaultMarkStyleAuto(t, nil, nil, markType)
}

// defaultMarkStyleAuto is defaultMarkStyle's E4-S3 auto-dark-aware
// sibling: when reg is non-nil (this Encode call owns theme
// resolution and the active theme has a resolvable DarkVariant),
// static per-mark-type theme colors are also resolved against
// darkTheme and registered as a light/dark pair instead of baked.
// darkTheme/reg both nil reproduces defaultMarkStyle exactly.
func defaultMarkStyleAuto(t, darkTheme *theme.Theme, reg *marks.ColorVarRegistry, markType string) scene.Style {
	style := hardcodedDefaultStyle(markType)
	if t == nil {
		return style
	}
	ms := t.MarkDefault(markType)
	if ms == nil {
		return style
	}
	var darkMS *theme.MarkStyle
	if darkTheme != nil {
		darkMS = darkTheme.MarkDefault(markType)
	}
	applyThemeMarkStyle(&style, ms, t, darkMS, reg)
	return style
}

// hardcodedDefaultStyle is the last-resort fallback used when a
// theme leaves a token nil. Matches the original P05 defaults so
// a fresh repo with no built-in theme tokens still produces an
// identifiable chart.
func hardcodedDefaultStyle(markType string) scene.Style {
	defaultFill, _ := scene.ColorFromHex("#3b82f6")
	switch markType {
	case "line", "rule", "sparkline", "tick":
		return scene.Style{Stroke: defaultFill, StrokeWidth: 1.5}
	case "network", "tree", "dendrogram":
		// network draws both scene.MarkLine edges and scene.MarkPoint /
		// scene.MarkRect nodes from this single Style (encode/marks/network.go
		// reuses in.Style for both), so it needs Fill (nodes) AND Stroke
		// (edges) — unlike line/rule/sparkline, which are Stroke-only.
		// tree/dendrogram share the same reuse pattern (encode/marks/tree.go
		// builds link edges as scene.MarkPath and nodes as scene.MarkPoint /
		// scene.MarkRect from one in.Style), but render/svg's renderPath
		// (unlike renderLine) does not hardcode fill="none" — tree.go clears
		// Fill on its own copy of the Style before building the link mark so
		// the shared Fill here only ever paints the node geoms.
		return scene.Style{Fill: defaultFill, Stroke: defaultFill, StrokeWidth: 1}
	case "area":
		return scene.Style{Fill: defaultFill, Opacity: 0.7}
	case "geoshape":
		fill, _ := scene.ColorFromHex("#cbd5e1")
		stroke, _ := scene.ColorFromHex("#ffffff")
		return scene.Style{Fill: fill, Stroke: stroke, StrokeWidth: 0.5}
	default:
		return scene.Style{Fill: defaultFill}
	}
}

// applyThemeMarkStyle folds a theme.MarkStyle into a scene.Style. A
// Fill/Stroke written as url(#name) that resolves against t's
// Gradients/Patterns registries (theme.Theme.ResolveFillRef, E3-S2)
// sets FillRef/StrokeRef to the def id instead of parsing as a
// literal color (E3-S3) — see scene.Style.FillRef. Hex parse failures
// (and a url(#name) value that doesn't resolve, which
// theme.Theme.Validate would already have rejected for any
// normally-loaded theme) degrade silently: the hardcoded fallback
// held before this call, so the user gets a chart even if a theme
// ships a malformed color.
//
// darkMS/reg (E4-S3) carry the DarkVariant counterpart's resolved
// MarkStyle for the same markType and the live color-var registry.
// Both nil reproduces the pre-E4-S3 behavior exactly (every call
// site outside this file's own Encode() — see defaultMarkStyleAuto).
// When both are set and ms.Fill/Stroke resolves to a plain hex color
// (not a gradient/pattern url(#) ref — those are unaffected by
// auto-dark, per the story's scope), the dark counterpart's same
// field is resolved too and the pair is registered via reg.Resolve,
// leaving Fill/Stroke nil and FillVar/StrokeVar set instead.
func applyThemeMarkStyle(style *scene.Style, ms *theme.MarkStyle, t *theme.Theme, darkMS *theme.MarkStyle, reg *marks.ColorVarRegistry) {
	if ms.Fill != "" {
		if id := t.ResolveFillRef(ms.Fill).DefID(); id != "" {
			style.FillRef = id
			style.Fill = nil
			style.FillVar = ""
		} else if c, err := scene.ColorFromHex(ms.Fill); err == nil {
			darkHex := ""
			if darkMS != nil {
				darkHex = darkMS.Fill
			}
			if v := resolveDarkPairedColor(reg, c, darkHex); v != "" {
				style.Fill = nil
				style.FillVar = v
			} else {
				style.Fill = c
				style.FillVar = ""
			}
			style.FillRef = ""
		}
	}
	if ms.Stroke != "" {
		if id := t.ResolveFillRef(ms.Stroke).DefID(); id != "" {
			style.StrokeRef = id
			style.Stroke = nil
			style.StrokeVar = ""
		} else if c, err := scene.ColorFromHex(ms.Stroke); err == nil {
			darkHex := ""
			if darkMS != nil {
				darkHex = darkMS.Stroke
			}
			if v := resolveDarkPairedColor(reg, c, darkHex); v != "" {
				style.Stroke = nil
				style.StrokeVar = v
			} else {
				style.Stroke = c
				style.StrokeVar = ""
			}
			style.StrokeRef = ""
		}
	}
	if ms.StrokeWidth != nil {
		style.StrokeWidth = *ms.StrokeWidth
	}
	if ms.StrokeDash != nil {
		style.StrokeDash = append([]float64(nil), ms.StrokeDash...)
	}
	if ms.Opacity != nil {
		style.Opacity = *ms.Opacity
	}
	// E4-S1: the theme's paint-alpha / typography tokens. These are
	// the theme.MarkStyle counterparts of spec.MarkDef's fill_opacity
	// / font_weight / font_style, and applyMarkDef runs after this
	// function, so a spec mark_def value shadows the theme token.
	// (theme.MarkStyle has no stroke_opacity or font-family token —
	// the mark def's stroke_opacity / font are spec-only.)
	if ms.FillOpacity != nil {
		v := *ms.FillOpacity
		style.FillOpacity = &v
	}
	if w, ok := normalizeFontWeight(ms.FontWeight); ok {
		style.FontWeight = w
	}
	if ms.FontStyle != "" {
		style.FontStyle = ms.FontStyle
	}
	if ms.LineHeight != nil {
		v := *ms.LineHeight
		style.LineHeight = &v
	}
	if ms.LetterSpacing != nil {
		v := *ms.LetterSpacing
		style.LetterSpacing = &v
	}
	if ms.Filter != "" {
		style.Filter = ms.Filter
	}
}

// resolveDarkPairedColor returns the "prism-resolved-N" var name for
// (light, darkHex) when reg is active (auto-dark) and darkHex parses
// to a valid color; "" otherwise — the caller then falls back to
// baking light as a literal hex, exactly the pre-E4-S3 behavior. This
// is the static-theme-color counterpart to
// encode/marks.resolveCategoryColor (which does the same job for
// scale-driven palette colors); kept separate because this call site
// works from a single already-parsed light *scene.Color plus a raw
// dark hex string, not a palette index.
func resolveDarkPairedColor(reg *marks.ColorVarRegistry, light *scene.Color, darkHex string) string {
	if reg == nil || darkHex == "" {
		return ""
	}
	dc, err := scene.ColorFromHex(darkHex)
	if err != nil {
		return ""
	}
	return reg.Resolve(light, dc)
}

// finalizeAutoDarkCSS regenerates sceneTheme.CSS from fullTheme,
// folding in any resolved-color-var pairs colorReg accumulated while
// this Encode call's marks were built (E4-S3). No-op unless isOwner
// (this call resolved its own theme rather than reusing a composite
// parent's — see Encode's isThemeOwner) and colorReg actually holds
// at least one pair; every other call leaves sceneTheme.CSS exactly
// as resolveThemeFull already set it (byte-identical to pre-E4-S3
// output whenever the active theme has no DarkVariant).
func finalizeAutoDarkCSS(sceneTheme *scene.Theme, fullTheme *theme.Theme, colorReg *marks.ColorVarRegistry, isOwner bool) {
	if !isOwner || colorReg == nil || sceneTheme == nil || fullTheme == nil {
		return
	}
	pairs := colorReg.Pairs()
	if len(pairs) == 0 {
		return
	}
	vars := make([]theme.ResolvedColorVar, len(pairs))
	for i, p := range pairs {
		vars[i] = theme.ResolvedColorVar{Name: p.Name, Light: p.Light, Dark: p.Dark}
	}
	sceneTheme.CSS = fullTheme.CSSVariables(vars...)
}

// applyMarkDef folds spec.MarkDef overrides into a style.
//
// It runs *after* defaultMarkStyleAuto (which folds in the theme's
// theme.MarkStyle cascade), so every field written here shadows the
// theme's same-named token — the spec-wins precedence documented in
// docs/src/concepts/themes.md. Fields the mark def leaves nil are not
// touched, so the theme value survives.
//
// Paint alphas: FillOpacity / StrokeOpacity are *independent* of
// Opacity, not overrides of it. All three ride into the SVG as
// separate attributes and compose multiplicatively, matching Vega's
// canvas renderer (`alpha = opacity * (fillOpacity ?? 1)`) and SVG's
// own compositing. See scene.Style.FillOpacity.
//
// Geometric mark-def fields (dx / dy / pad_angle) are not style —
// they land on the geometry in the per-mark encoders
// (encode/marks/text.go, encode/marks/arc.go).
func applyMarkDef(def *spec.MarkDef, style *scene.Style) {
	if def == nil {
		return
	}
	if def.Fill != "" {
		if c, err := scene.ColorFromHex(def.Fill); err == nil {
			style.Fill = c
		}
	}
	if def.Stroke != "" {
		if c, err := scene.ColorFromHex(def.Stroke); err == nil {
			style.Stroke = c
		}
	}
	if def.StrokeWidth != nil {
		style.StrokeWidth = *def.StrokeWidth
	}
	// stroke_dash (E7-S4) is a whole-pattern override, not a merge:
	// a spec dash replaces the theme MarkStyle.StrokeDash applyThemeMarkStyle
	// may already have written, the same all-or-nothing rule every
	// other field here follows. Copied rather than aliased so a later
	// mutation of the spec slice cannot reach into the Scene IR.
	// An explicit empty array is indistinguishable from absent on the
	// wire (both decode to a zero-length slice), so it leaves the
	// theme value alone — to draw solid over a dashed theme, omit the
	// key and set stroke_dash on the theme's mark block instead.
	if len(def.StrokeDash) > 0 {
		style.StrokeDash = append([]float64(nil), def.StrokeDash...)
	}
	if def.Opacity != nil {
		style.Opacity = *def.Opacity
	}
	if def.FillOpacity != nil {
		v := *def.FillOpacity
		style.FillOpacity = &v
	}
	if def.StrokeOpacity != nil {
		v := *def.StrokeOpacity
		style.StrokeOpacity = &v
	}
	if def.Font != "" {
		style.FontFamily = def.Font
	}
	if w, ok := normalizeFontWeight(def.FontWeight); ok {
		style.FontWeight = w
	}
	if def.FontStyle != "" {
		style.FontStyle = def.FontStyle
	}
}

// normalizeFontWeight folds spec's polymorphic font_weight (a JSON
// number or one of the CSS keywords) into scene.Style's numeric
// FontWeight. ok is false for nil, an empty string, or anything the
// keyword table and the number forms don't cover — the caller then
// leaves the existing weight alone rather than writing a 0.
//
// "bolder" / "lighter" are relative in CSS; SVG text in Prism always
// starts from the default inherited weight of 400, so they resolve to
// CSS's computed values for that base — 700 and 100 respectively.
func normalizeFontWeight(v any) (int, bool) {
	switch w := v.(type) {
	case nil:
		return 0, false
	case float64:
		return int(w), w > 0
	case float32:
		return int(w), w > 0
	case int:
		return w, w > 0
	case int64:
		return int(w), w > 0
	case string:
		switch w {
		case "":
			return 0, false
		case "normal":
			return 400, true
		case "bold":
			return 700, true
		case "lighter":
			return 100, true
		case "bolder":
			return 700, true
		}
		n, err := strconv.Atoi(w)
		if err != nil || n <= 0 {
			return 0, false
		}
		return n, true
	}
	return 0, false
}

// specMarkToScene maps the spec's mark-type string to the canonical
// scene.MarkType. "bar" → MarkRect, "line"/"area"/"point"/"rule"
// map verbatim. Unknown types pass through as-is (the dispatch in
// encode/marks will have already emitted a warning).
func specMarkToScene(markType string) scene.MarkType {
	switch markType {
	case "bar":
		return scene.MarkRect
	case "line":
		return scene.MarkLine
	case "area":
		return scene.MarkArea
	case "point":
		return scene.MarkPoint
	case "rule":
		return scene.MarkRule
	case "arc", "pie", "donut":
		return scene.MarkArc
	case "text":
		return scene.MarkText
	case "path":
		return scene.MarkPath
	case "image":
		return scene.MarkImage
	case "geoshape":
		return scene.MarkGeoshape
	case "geopoint":
		return scene.MarkPoint
	}
	return scene.MarkType(markType)
}

// titleText extracts a plain-string title from the spec's
// polymorphic title field. The TextOrTextObj union exposes both a
// bare-string Text and a rich-object Obj; we pick whichever is set.
// Subtitle / per-language titles land in P06.
func titleText(s *spec.Spec) string {
	if s.Title == nil {
		return ""
	}
	if s.Title.Text != nil {
		return *s.Title.Text
	}
	if s.Title.Obj != nil {
		return s.Title.Obj.Text
	}
	return ""
}

// joinNodeIDs renders the executor's table map keys as a
// comma-separated string for error contexts.
func joinNodeIDs(tables map[plan.NodeID]*table.Table) string {
	keys := make([]string, 0, len(tables))
	for k := range tables {
		keys = append(keys, string(k))
	}
	// Local insertion sort for determinism (small map).
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	if len(keys) == 0 {
		return ""
	}
	out := keys[0]
	for _, k := range keys[1:] {
		out += ", " + k
	}
	return out
}

// colorScaleOpts lifts a color channel's scale block into the
// ScaleOpts the palette cascade reads (scheme, inline range,
// interpolation space). A nil channel or a channel with no scale
// block yields the zero value, which reads as "all defaults".
func colorScaleOpts(ch *spec.MarkChannel) ScaleOpts {
	if ch == nil {
		return ScaleOpts{}
	}
	return ScaleOptsFromSpec(ch.Scale)
}

// resolveTheme picks the active theme. Precedence:
//  1. opts.Theme — explicit scene-IR override (CSS string carried).
//  2. opts.ThemeName + spec.theme — registry lookup + sparse override.
//  3. spec.theme alone (uses light as base when name omitted).
//  4. registered light theme.
//
// Returns PRISM_RENDER_THEME_UNKNOWN when ThemeName / spec.theme.name
// references an unregistered theme.
// resolveThemeFull returns both the wire-shape *scene.Theme and the
// full *theme.Theme. Encoders that need to read per-mark defaults,
// range slots, or named schemes hold onto the full struct; the
// scene theme stays the source of CSS bytes that ride into SceneDoc.
func resolveThemeFull(opts EncodeOpts, override *spec.ThemeOverride) (*scene.Theme, *theme.Theme, error) {
	if opts.Theme != nil {
		// Pre-resolved scene theme path (composite cells, RPC). When
		// the parent also handed us the full theme, reuse it so
		// per-cell encoding still consults theme.Marks; otherwise
		// fall back to a name-only stub.
		full := opts.FullTheme
		if full == nil {
			full = &theme.Theme{Name: opts.Theme.Name}
		}
		return opts.Theme, full, nil
	}
	name := opts.ThemeName
	if override != nil && override.Name != "" {
		name = override.Name
	}
	if name == "" {
		name = "light"
	}
	base, ok := theme.Get(name)
	if !ok {
		return nil, nil, prismerrors.New(
			"PRISM_RENDER_THEME_UNKNOWN",
			fmt.Sprintf("Unknown theme %q.", name),
			map[string]any{"Theme": name, "Available": joinNames(theme.Names())},
		)
	}
	merged := base
	if override != nil {
		merged = theme.ApplyOverride(base, override)
	}
	scn := merged.ToSceneTheme()
	scn.Name = merged.Name
	scn.CSS = merged.CSSVariables()
	return scn, merged, nil
}

// findCellThemeOverride returns the ThemeOverride matching (row, col)
// in overrides, or nil when no entry addresses that cell. Overrides
// are addressed by 0-based (Row, Column) grid position — the same
// addressing facet/repeat assign to scene.SceneCell.Row/Col (see
// spec.CellThemeOverride's doc comment). Last match wins when the
// spec (unusually) lists more than one entry for the same cell, so
// behaviour is deterministic and matches how a caller reading the
// list top-to-bottom would expect a later entry to take precedence.
func findCellThemeOverride(overrides []spec.CellThemeOverride, row, col int) *spec.ThemeOverride {
	var found *spec.ThemeOverride
	for i := range overrides {
		if overrides[i].Row == row && overrides[i].Column == col {
			found = &overrides[i].Theme
		}
	}
	return found
}

// resolveCellTheme layers a per-cell ThemeOverride on top of the
// chart's already-resolved base theme, using the exact same
// theme.ApplyOverride + ToSceneTheme/CSSVariables path
// resolveThemeFull uses for the spec-level `theme` override — this
// is a new call site for that machinery, not a new merge. When
// override is nil the base scene/full theme pair is returned
// unchanged (no allocation, byte-identical output for cells with no
// matching override).
func resolveCellTheme(baseScene *scene.Theme, baseFull *theme.Theme, override *spec.ThemeOverride) (*scene.Theme, *theme.Theme) {
	if override == nil {
		return baseScene, baseFull
	}
	merged := theme.ApplyOverride(baseFull, override)
	scn := merged.ToSceneTheme()
	scn.Name = merged.Name
	scn.CSS = merged.CSSVariables()
	return scn, merged
}

// joinNames is the tiny comma-joiner used in error contexts.
func joinNames(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	out := xs[0]
	for _, x := range xs[1:] {
		out += ", " + x
	}
	return out
}

// axisHidden reports whether a position channel carries an explicit
// `"axis": null` (Vega-Lite's suppression syntax). An absent axis key
// is NOT hidden — spec.PositionChannel decodes the two states apart.
func axisHidden(ch *spec.PositionChannel) bool {
	return ch != nil && ch.AxisHidden
}

// legendHidden reports whether a mark channel carries an explicit
// `"legend": null`. As with axisHidden, an absent key is not hidden.
func legendHidden(ch *spec.MarkChannel) bool {
	return ch != nil && ch.LegendHidden
}

// placementFor returns the axis placement for a flat encoding: each
// channel's `axis.orient` resolved into a side, with its
// `"axis": null` suppression applied. The layout reservation and the
// axis-building guards below read the same value, so padding always
// follows the axis to the side it moved to.
//
// Orient is read through axisOptsFor, keeping that the single reader
// of `channel.axis`.
func placementFor(enc *spec.Encoding) AxisPlacement {
	p := DefaultAxisPlacement()
	if enc == nil {
		return p
	}
	p.X = AxisPositionFor(scene.ChannelX, axisOptsFor(enc.X).Orient)
	p.Y = AxisPositionFor(scene.ChannelY, axisOptsFor(enc.Y).Orient)
	p.XHidden = axisHidden(enc.X)
	p.YHidden = axisHidden(enc.Y)
	// E3-S2: a component suppressed by `axis.labels: false` /
	// `axis.ticks: false` releases the padding it would have reserved,
	// resolved through the same axisOptsFor the axis is built from.
	p.ReserveFrom(axisOptsFor(enc.X), axisOptsFor(enc.Y))
	return p
}

// specAxisHidden reports whether a child spec hides the axis for the
// given cartesian channel. A nested composite child carries no
// encoding block of its own and hides nothing.
func specAxisHidden(s *spec.Spec, ch scene.Channel) bool {
	if s == nil || s.Encoding == nil {
		return false
	}
	switch ch {
	case scene.ChannelX:
		return axisHidden(s.Encoding.X)
	case scene.ChannelY:
		return axisHidden(s.Encoding.Y)
	}
	return false
}

// specDeclaresChannel reports whether a child spec binds the given
// cartesian channel at all (hidden or not).
func specDeclaresChannel(s *spec.Spec, ch scene.Channel) bool {
	if s == nil || s.Encoding == nil {
		return false
	}
	switch ch {
	case scene.ChannelX:
		return s.Encoding.X != nil
	case scene.ChannelY:
		return s.Encoding.Y != nil
	}
	return false
}

// layerAxisHidden reports whether a stack of layered children agrees
// that the channel's axis is suppressed: at least one child declares
// the channel and every child that declares it sets `"axis": null`.
// Layers share one pair of axes, so a single layer that still wants
// its axis keeps it — and keeps its padding — for the whole stack.
func layerAxisHidden(specs []*spec.Spec, ch scene.Channel) bool {
	declared := false
	for _, s := range specs {
		if !specDeclaresChannel(s, ch) {
			continue
		}
		declared = true
		if !specAxisHidden(s, ch) {
			return false
		}
	}
	return declared
}

// axisOptsFor resolves AxisOpts from a PositionChannel. Reads
// channel.axis.{orient, title, grid, label_angle, label_overlap,
// format}. Defaults match DefaultAxisOpts; the spec selectively
// overrides.
func axisOptsFor(ch *spec.PositionChannel) AxisOpts {
	title := ""
	if ch != nil {
		title = ch.Field
	}
	opts := DefaultAxisOpts(title)
	if ch == nil {
		return opts
	}
	if ch.Axis == nil {
		return opts
	}
	opts.Orient = ch.Axis.Orient
	if t, ok := axisTitleString(ch.Axis.Title); ok {
		opts.Title = t
	}
	if ch.Axis.Grid != nil {
		opts.Grid = *ch.Axis.Grid
	}
	if ch.Axis.LabelAngle != nil {
		opts.LabelAngle = *ch.Axis.LabelAngle
	}
	if mode, ok := overlapMode(ch.Axis.LabelOverlap); ok {
		opts.LabelOverlap = mode
	}
	if ch.Axis.Format != "" {
		opts.Format = ch.Axis.Format
	}
	// E3-S2: component visibility, geometry and layering. Each is an
	// optional pointer, so an absent key leaves the default in place
	// and the three visibility switches compose independently.
	if ch.Axis.Labels != nil {
		opts.Labels = *ch.Axis.Labels
	}
	if ch.Axis.Ticks != nil {
		opts.Ticks = *ch.Axis.Ticks
	}
	if ch.Axis.Domain != nil {
		opts.Domain = *ch.Axis.Domain
	}
	opts.TickSize = ch.Axis.TickSize
	opts.LabelPadding = ch.Axis.LabelPadding
	opts.TitlePadding = ch.Axis.TitlePadding
	opts.LabelLimit = ch.Axis.LabelLimit
	if ch.Axis.Zindex != nil {
		opts.Zindex = *ch.Axis.Zindex
	}
	if ch.Axis.TickCount != nil {
		n := *ch.Axis.TickCount
		opts.TickCount = &n
	}
	if ch.Axis.TickMinStep != nil {
		opts.TickMinStep = *ch.Axis.TickMinStep
	}
	if len(ch.Axis.Values) > 0 {
		opts.Values = append([]any(nil), ch.Axis.Values...)
	}
	return opts
}

// axisOptsForTitled resolves AxisOpts from a PositionChannel, falling
// back to the supplied title when the channel names no field. An
// explicit `"title": false` still suppresses the title — the fallback
// only fills a title the spec never asked about.
func axisOptsForTitled(ch *spec.PositionChannel, fallback string) AxisOpts {
	opts := axisOptsFor(ch)
	if opts.Title == "" && !axisTitleExplicit(ch) {
		opts.Title = fallback
	}
	return opts
}

// axisTitleExplicit reports whether the channel's axis block sets a
// usable `title` (a string, or `false` to suppress).
func axisTitleExplicit(ch *spec.PositionChannel) bool {
	if ch == nil || ch.Axis == nil {
		return false
	}
	_, ok := axisTitleString(ch.Axis.Title)
	return ok
}

// axisTitleString accepts the polymorphic axis.title field (string or
// false to suppress). Returns ("", true) when explicitly suppressed.
func axisTitleString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case bool:
		if !t {
			return "", true
		}
	}
	return "", false
}

// overlapMode normalises axis.label_overlap (bool or string) onto the
// modes applyLabelOverlap acts on.
//
// Every spelling schema/v1/axis.schema.json accepts must land on one of
// them. A string that reaches applyLabelOverlap unrecognised matches no
// branch and hides nothing, so the author's setting silently does
// nothing — "greedy" shipped in the schema enum in exactly that state.
// An unknown string falls back to the default rather than disabling the
// pass, so a typo degrades to normal behaviour instead of quietly
// turning overlap handling off.
func overlapMode(v any) (string, bool) {
	switch t := v.(type) {
	case bool:
		if t {
			return overlapParity, true
		}
		return overlapNone, true
	case string:
		switch t {
		case overlapNone, "false":
			return overlapNone, true
		case overlapGreedy:
			return overlapGreedy, true
		default:
			// "parity", "auto", and anything unrecognised.
			return overlapParity, true
		}
	}
	return "", false
}

// joinTableFields renders the table's columns as a comma-separated
// string for error contexts.
func joinTableFields(tbl *table.Table) string {
	names := tbl.FieldNames()
	if len(names) == 0 {
		return ""
	}
	out := names[0]
	for _, n := range names[1:] {
		out += ", " + n
	}
	return out
}
