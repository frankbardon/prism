package encode

import (
	"fmt"

	"github.com/frankbardon/prism/encode/marks"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// EncodeOpts controls the encoder's per-call layout knobs. Width
// and Height default to 800×600 when zero. ThemeName selects a
// registered theme (light/dark/print + user-loaded); Theme is the
// resolved scene-IR theme override (wins over ThemeName).
type EncodeOpts struct {
	Width     float64
	Height    float64
	Theme     *scene.Theme
	ThemeName string
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
	sceneTheme, err := resolveTheme(opts, s.Theme)
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
	layout := Compute(width, height, hasTitle)

	var warnings []scene.Warning

	// Resolve x / y scales.
	xScale, xWarn, err := resolveChannel(enc.X, tbl, layout.Plot.X, layout.Plot.Right())
	if err != nil {
		return nil, err
	}
	if xWarn != nil {
		warnings = append(warnings, *xWarn)
	}
	// Y is inverted: low data → high pixel (the SVG y-axis grows
	// downward). Pass (rangeMax, rangeMin) so the linear interpolation
	// flips naturally.
	yScale, yWarn, err := resolveChannel(enc.Y, tbl, layout.Plot.Bottom(), layout.Plot.Y)
	if err != nil {
		return nil, err
	}
	if yWarn != nil {
		warnings = append(warnings, *yWarn)
	}

	// Build axes (only when the channel was bound).
	axes := make([]scene.Axis, 0, 2)
	if xScale != nil {
		axes = append(axes, BuildAxisWithOpts(xScale, scene.ChannelX, scene.AxisPositionBottom, layout.Plot, axisOptsFor(enc.X)))
	}
	if yScale != nil {
		axes = append(axes, BuildAxisWithOpts(yScale, scene.ChannelY, scene.AxisPositionLeft, layout.Plot, axisOptsFor(enc.Y)))
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
			Field:      enc.Color.Field,
			Categories: cats,
			Palette:    DefaultPalette(),
		}
	}

	// Mark-level style overrides.
	style := defaultMarkStyle(markType)
	if s.Mark != nil && s.Mark.Def != nil {
		applyMarkDef(s.Mark.Def, &style)
	}

	markInputs := marks.Inputs{
		Table:  tbl,
		X:      marks.Channel{Field: fieldOf(enc.X), Scale: toMarkScale(xScale)},
		Y:      marks.Channel{Field: fieldOf(enc.Y), Scale: toMarkScale(yScale)},
		Color:  colorChannel,
		Layout: layout.Plot,
		Style:  style,
	}
	if s.Mark != nil {
		markInputs.Mark = s.Mark.Def
	}

	markList, markWarn, err := marks.Encode(markType, markInputs)
	if err != nil {
		return nil, err
	}
	if markWarn != nil {
		warnings = append(warnings, *markWarn)
	}

	// Wrap into the full nesting. Map spec mark type ("bar", "line"…)
	// to the canonical scene.MarkType (MarkRect, MarkLine…).
	layer := scene.SceneLayer{
		ID:    "layer-0",
		Mark:  specMarkToScene(markType),
		Marks: markList,
	}
	// Build legends for non-trivial mark channels.
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
		ID:      "scene-0",
		Frame:   layout.Frame,
		Plot:    layout.Plot,
		Axes:    axes,
		Legends: legends,
		Layers:  []scene.SceneLayer{layer},
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

// resolveChannel turns a PositionChannel + table into a Scale.
// Returns (nil, nil, nil) when the channel is nil or has no field
// binding — the encoder skips axis creation in that case.
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

// defaultMarkStyle returns the P05-default style for a mark type.
// All marks pick up the theme's category-1 fill unless the spec's
// MarkDef overrides.
func defaultMarkStyle(markType string) scene.Style {
	defaultFill, _ := scene.ColorFromHex("#3b82f6")
	switch markType {
	case "line", "rule":
		// Lines / rules use stroke, not fill.
		return scene.Style{
			Stroke:      defaultFill,
			StrokeWidth: 1.5,
		}
	case "area":
		// Area uses fill + a lighter stroke.
		return scene.Style{
			Fill:    defaultFill,
			Opacity: 0.7,
		}
	default:
		return scene.Style{Fill: defaultFill}
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

// resolveTheme picks the active theme. Precedence:
//  1. opts.Theme — explicit scene-IR override (CSS string carried).
//  2. opts.ThemeName + spec.theme — registry lookup + sparse override.
//  3. spec.theme alone (uses light as base when name omitted).
//  4. registered light theme.
//
// Returns PRISM_RENDER_THEME_UNKNOWN when ThemeName / spec.theme.name
// references an unregistered theme.
func resolveTheme(opts EncodeOpts, override *spec.ThemeOverride) (*scene.Theme, error) {
	if opts.Theme != nil {
		return opts.Theme, nil
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
		return nil, prismerrors.New(
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
	return scn, nil
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
