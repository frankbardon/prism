package encode

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// InertFieldWarnings (E7-S1) reports every spec key that decodes
// cleanly, survives validation, and then reaches no consumer — the
// silent no-op this effort exists to eliminate. Each warning names
// the field path, the mark or channel it was written on, and why
// nothing reads it, so an author can act on it without reading the
// encoder.
//
// It is a pure function of the spec: no table, no theme, no resolved
// scale. That keeps it callable from the one place in the pipeline
// that owns the whole tree (Encode / EncodeComposite at the top of
// the tree, never a composition cell), which is what stops a layer
// from being reported once per cell the way PRISM_WARN_AXIS_CONFIG_
// CONFLICT was before E1-S2 moved it to a single reporter.
//
// Silence is the contract for anything honoured, anything rejected at
// validate (a rejection is already visible), and anything inert by
// design — `json:"-"` bindings (Data.Source, ChannelCommon.FieldRef)
// and validate-only fields (Axis.Format / Legend.Format, which
// PRISM_SPEC_011 reads). A false positive here trains authors to
// ignore warnings, so a check lands only for a surface verified dead
// end-to-end.
func InertFieldWarnings(s *spec.Spec) []scene.Warning {
	var out []scene.Warning
	walkInertSpec(s, "", "", &out)
	return out
}

// walkInertSpec recurses through the composition tree, prefixing each
// child's findings with its path (layer[0], concat[1], spec, …).
// parentMark carries the enclosing mark type so a layer child that
// declares only style properties is still checked against the mark it
// inherits.
func walkInertSpec(s *spec.Spec, path, parentMark string, out *[]scene.Warning) {
	if s == nil {
		return
	}
	markType := parentMark
	if mt := s.Mark.TypeName(); mt != "" {
		markType = mt
	}

	walkInertChildren(s.Layer, path, "layer", markType, out)
	walkInertChildren(s.Concat, path, "concat", markType, out)
	walkInertChildren(s.HConcat, path, "hconcat", markType, out)
	walkInertChildren(s.VConcat, path, "vconcat", markType, out)

	if s.ChildSpec != nil {
		childPath := joinInertPath(path, "spec")
		if s.Facet != nil {
			facetChildWarnings(s.ChildSpec, childPath, out)
		}
		walkInertSpec(s.ChildSpec, childPath, markType, out)
	}

	inertMarkDef(s.Mark, markType, path, out)
	inertEncoding(s.Encoding, markType, path, out)
}

func walkInertChildren(children []*spec.Spec, path, key, markType string, out *[]scene.Warning) {
	for i, child := range children {
		walkInertSpec(child, joinInertPath(path, fmt.Sprintf("%s[%d]", key, i)), markType, out)
	}
}

// joinInertPath appends one segment to a dotted spec path.
func joinInertPath(path, seg string) string {
	if path == "" {
		return seg
	}
	return path + "." + seg
}

// --- mark_def ------------------------------------------------------

// markDefOwners maps a mark_def property to the mark types whose
// encoder actually reads it. A property absent from this table is
// never reported — the table is an allowlist of *checked* keys, not a
// model of the whole mark_def, so a new property is silent until
// someone records where it is honoured. An empty owner list means no
// encoder reads the property on any mark.
//
// Every entry was derived from the reading call site, not from the
// schema: grep the mark encoders for `in.Mark.<Field>` to re-verify.
var markDefOwners = map[string][]string{
	// Polar geometry — encode/marks/arc.go.
	"inner_radius":       {"arc", "pie", "donut"},
	"outer_radius":       {"arc", "pie", "donut"},
	"inner_radius_ratio": {"arc", "pie", "donut"},
	"pad_angle":          {"arc", "pie", "donut"},
	// Text placement / typography — encode/marks/text.go.
	"dx":        {"text"},
	"dy":        {"text"},
	"align":     {"text"},
	"baseline":  {"text"},
	"angle":     {"text"},
	"font_size": {"text"},
	// Curve interpolation — encode/marks/curve.go, reached by the
	// line / area encoders and the spark marks that wrap them.
	"interpolate": {"line", "area", "sparkline", "sparkarea"},
	"tension":     {"line", "area", "sparkline", "sparkarea"},
	// Rounded rect corners — bar.go / progress.go / winloss.go;
	// sparkbar wraps encodeBar.
	"corner_radius": {"bar", "sparkbar", "progress", "winloss"},
	// Symbol / glyph size — point.go, geopoint.go, image.go, tick.go.
	"size": {"point", "geopoint", "image", "tick"},
	// Per-family inputs.
	"maxbins":           {"histogram"},
	"violin_resolution": {"violin"},
	"link_shape":        {"tree", "dendrogram"},
	"node_shape":        {"tree", "dendrogram", "network"},
	"node_size":         {"tree", "dendrogram", "network"},
	"iterations":        {"network"},
	"link_distance":     {"network"},
	"charge":            {"network"},
	"seed":              {"network"},
	"target":            {"bullet"},
	"bands":             {"bullet"},
	"comparative":       {"bullet"},
	"orientation":       {"bullet"},
	"total":             {"progress"},
	"thickness":         {"progress"},
	"point_last":        {"sparkline", "sparkbar", "sparkarea"},
	"point_extent":      {"sparkline", "sparkbar", "sparkarea"},
	"reference_band":    {"sparkline", "sparkbar", "sparkarea"},
	"page_size":         {"table"},
	"renderer":          {"custom"},
	"url":               {"image"},
	"path":              {"path"},
	// Declared, schema-advertised, read by nothing anywhere.
	"shape":   {},
	"tooltip": {},
	"layout":  {},
	// stroke_dash is ABSENT on purpose (E7-S4): applyMarkDef now
	// folds it into scene.Style.StrokeDash for every mark type and
	// render/svg emits stroke-dasharray, so it belongs with the other
	// universal style properties (fill, stroke, stroke_width, opacity,
	// fill_opacity, …) that this allowlist never names precisely
	// because no mark type can render them inert.
}

// markDefSet lists the mark_def properties this spec actually sets,
// keyed by wire name. Only keys present in markDefOwners are worth
// reporting, so the switch below covers exactly those.
func markDefSet(def *spec.MarkDef) []string {
	if def == nil {
		return nil
	}
	set := map[string]bool{
		"inner_radius":       def.InnerRadius != nil,
		"outer_radius":       def.OuterRadius != nil,
		"inner_radius_ratio": def.InnerRadiusRatio != nil,
		"pad_angle":          def.PadAngle != nil,
		"dx":                 def.Dx != nil,
		"dy":                 def.Dy != nil,
		"align":              def.Align != "",
		"baseline":           def.Baseline != "",
		"angle":              def.Angle != nil,
		"font_size":          def.FontSize != nil,
		"interpolate":        def.Interpolate != "",
		"tension":            def.Tension != nil,
		"corner_radius":      def.CornerRadius != nil,
		"size":               def.Size != nil,
		"maxbins":            def.Maxbins != nil,
		"violin_resolution":  def.ViolinResolution != nil,
		"link_shape":         def.LinkShape != "",
		"node_shape":         def.NodeShape != "",
		"node_size":          def.NodeSize != nil,
		"iterations":         def.Iterations != nil,
		"link_distance":      def.LinkDistance != nil,
		"charge":             def.Charge != nil,
		"seed":               def.Seed != nil,
		"target":             def.Target != nil,
		"bands":              len(def.Bands) > 0,
		"comparative":        def.Comparative != nil,
		"orientation":        def.Orientation != "",
		"total":              def.Total != nil,
		"thickness":          def.Thickness != nil,
		"point_last":         def.PointLast,
		"point_extent":       def.PointExtent,
		"reference_band":     def.ReferenceBand != nil,
		"page_size":          def.PageSize != nil,
		"renderer":           def.Renderer != "",
		"url":                def.URL != "",
		"path":               def.Path != "",
		"shape":              def.Shape != "",
		"tooltip":            def.Tooltip != nil,
		"layout":             def.Layout != "",
	}
	out := make([]string, 0, len(set))
	for k, present := range set {
		if present {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func inertMarkDef(m *spec.Mark, markType, path string, out *[]scene.Warning) {
	if m == nil || m.Def == nil {
		return
	}
	for _, prop := range markDefSet(m.Def) {
		owners, checked := markDefOwners[prop]
		if !checked || containsString(owners, markType) {
			continue
		}
		*out = append(*out, scene.Warning{
			Code: scene.WarnMarkDefInert,
			Message: fmt.Sprintf(
				"%s: mark_def %q is not read by the %q mark — it decodes and is discarded (%s).",
				joinInertPath(path, "mark."+prop), prop, markType, ownerPhrase(owners)),
			Details: map[string]any{
				"Path":     joinInertPath(path, "mark."+prop),
				"Property": prop,
				"Mark":     markType,
				"Owners":   ownerPhrase(owners),
			},
		})
	}
}

// ownerPhrase renders a mark_def property's owner list for a message.
func ownerPhrase(owners []string) string {
	if len(owners) == 0 {
		return "no mark encoder reads it on any mark type"
	}
	return "read by " + strings.Join(owners, ", ")
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// --- channels ------------------------------------------------------

// deadChannels are encoding channels no mark encoder reads at all.
// Each is decoded, schema-advertised and allowlisted by
// validate/rules/channel_for_mark.go, and then reaches marks.Inputs
// through no field — verified by grepping encode/ for the binding.
// `opacity` is intentionally absent: the heatmap encoder does read it,
// so it is handled per-mark below.
var deadChannels = map[string]string{
	"fill":   "no mark encoder reads the fill channel; use mark_def.fill for a constant, or the color channel for a data-driven fill",
	"stroke": "no mark encoder reads the stroke channel; use mark_def.stroke for a constant",
	"size":   "no mark encoder reads the size channel; mark_def.size sets a constant symbol size",
	"shape":  "no mark encoder reads the shape channel — every point renders as a circle",
}

func inertEncoding(enc *spec.Encoding, markType, path string, out *[]scene.Warning) {
	if enc == nil {
		return
	}
	encPath := joinInertPath(path, "encoding")

	markChannels := []struct {
		name string
		ch   *spec.MarkChannel
	}{
		{"color", enc.Color},
		{"fill", enc.Fill},
		{"stroke", enc.Stroke},
		{"opacity", enc.Opacity},
		{"size", enc.Size},
		{"shape", enc.Shape},
	}
	for _, mc := range markChannels {
		if mc.ch == nil {
			continue
		}
		chPath := joinInertPath(encPath, mc.name)
		if reason, dead := deadChannels[mc.name]; dead && !channelHasCondition(&mc.ch.ChannelCommon) {
			appendChannelInert(out, chPath, mc.name, markType, reason)
		}
		if mc.name == "opacity" && markType != "heatmap" && !channelHasCondition(&mc.ch.ChannelCommon) {
			appendChannelInert(out, chPath, mc.name, markType,
				"only the heatmap encoder reads a field-driven opacity channel; every other mark ignores it (mark_def.opacity sets a constant)")
		}
		inertChannelCommon(&mc.ch.ChannelCommon, mc.name, chPath, out)
		inertLegend(mc.ch, mc.name, chPath, out)
	}

	positions := []struct {
		name string
		ch   *spec.PositionChannel
	}{
		{"x", enc.X}, {"y", enc.Y}, {"x2", enc.X2}, {"y2", enc.Y2},
		{"theta", enc.Theta}, {"radius", enc.Radius},
	}
	for _, pc := range positions {
		if pc.ch == nil {
			continue
		}
		chPath := joinInertPath(encPath, pc.name)
		inertChannelCommon(&pc.ch.ChannelCommon, pc.name, chPath, out)
	}

	// Table columns carry the same channel shape. `title` IS read
	// there (it becomes the column header) and, since E7-S4, so is
	// `format` — encode/table.go runs it through the encode/format d3
	// subset into scene.TableRow.Display. The one surviving dead
	// combination is a format on a column bound to a sub-mark: that
	// column renders geometry, so there is no text for a specifier to
	// shape.
	for i, col := range enc.Columns {
		colPath := fmt.Sprintf("%s.columns[%d]", encPath, i)
		if col.Format != "" && col.Mark != "" {
			appendChannelInert(out, joinInertPath(colPath, "format"), "columns", markType,
				fmt.Sprintf("a table column bound to the %q sub-mark renders geometry, not text, so its format string is parsed by validate (PRISM_SPEC_011) and then read by nothing", col.Mark))
		}
	}
}

func channelHasCondition(common *spec.ChannelCommon) bool {
	return common != nil && common.Condition != nil
}

func appendChannelInert(out *[]scene.Warning, path, channel, markType, reason string) {
	*out = append(*out, scene.Warning{
		Code:    scene.WarnChannelInert,
		Message: fmt.Sprintf("%s: %s.", path, reason),
		Details: map[string]any{
			"Path":    path,
			"Channel": channel,
			"Mark":    markType,
			"Reason":  reason,
		},
	})
}

// inertChannelCommon reports the shared channel keys that reach no
// consumer, plus the channel's scale block.
func inertChannelCommon(common *spec.ChannelCommon, channel, chPath string, out *[]scene.Warning) {
	if common == nil {
		return
	}
	if common.Title != "" {
		where := "axis.title"
		if isMarkChannelName(channel) {
			where = "legend.title"
		}
		appendChannelInert(out, joinInertPath(chPath, "title"), channel, "",
			"a channel-level title is not read — the axis / legend title comes from the field name or from "+where)
	}
	if common.Format != "" && channel != "text" && channel != "tooltip" {
		appendChannelInert(out, joinInertPath(chPath, "format"), channel, "",
			"a channel-level format is validated (PRISM_SPEC_011) but read by nothing — use axis.format / legend.format, or the text / tooltip channel's own format")
	}
	inertScale(common.Scale, common.Type, channel, chPath, out)
}

func isMarkChannelName(channel string) bool {
	switch channel {
	case "color", "fill", "stroke", "opacity", "size", "shape":
		return true
	}
	return false
}

// --- scale ---------------------------------------------------------

// scaleFamilyOf resolves the scale family a channel lands on, from an
// explicit scale.type or (failing that) the channel's declared data
// type — the same two-step ResolveScaleWithOpts / ResolveScaleTyped
// make at encode time. It returns "" when the family cannot be known
// from the spec alone, which suppresses every family-conditional
// check rather than guessing.
func scaleFamilyOf(sc *spec.Scale, channelType string) string {
	if sc != nil && sc.Type != "" {
		switch sc.Type {
		case "linear", "log", "pow", "sqrt", "time", "band", "point", "ordinal":
			return sc.Type
		}
		return ""
	}
	switch channelType {
	case "quantitative":
		return "linear"
	case "temporal":
		return "time"
	case "nominal", "ordinal":
		return "band"
	}
	return ""
}

func isDiscreteFamily(family string) bool {
	switch family {
	case "band", "point", "ordinal":
		return true
	}
	return false
}

// inertScale reports scale-block properties the resolved family does
// not read. Colour-family channels are exempt from the palette keys
// (`range` / `scheme` / `interpolate`) because those ARE the colour
// cascade; position channels are not, and a `range` on one is already
// rejected at validate (PRISM_SPEC_045), so only `scheme` /
// `interpolate` are reported there.
func inertScale(sc *spec.Scale, channelType, channel, chPath string, out *[]scene.Warning) {
	if sc == nil {
		return
	}
	scPath := joinInertPath(chPath, "scale")
	family := scaleFamilyOf(sc, channelType)
	report := func(prop, reason string) {
		*out = append(*out, scene.Warning{
			Code: scene.WarnScaleFieldInert,
			Message: fmt.Sprintf("%s: scale %q is not read — %s.",
				joinInertPath(scPath, prop), prop, reason),
			Details: map[string]any{
				"Path":     joinInertPath(scPath, prop),
				"Property": prop,
				"Channel":  channel,
				"Family":   family,
				"Reason":   reason,
			},
		})
	}

	// Palette keys are colour-only.
	if channel != "color" {
		if sc.Scheme != "" {
			report("scheme", "a named colour scheme is read by the colour palette cascade only")
		}
		if sc.Interpolate != "" {
			report("interpolate", "colour-space interpolation is read by the colour palette cascade only")
		}
		if sc.Range != nil && channel != "x" && channel != "y" && channel != "x2" && channel != "y2" {
			report("range", "only an inline list of colour strings on the colour channel is lifted; numeric (size / opacity) and named-reference ranges are read by nothing")
		}
	} else if sc.Range != nil && colorRangeList(sc.Range) == nil {
		report("range", "only an inline list of colour strings is lifted; the numeric and named-reference range forms are read by nothing")
	}

	if family == "" {
		return
	}
	if isDiscreteFamily(family) {
		if sc.Zero != nil {
			report("zero", "zero-forcing shapes a continuous domain; a "+family+" domain is a category list")
		}
		if sc.Nice != nil {
			report("nice", "nice rounding shapes a continuous domain; a "+family+" domain is a category list")
		}
		if sc.Clamp != nil {
			report("clamp", "clamping bounds a continuous output; a "+family+" scale has no out-of-domain pixel to clamp")
		}
		if sc.Base != nil {
			report("base", "a logarithm base applies to a log scale only")
		}
		if sc.Exponent != nil {
			report("exponent", "an exponent applies to a pow scale only")
		}
		if family == "ordinal" {
			if sc.Padding != nil {
				report("padding", "an ordinal scale spreads its categories evenly across the range and reads no padding")
			}
			if sc.PaddingInner != nil {
				report("padding_inner", "an ordinal scale spreads its categories evenly across the range and reads no padding")
			}
			if sc.PaddingOuter != nil {
				report("padding_outer", "an ordinal scale spreads its categories evenly across the range and reads no padding")
			}
			if sc.Align != nil {
				report("align", "an ordinal scale spreads its categories evenly across the range and reads no alignment")
			}
		}
		if family == "point" && sc.PaddingInner != nil {
			report("padding_inner", "a point scale has no band to pad inside; use padding / padding_outer")
		}
		return
	}

	// Continuous families.
	if sc.Padding != nil {
		report("padding", "band / point padding shapes a discrete range; a "+family+" scale spans its range continuously")
	}
	if sc.PaddingInner != nil {
		report("padding_inner", "band padding shapes a discrete range; a "+family+" scale spans its range continuously")
	}
	if sc.PaddingOuter != nil {
		report("padding_outer", "band padding shapes a discrete range; a "+family+" scale spans its range continuously")
	}
	if sc.Align != nil {
		report("align", "alignment positions categories inside a discrete range; a "+family+" scale spans its range continuously")
	}
	if sc.Base != nil && family != "log" {
		report("base", "a logarithm base applies to a log scale only")
	}
	if sc.Exponent != nil && family != "pow" {
		if family == "sqrt" {
			report("exponent", "a sqrt scale pins its exponent at 0.5; use \"type\": \"pow\" to choose one")
		} else {
			report("exponent", "an exponent applies to a pow scale only")
		}
	}
	if sc.Zero != nil && (family == "log" || family == "time") {
		if family == "log" {
			report("zero", "a log domain cannot contain zero, so zero-forcing never runs on it")
		} else {
			report("zero", "zero-forcing is not applied to a time domain")
		}
	}
}

// --- legend --------------------------------------------------------

// inertLegend reports a colour channel that gets no legend at all.
//
// The per-property half of this check is gone. E7-S1 shipped a
// deadLegendProps table (type / direction / symbol_type / symbol_size /
// tick_count) and E3-S4 landed the consumers for every one of them in
// the same wave: ResolveLegendContent reads all five. The table had
// become a false positive — warning that a key does nothing while the
// rendered SVG obeyed it — so it is retired along with
// PRISM_WARN_LEGEND_FIELD_INERT (see errors/codes.go).
// internal/gates/inert_table_sync_test.go is what catches this class
// now: it cross-checks the properties the detector calls dead against
// the typed-consumer analysis, so a detector entry cannot outlive the
// gap it describes, and it asserts that every `legend` schema property
// still has a consumer.
func inertLegend(ch *spec.MarkChannel, channel, chPath string, out *[]scene.Warning) {
	if ch == nil {
		return
	}
	if channel == "color" && isContinuousChannelType(ch.Type) && !ch.LegendHidden {
		*out = append(*out, scene.Warning{
			Code: scene.WarnLegendNotBuilt,
			Message: fmt.Sprintf(
				"%s: a %s colour channel renders with no legend — the symbol legend needs discrete categories and no code path builds a gradient legend, so the colour encoding has no key.",
				chPath, ch.Type),
			Details: map[string]any{
				"Path":    chPath,
				"Channel": channel,
				"Type":    ch.Type,
			},
		})
	}
}

func isContinuousChannelType(t string) bool {
	return t == "quantitative" || t == "temporal"
}

// --- facet ---------------------------------------------------------

// facetChildWarnings reports the three encoding features a facet
// child asks for and never gets. plan/build's buildFacetComposite
// strips the child encoding before Build, so injectEncodingAggregate,
// injectEncodingStack and injectEncodingOrder never see it — one
// limitation blocking three features (E5-S2 / E5-S4 both recorded it).
func facetChildWarnings(child *spec.Spec, childPath string, out *[]scene.Warning) {
	if child == nil || child.Encoding == nil {
		return
	}
	enc := child.Encoding
	report := func(feature, reason string) {
		*out = append(*out, scene.Warning{
			Code: scene.WarnFacetChildSkipped,
			Message: fmt.Sprintf(
				"%s: a facet child's %s is dropped — the child encoding is stripped before the plan is built, so %s.",
				childPath, feature, reason),
			Details: map[string]any{
				"Path":    childPath,
				"Feature": feature,
				"Reason":  reason,
			},
		})
	}
	if facetChildAggregates(enc) {
		report("channel-level aggregate",
			"no synthetic group-by node is injected and the raw rows are drawn unaggregated")
	}
	if enc.X != nil && enc.X.Stack != nil || enc.Y != nil && enc.Y.Stack != nil {
		report("stack", "no stack node is injected and the series overdraw instead of stacking")
	}
	if enc.Order != nil {
		report("order channel", "no sort node is injected and the rows keep their upstream order")
	}
}

// facetChildAggregates reports whether any channel of a facet child
// asks for a channel-level aggregate.
func facetChildAggregates(enc *spec.Encoding) bool {
	commons := []*spec.ChannelCommon{}
	add := func(c *spec.ChannelCommon) {
		if c != nil {
			commons = append(commons, c)
		}
	}
	if enc.X != nil {
		add(&enc.X.ChannelCommon)
	}
	if enc.Y != nil {
		add(&enc.Y.ChannelCommon)
	}
	if enc.Theta != nil {
		add(&enc.Theta.ChannelCommon)
	}
	if enc.Radius != nil {
		add(&enc.Radius.ChannelCommon)
	}
	if enc.Color != nil {
		add(&enc.Color.ChannelCommon)
	}
	if enc.Text != nil && enc.Text.Aggregate != "" {
		return true
	}
	for _, c := range commons {
		if c.Aggregate != "" {
			return true
		}
	}
	return false
}
