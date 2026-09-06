package encode

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// applyCategoryStyles consults t.CategoryStyles for any mark channel
// bound to a field with a matching entry, resolving each datum's
// style by looking up its field value in the nested field→value→style
// map and merging the matched MarkStyle onto the mark's already
// -resolved style. Mirrors encode_condition.go's per-row resolution
// (applyConditions) but is simpler — CategoryStyles is a direct field
// -value lookup, not a predicate evaluation.
//
// Precedence: the caller MUST run applyCategoryStyles BEFORE
// applyConditions in the encode pipeline. A spec-level spec.Condition
// targeting the same field/value on the same channel is applied
// afterward and unconditionally overwrites whichever style attrs it
// resolves, so "later write wins" gives the condition precedence over
// the theme-level CategoryStyle for any datum both apply to — the
// precedence the interview/PRD locked in (explicit spec beats theme
// default).
//
// Data whose field value has no matching CategoryStyles entry is left
// completely untouched — it keeps whatever style earlier stages
// (theme mark default + spec.MarkDef + channel base value) already
// resolved.
func applyCategoryStyles(enc *spec.Encoding, tbl *table.Table, t *theme.Theme, markList []scene.Mark) {
	if enc == nil || t == nil || len(t.CategoryStyles) == 0 || len(markList) == 0 || tbl == nil {
		return
	}
	for _, field := range categoryStyleFieldsAt(enc) {
		values, ok := t.CategoryStyles[field]
		if !ok || len(values) == 0 {
			continue
		}
		for mi := range markList {
			applyCategoryStyleToMark(&markList[mi], tbl, field, values, t)
		}
	}
}

// categoryStyleFieldsAt enumerates the distinct field names bound
// across every channel in enc — the same channel set
// encode_condition.go's channelConditionsAt walks (position channels
// plus the mark-style channels), since theme.CategoryStyles is keyed
// by field name rather than by channel: any channel binding a field
// with a matching entry is eligible, per the story's "any mark channel
// bound to a field" acceptance criterion.
func categoryStyleFieldsAt(enc *spec.Encoding) []string {
	if enc == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(common *spec.ChannelCommon) {
		if common == nil || common.Field == "" || seen[common.Field] {
			return
		}
		seen[common.Field] = true
		out = append(out, common.Field)
	}
	if enc.X != nil {
		add(&enc.X.ChannelCommon)
	}
	if enc.Y != nil {
		add(&enc.Y.ChannelCommon)
	}
	if enc.X2 != nil {
		add(&enc.X2.ChannelCommon)
	}
	if enc.Y2 != nil {
		add(&enc.Y2.ChannelCommon)
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
	if enc.Fill != nil {
		add(&enc.Fill.ChannelCommon)
	}
	if enc.Stroke != nil {
		add(&enc.Stroke.ChannelCommon)
	}
	if enc.Opacity != nil {
		add(&enc.Opacity.ChannelCommon)
	}
	if enc.Size != nil {
		add(&enc.Size.ChannelCommon)
	}
	if enc.Shape != nil {
		add(&enc.Shape.ChannelCommon)
	}
	return out
}

// applyCategoryStyleToMark resolves m's row value for field against
// values (theme.CategoryStyles[field]) and, on a match, merges the
// matched MarkStyle onto m's style. MergeMarkStyle(nil, cs) both
// deep-clones cs (so applyThemeMarkStyle never aliases the theme's own
// map) and gives the "merge the matching MarkStyle onto the base mark
// style" semantics the story calls for: fields the CategoryStyle entry
// doesn't set stay nil/empty and applyThemeMarkStyle leaves m.Style's
// existing value for that attr untouched, so only the fields the
// theme author actually specified move.
func applyCategoryStyleToMark(m *scene.Mark, tbl *table.Table, field string, values map[string]*theme.MarkStyle, t *theme.Theme) {
	if m == nil || m.Datum == nil {
		return
	}
	raw := lookupRowField(m, tbl, field)
	if raw == nil {
		return
	}
	cs, ok := values[stringifyCategoryValue(raw)]
	if !ok || cs == nil {
		return
	}
	merged := theme.MergeMarkStyle(nil, cs)
	applyThemeMarkStyle(&m.Style, merged, t, nil, nil)
}

// stringifyCategoryValue renders a row value the way theme.CategoryStyles
// keys it: field values are stringified (docs/src/concepts/themes.md's
// "Category styles" section — nominal/ordinal string categories are the
// documented use case). A plain string round-trips verbatim; anything
// else falls back to fmt's default formatting.
func stringifyCategoryValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
