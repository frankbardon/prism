package encode

import (
	"fmt"

	"github.com/frankbardon/prism/encode/marks"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// EncodeOpts controls the encoder's per-call layout knobs. Width
// and Height default to 800×600 when zero. Theme defaults to
// scene.Default() when nil.
type EncodeOpts struct {
	Width  float64
	Height float64
	Theme  *scene.Theme
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
	theme := opts.Theme
	if theme == nil {
		theme = scene.Default()
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
		title := ""
		if enc.X != nil {
			title = enc.X.Field
		}
		axes = append(axes, BuildAxis(xScale, scene.ChannelX, scene.AxisPositionBottom, layout.Plot, title))
	}
	if yScale != nil {
		title := ""
		if enc.Y != nil {
			title = enc.Y.Field
		}
		axes = append(axes, BuildAxis(yScale, scene.ChannelY, scene.AxisPositionLeft, layout.Plot, title))
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

	// Wrap into the full nesting.
	layer := scene.SceneLayer{
		ID:    "layer-0",
		Mark:  scene.MarkType(markType),
		Marks: markList,
	}
	sceneObj := scene.Scene{
		ID:     "scene-0",
		Frame:  layout.Frame,
		Plot:   layout.Plot,
		Axes:   axes,
		Layers: []scene.SceneLayer{layer},
	}
	if hasTitle {
		sceneObj.Title = &scene.TextElement{
			Content: titleText(s),
			X:       layout.Plot.CenterX(),
			Y:       20,
		}
	}
	doc := scene.NewDoc()
	doc.Theme = theme
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

// titleText extracts a plain-string title from the spec's polymorphic
// title field. Subtitle / per-language titles are P06.
func titleText(s *spec.Spec) string {
	if s.Title == nil {
		return ""
	}
	// spec.TextOrTextObj is a custom union; fall back to fmt.Sprintf
	// to capture either the Single or Multi variant. P06 owns proper
	// extraction once we settle the title API.
	return fmt.Sprintf("%v", s.Title)
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
