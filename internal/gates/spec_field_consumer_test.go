// spec_field_consumer_test.go is the dead-spec-field gate (E7-S3).
//
// Prism advertises its spec surface twice — once as a Go struct in spec/ and
// once as a JSON Schema in schema/v1/ — and for ~50 fields neither
// advertisement was true: the key decoded, validated, and then reached no
// consumer at all. `{"scale": {"zero": false}}` produced no error, no
// warning, and a chart with zero still in the domain. This gate makes that
// class of bug fail the build instead of shipping.
//
// The rule: every exported, JSON-tagged field on a spec/ struct must have at
// least one type-checked reference from outside spec/, or carry an entry in
// knownInertSpecFields explaining why it does not. The allowlist is shaped
// like cmd/prism/cmd_mcp.go's mcpOptionsAllowlist — a reason string per
// entry, never a bare name — so a future reader can tell a decision from an
// oversight.
//
// What this gate CANNOT catch, stated plainly because a gate whose limits are
// unknown is worse than none:
//
//   - It proves a field is *read*, not that reading it changes the output.
//     `encoding.shape` is read (the inert detector tests the pointer) and
//     still draws nothing. Whole-channel and whole-block deadness is the
//     inert-table sync gate's job (inert_table_sync_test.go) and, at
//     runtime, encode/inert.go's.
//   - A field read only through an accessor method on its own spec type
//     looks dead here, because the read happens inside spec/. MarkDef.Type
//     is the standing example — see its allowlist entry.
//   - It says nothing about the JS renderer: the vendored ESM consumes the
//     Scene IR, not the spec, so a spec field cannot have a JS-only
//     consumer. Scene IR fields are a separate surface with the same drift
//     risk and are out of scope here.
//   - Build-tagged files are covered for the host and js/wasm platforms
//     only. A file gated behind some third build tag would be invisible.
package gates

import (
	"sort"
	"strings"
	"testing"
)

// knownInertSpecFields lists the exported, JSON-tagged spec fields that
// reach no consumer outside spec/, each with the reason it is tolerated.
//
// Adding an entry is a decision to ship an advertised key that does nothing.
// Prefer wiring the field, or deleting it from both the struct and
// schema/v1/. If you do add one, say *why* it is inert and what an author
// should use instead — this map is the only record that the deadness was
// noticed rather than missed.
//
// The gate also fails on a stale entry: once a field gains a consumer its
// entry here must go, so the list cannot quietly rot into a blanket
// exemption.
var knownInertSpecFields = map[string]string{
	// --- Top-level chart properties -------------------------------------
	"Spec.Width": "chart size comes from encode.EncodeOpts (the CLI --width flag, the rpc render request, " +
		"or the web component's attribute); the spec-level width is decoded and dropped.",
	"Spec.Height": "chart size comes from encode.EncodeOpts (the CLI --height flag, the rpc render request, " +
		"or the web component's attribute); the spec-level height is decoded and dropped.",
	"Spec.Padding": "plot padding is computed by encode/layout.go from the resolved axis placement and " +
		"theme metrics; the spec-level padding block is decoded and dropped.",
	"Spec.Background": "the chart background comes from the resolved theme (theme/css.go emits it as a CSS " +
		"variable); the spec-level background is decoded and dropped.",
	"Spec.Subtitle": "only `title` becomes a scene.TextElement; no encoder emits a subtitle element, so the " +
		"key renders nothing.",
	"Spec.Description": "an authoring / provenance note. Nothing emits it — render/svg has no <desc> or " +
		"aria-description emitter.",

	// --- Rich text object ------------------------------------------------
	// encode's title path reads Title.Obj.Text and nothing else; title
	// typography comes from the theme's title block.
	"TextObj.Font":       "title typography comes from the theme title block; only `text` is read off the rich title object.",
	"TextObj.FontSize":   "title typography comes from the theme title block; only `text` is read off the rich title object.",
	"TextObj.FontWeight": "title typography comes from the theme title block; only `text` is read off the rich title object.",
	"TextObj.FontStyle":  "title typography comes from the theme title block; only `text` is read off the rich title object.",
	"TextObj.Color":      "the title colour comes from the theme title block; only `text` is read off the rich title object.",
	"TextObj.Align":      "the title anchor is fixed by encode/layout.go; only `text` is read off the rich title object.",
	"TextObj.Baseline":   "the title anchor is fixed by encode/layout.go; only `text` is read off the rich title object.",

	// --- Transform blocks ------------------------------------------------
	"BinTransform.Bin": "plan/build/build.go always constructs nodes.BinParams{Auto: true}, so the bin " +
		"parameter object is decoded and dropped — bin edges are always chosen automatically.",
	"BinSpec.Maxbins": "BinSpec is the typed shape behind BinTransform.Bin, which plan/build never reads. " +
		"mark_def.maxbins is the histogram mark's own, honoured knob.",
	"BinSpec.Step":   "BinSpec is the typed shape behind BinTransform.Bin, which plan/build never reads.",
	"BinSpec.Extent": "BinSpec is the typed shape behind BinTransform.Bin, which plan/build never reads.",
	"CalcExpr.If": "a decode-time alias for `case`: CalcExpr.UnmarshalJSON copies If into Case and clears " +
		"it, so nothing outside spec/ can observe the field.",

	// --- Channel-level keys ----------------------------------------------
	"ChannelCommon.Bin": "channel-level binning is not implemented — no synthetic bin node is injected. Use " +
		"the `bin` transform, or mark_def.maxbins on a histogram.",
	"ChannelCommon.Sort": "channel-level sorting is not implemented. A discrete scale's category order " +
		"follows the data (or an explicit scale.domain); use the `sort` transform or the `order` channel.",
	"TextChannel.Type": "a text / tooltip channel formats the raw cell value and resolves no scale, so its " +
		"declared type is not read. (The `type` on a position / mark channel is read.)",
	"TextChannel.Title": "a text / tooltip channel has no axis or legend to title. A *table column* title IS " +
		"honoured — that is ChannelCommon.Title reached through spec.TableColumn, a different field.",
	"OrderChannelEntry.Type": "the order channel sorts on the column's own table kind; the declared type is " +
		"not read.",
	"DetailChannelEntry.Type": "the detail channel only partitions rows on the tuple of its field values " +
		"(encode/marks/group.go) and resolves no scale, so the declared type is not read.",
	"ConditionTest.Type": "a condition entry supplies a literal value or a field binding applied against the " +
		"channel's own resolved scale; the entry's declared type is not read.",
	"ConditionTest.Scale": "a condition entry cannot carry a scale of its own — encode/encode_condition.go " +
		"evaluates it against the channel's resolved scale.",

	// --- Facet ------------------------------------------------------------
	"FacetChannel.Type": "encode/encode_facet.go reads only the facet channel's `field`; cell order follows " +
		"the column's own kind, so the declared type is not read.",
	"FacetChannel.Sort": "facet cell order follows the data's category order; a per-facet sort is not " +
		"implemented. Order the rows upstream with the `sort` transform.",
	"FacetChannel.Header":     "facet headers render the cell's own category value; the header block is decoded and dropped.",
	"FacetChannelHead.Title":  "see FacetChannel.Header — the whole header block is inert.",
	"FacetChannelHead.Labels": "see FacetChannel.Header — the whole header block is inert.",

	// --- Cross-child resolution ------------------------------------------
	"Resolve.Legend": "cross-child resolution is applied to scales and axes; a legend follows the scale it " +
		"describes, so `resolve.legend` is decoded and dropped.",
	"ResolveChannelMap.Fill":   "resolution is looked up for x / y / x2 / y2 / color / opacity / size / shape; `fill` is not a resolved channel.",
	"ResolveChannelMap.Stroke": "resolution is looked up for x / y / x2 / y2 / color / opacity / size / shape; `stroke` is not a resolved channel.",
	"ResolveChannelMap.Theta":  "polar scales are resolved per cell and never shared across composition children.",
	"ResolveChannelMap.Radius": "polar scales are resolved per cell and never shared across composition children.",

	// --- Selection --------------------------------------------------------
	"PointSelection.Type": "the `type` key is the union discriminator, consumed inside " +
		"spec.Selection.UnmarshalJSON; downstream code branches on which variant pointer is set.",
	"IntervalSelection.Type": "the `type` key is the union discriminator, consumed inside " +
		"spec.Selection.UnmarshalJSON; downstream code branches on which variant pointer is set.",
	"PointSelection.Toggle": "encode/selection_build.go materialises the binding (fields / encodings / on); " +
		"toggle semantics are not implemented on either side.",
	"PointSelection.Nearest": "encode/selection_build.go materialises the binding only; nearest-point " +
		"resolution is not implemented on either side.",
	"PointSelection.Empty": "encode/selection_build.go materialises the binding only; the empty-selection " +
		"policy is not implemented on either side.",
	"IntervalSelection.On": "the point variant's `on` is read; the interval variant's is not — brush " +
		"gestures are fixed in the vendored web component.",
	"IntervalSelection.Translate": "brush translate / zoom gestures are not implemented; only the interval's " +
		"`encodings` binding reaches the Scene IR.",
	"IntervalSelection.Zoom": "brush translate / zoom gestures are not implemented; only the interval's " +
		"`encodings` binding reaches the Scene IR.",
	"IntervalSelection.Mark":       "the brush rectangle is styled from the theme, not from the spec; see IntervalMarkProp.",
	"IntervalMarkProp.Fill":        "the brush rectangle is styled from the theme; IntervalSelection.Mark never reaches an encoder.",
	"IntervalMarkProp.FillOpacity": "the brush rectangle is styled from the theme; IntervalSelection.Mark never reaches an encoder.",
	"IntervalMarkProp.Stroke":      "the brush rectangle is styled from the theme; IntervalSelection.Mark never reaches an encoder.",
	"IntervalMarkProp.StrokeWidth": "the brush rectangle is styled from the theme; IntervalSelection.Mark never reaches an encoder.",

	// --- Theme ------------------------------------------------------------
	"ThemeOverride.Padding": "a legacy flat override kept for v1 back-compat. Chart padding is computed by " +
		"encode/layout.go and never read from the spec — see Spec.Padding.",

	// --- mark_def ---------------------------------------------------------
	"MarkDef.Type": "READ, but through an accessor: every consumer calls spec.Mark.TypeName(), which reads " +
		"this field inside spec/. The gate sees typed references only, and this one is intra-package. This " +
		"is the standing example of the gate's accessor blind spot.",
	"MarkDef.Shape": "KNOWN DEAD (user decision, E7-S1): the point renderer always emits a circle for a " +
		"mark, and scene.PointGeom.Shape has no producer. The swatch emitter added in E3-S4 draws all five " +
		"shapes, so wiring this is now plumbing in encode/marks/point.go.",
	"MarkDef.Tooltip": "KNOWN DEAD (user decision, E7-S1): tooltips come from the `tooltip` encoding " +
		"channel; the mark-level switch has no reader.",
	"MarkDef.Layout": "KNOWN DEAD (user decision, E7-S1): the network encoder always calls ForceLayout, so " +
		"neither \"force\" nor \"random\" changes anything.",
}

func TestPrismSpecFieldsHaveConsumers(t *testing.T) {
	res := analyzeSpecConsumers(t)

	var dead, stale []string
	for key, f := range res.fields {
		_, allowed := knownInertSpecFields[key]
		switch {
		case res.consumed[key] && allowed:
			stale = append(stale, key+" (json: "+f.JSON+")")
		case !res.consumed[key] && !allowed:
			dead = append(dead, key+" (json: "+f.JSON+")")
		}
	}
	sort.Strings(dead)
	sort.Strings(stale)

	if len(dead) > 0 {
		t.Errorf("%d spec field(s) decode but reach no consumer outside spec/:\n  %s\n\n"+
			"A field here is advertised by spec/ (and usually by schema/v1/) and then does nothing: it "+
			"round-trips through the spec unchanged and changes no pixel. Fix it one of three ways, in "+
			"order of preference:\n"+
			"  1. Wire it — read it where the feature belongs (encode/, plan/build/, validate/rules/) and "+
			"update the docs page the Update Demand names.\n"+
			"  2. Delete it — from the Go struct AND from schema/v1/, so neither surface advertises it.\n"+
			"  3. Record it — add an entry to knownInertSpecFields in "+
			"internal/gates/spec_field_consumer_test.go saying why it is inert and what an author should "+
			"use instead. A bare name is not enough; the reason string is the point.",
			len(dead), strings.Join(dead, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("%d knownInertSpecFields entr(ies) name a field that now HAS a consumer:\n  %s\n\n"+
			"Delete the entry. Leaving it would exempt a live field from the gate forever, which is how an "+
			"allowlist rots into a blanket opt-out.",
			len(stale), strings.Join(stale, "\n  "))
	}

	for key, reason := range knownInertSpecFields {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("knownInertSpecFields[%q] carries no reason", key)
		}
		if _, ok := res.fields[key]; !ok {
			t.Errorf("knownInertSpecFields[%q] names no exported, JSON-tagged spec field — the field was "+
				"renamed or removed; drop the entry", key)
		}
	}
}
