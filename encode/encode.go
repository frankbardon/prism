package encode

import (
	"fmt"

	"github.com/frankbardon/prism/encode/marks"
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
func Encode(s *spec.Spec, tables map[plan.NodeID]*table.Table, tipID plan.NodeID, opts EncodeOpts) (*scene.SceneDoc, error) {
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

	tbl, ok := tables[tipID]
	if !ok || tbl == nil {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Encoder asked for tip node %q but executor returned no table for it.", tipID),
			map[string]any{"Field": string(tipID), "Source": "<executor>", "Available": joinNodeIDs(tables)},
		)
	}

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
	// Sparkline (D067): 4-px-padded plot rect, no axis/legend/title
	// reservation; the title block, axes, and legends are suppressed
	// at scene-assembly time below.
	var layout Layout
	if markType == "sparkline" {
		layout = ComputeSparkline(width, height)
		hasTitle = false
	} else {
		layout = Compute(width, height, hasTitle)
	}

	var warnings []scene.Warning

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
	polarMark := markType == "arc" || markType == "pie" || markType == "donut"
	selfScaleMark := markType == "histogram"
	specialtyMark := markType == "sankey" || markType == "funnel" || markType == "path" ||
		markType == "tree" || markType == "dendrogram" || markType == "network"
	geoMark := markType == "geoshape" || markType == "geopoint"

	// Resolve x / y scales (composite caller may supply pre-computed
	// shared overrides per P09 / D057; honour them when present so
	// every cell in a faceted grid lands on the same domain).
	var (
		xScale Scale
		yScale Scale
		xWarn  *scene.Warning
		yWarn  *scene.Warning
	)
	if !polarMark && !selfScaleMark && !specialtyMark && !geoMark {
		if opts.OverrideXScale != nil {
			xScale = opts.OverrideXScale
		} else {
			xScale, xWarn, err = resolveChannel(enc.X, tbl, layout.Plot.X, layout.Plot.Right())
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
			yScale, yWarn, err = resolveChannel(enc.Y, tbl, layout.Plot.Bottom(), layout.Plot.Y)
			if err != nil {
				return nil, err
			}
			if yWarn != nil {
				warnings = append(warnings, *yWarn)
			}
		}
	}

	// Build axes (only when the channel was bound). Sparkline (D067)
	// suppresses axes entirely — leave axes empty.
	axes := make([]scene.Axis, 0, 2)
	if markType != "sparkline" && !geoMark {
		if xScale != nil {
			axes = append(axes, BuildAxisWithOpts(xScale, scene.ChannelX, scene.AxisPositionBottom, layout.Plot, axisOptsFor(enc.X)))
		}
		if yScale != nil {
			axes = append(axes, BuildAxisWithOpts(yScale, scene.ChannelY, scene.AxisPositionLeft, layout.Plot, axisOptsFor(enc.Y)))
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
		cats := []string{}
		seen := map[string]bool{}
		for i := 0; i < col.Len(); i++ {
			s, ok := col.ValueAt(i).(string)
			if !ok || seen[s] {
				continue
			}
			seen[s] = true
			cats = append(cats, s)
		}
		colorChannel = &marks.ColorChannel{
			Field:             enc.Color.Field,
			Categories:        cats,
			Palette:           ResolveCategoricalPalette(fullTheme, schemeNameOf(enc.Color)),
			SequentialPalette: ResolveSequentialPalette(fullTheme, schemeNameOf(enc.Color)),
		}
	}

	// Mark-level style overrides.
	style := defaultMarkStyle(fullTheme, markType)
	if s.Mark != nil && s.Mark.Def != nil {
		applyMarkDef(s.Mark.Def, &style)
	}

	// For polar marks (arc/pie/donut), the theta channel field flows
	// in via marks.Channel.X.Field — the arc encoder builds its own
	// share-based geometry without an x/y scale (D059).
	markX := marks.Channel{Field: fieldOf(enc.X), Scale: toMarkScale(xScale)}
	markY := marks.Channel{Field: fieldOf(enc.Y), Scale: toMarkScale(yScale)}
	if polarMark && enc.Theta != nil && enc.Theta.Field != "" {
		markX = marks.Channel{Field: enc.Theta.Field}
	}

	markInputs := marks.Inputs{
		Table:    tbl,
		X:        markX,
		Y:        markY,
		Color:    colorChannel,
		Layout:   layout.Plot,
		Style:    style,
		Tooltip:  enc.Tooltip,
		KeyField: keyFieldFromEncoding(enc),
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
		if err := applyConditions(enc, tbl, hr.Marks); err != nil {
			return nil, err
		}
		if hr.XScale != nil {
			axes = append(axes, BuildAxisWithOpts(hr.XScale, scene.ChannelX, scene.AxisPositionBottom, layout.Plot, axisOptsFor(enc.X)))
		}
		if hr.YScale != nil {
			yTitle := "count"
			if enc.Y != nil && enc.Y.Field != "" {
				yTitle = enc.Y.Field
			}
			axes = append(axes, BuildAxisWithOpts(hr.YScale, scene.ChannelY, scene.AxisPositionLeft, layout.Plot, DefaultAxisOpts(yTitle)))
		}
		return buildSceneDoc(s, layout, axes, hr.Marks, markType, colorChannel, enc, sceneTheme, warnings, hasTitle), nil
	}

	markList, markWarn, err := marks.Encode(markType, markInputs)
	if err != nil {
		return nil, err
	}
	if markWarn != nil {
		warnings = append(warnings, *markWarn)
	}

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
	// suppresses legends entirely.
	var legends []scene.Legend
	if markType != "sparkline" && colorChannel != nil && len(colorChannel.Categories) > 1 {
		// Sankey populates colorChannel from source ∪ target nodes when
		// no explicit color binding exists (D064); use colorChannel.Field
		// as the legend title in that case.
		title := colorChannel.Field
		if enc.Color != nil && enc.Color.Field != "" {
			title = enc.Color.Field
		}
		legend := BuildSymbolLegend(LegendInputs{
			Channel:    scene.ChannelColor,
			Title:      title,
			Categories: colorChannel.Categories,
			Palette:    colorChannel.Palette,
			Position:   scene.LegendTopRight,
		}, layout.Plot)
		if legend != nil {
			legends = append(legends, *legend)
		}
	}

	sceneObj := scene.Scene{
		ID:         "scene-0",
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
) *scene.SceneDoc {
	layer := scene.SceneLayer{
		ID:    "layer-0",
		Mark:  specMarkToScene(markType),
		Marks: markList,
	}
	var legends []scene.Legend
	if colorChannel != nil && len(colorChannel.Categories) > 1 {
		title := enc.Color.Field
		legend := BuildSymbolLegend(LegendInputs{
			Channel:    scene.ChannelColor,
			Title:      title,
			Categories: colorChannel.Categories,
			Palette:    colorChannel.Palette,
			Position:   scene.LegendTopRight,
		}, layout.Plot)
		if legend != nil {
			legends = append(legends, *legend)
		}
	}
	sceneObj := scene.Scene{
		ID:         "scene-0",
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
func resolveChannel(ch *spec.PositionChannel, tbl *table.Table, rmin, rmax float64) (Scale, *scene.Warning, error) {
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
	values := make([]any, col.Len())
	for i := 0; i < col.Len(); i++ {
		values[i] = col.ValueAt(i)
	}
	if ch.Scale != nil && ch.Scale.Type != "" {
		opts := ScaleOpts{}
		if ch.Scale.Base != nil {
			opts.Base = *ch.Scale.Base
		}
		if ch.Scale.Exponent != nil {
			opts.Exp = *ch.Scale.Exponent
		}
		return ResolveScaleTyped(scene.ScaleType(ch.Scale.Type), values, rmin, rmax, opts)
	}
	return ResolveScale(ch.Type, col.Kind(), values, rmin, rmax)
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
func defaultMarkStyle(t *theme.Theme, markType string) scene.Style {
	style := hardcodedDefaultStyle(markType)
	if t == nil {
		return style
	}
	if ms := t.MarkDefault(markType); ms != nil {
		applyThemeMarkStyle(&style, ms)
	}
	return style
}

// hardcodedDefaultStyle is the last-resort fallback used when a
// theme leaves a token nil. Matches the original P05 defaults so
// a fresh repo with no built-in theme tokens still produces an
// identifiable chart.
func hardcodedDefaultStyle(markType string) scene.Style {
	defaultFill, _ := scene.ColorFromHex("#3b82f6")
	switch markType {
	case "line", "rule":
		return scene.Style{Stroke: defaultFill, StrokeWidth: 1.5}
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

// applyThemeMarkStyle folds a theme.MarkStyle into a scene.Style.
// Hex parse failures degrade silently (the hardcoded fallback held
// before this call, so the user gets a chart even if a theme ships
// a malformed color).
func applyThemeMarkStyle(style *scene.Style, ms *theme.MarkStyle) {
	if ms.Fill != "" {
		if c, err := scene.ColorFromHex(ms.Fill); err == nil {
			style.Fill = c
		}
	}
	if ms.Stroke != "" {
		if c, err := scene.ColorFromHex(ms.Stroke); err == nil {
			style.Stroke = c
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
}

// applyMarkDef folds spec.MarkDef overrides into a style. P05
// honours Fill, Stroke, StrokeWidth, Opacity; richer fields land in
// P06.
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
	if def.Opacity != nil {
		style.Opacity = *def.Opacity
	}
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

// schemeNameOf returns the scheme name from a color-channel
// scale block, or "" when absent.
func schemeNameOf(ch *spec.MarkChannel) string {
	if ch == nil || ch.Scale == nil {
		return ""
	}
	return ch.Scale.Scheme
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

// axisOptsFor resolves AxisOpts from a PositionChannel. Reads
// channel.axis.{title, grid, label_angle, label_overlap, format}.
// Defaults match DefaultAxisOpts; the spec selectively overrides.
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
	return opts
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

// overlapMode normalises axis.label_overlap (bool or string).
func overlapMode(v any) (string, bool) {
	switch t := v.(type) {
	case bool:
		if t {
			return "parity", true
		}
		return "none", true
	case string:
		return t, true
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
