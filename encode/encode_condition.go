package encode

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// applyConditions walks every condition-bearing channel in enc and
// either bakes a static (`test`-driven) outcome into each mark's Style
// or appends a selection-driven ConditionalAttr to Mark.Conditions.
//
// The "otherwise" branch reuses each channel's own value/field, which
// the per-mark encoders have already resolved into Style — so this
// function only needs to overlay matching conditions.
//
// The encoder calls this once per layer after marks.Encode produces
// the per-row mark list. Marks without a Datum (no source row, e.g.
// composite intermediate marks) are skipped silently.
func applyConditions(enc *spec.Encoding, tbl *table.Table, markList []scene.Mark) error {
	if enc == nil || len(markList) == 0 {
		return nil
	}
	for _, ch := range channelConditionsAt(enc) {
		attr := conditionAttrFor(ch.Name)
		for mi := range markList {
			if err := applyConditionsToMark(&markList[mi], tbl, ch, attr); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyConditionsToMark evaluates each condition entry against the
// mark's datum row. The first entry that matches wins:
//   - test-form: evaluate the structured predicate against the row; a
//     true result → bake the entry's value into the mark's Style; later
//     entries skipped.
//   - selection-form: append a ConditionalAttr with the channel's
//     pre-resolved fallback as Otherwise; later entries skipped.
//
// Selection-driven entries can stack with one static entry that
// preceded them only when the static entry did not match — they live
// in the Conditions slice in declaration order. The first matching
// static entry short-circuits the whole list.
func applyConditionsToMark(m *scene.Mark, tbl *table.Table, ch conditionChannel, attr string) error {
	if m == nil {
		return nil
	}
	env := datumEnv(m, tbl)
	otherwise := currentStyleValue(m, attr)
	for i, entry := range ch.Cond.Entries() {
		switch {
		case entry.Test != nil:
			if entry.Test.EvalRow(env) {
				if entry.Value != nil {
					if err := applyStyleAttr(&m.Style, attr, entry.Value); err != nil {
						return prismerrors.New(
							"PRISM_ENCODE_001",
							fmt.Sprintf("Condition value on channel %s entry[%d] could not be applied: %v.", ch.Name, i, err),
							map[string]any{"Channel": ch.Name, "Entry": i, "Value": entry.Value, "Reason": err.Error()},
						)
					}
				}
				return nil
			}
		case entry.Selection != "":
			when := entry.Value
			if when == nil {
				// Selection-form without value inherits the channel's
				// own field via the row's datum. Carry the row value
				// through unchanged.
				when = lookupRowField(m, tbl, ch.Common.Field)
			}
			m.Conditions = append(m.Conditions, scene.ConditionalAttr{
				Attr:      attr,
				Selection: entry.Selection,
				WhenValue: when,
				Otherwise: otherwise,
			})
			return nil
		}
	}
	return nil
}

type conditionChannel struct {
	Name   string
	Common *spec.ChannelCommon
	Cond   *spec.Condition
}

// channelConditionsAt enumerates only the condition-bearing channels
// in enc. Mirrors the helper in validate/rules but kept private to
// encode so encoders don't drag in the rules package.
func channelConditionsAt(enc *spec.Encoding) []conditionChannel {
	if enc == nil {
		return nil
	}
	var out []conditionChannel
	add := func(name string, common *spec.ChannelCommon) {
		if common == nil || common.Condition == nil {
			return
		}
		out = append(out, conditionChannel{Name: name, Common: common, Cond: common.Condition})
	}
	if enc.X != nil {
		add("x", &enc.X.ChannelCommon)
	}
	if enc.Y != nil {
		add("y", &enc.Y.ChannelCommon)
	}
	if enc.X2 != nil {
		add("x2", &enc.X2.ChannelCommon)
	}
	if enc.Y2 != nil {
		add("y2", &enc.Y2.ChannelCommon)
	}
	if enc.Theta != nil {
		add("theta", &enc.Theta.ChannelCommon)
	}
	if enc.Radius != nil {
		add("radius", &enc.Radius.ChannelCommon)
	}
	if enc.Color != nil {
		add("color", &enc.Color.ChannelCommon)
	}
	if enc.Fill != nil {
		add("fill", &enc.Fill.ChannelCommon)
	}
	if enc.Stroke != nil {
		add("stroke", &enc.Stroke.ChannelCommon)
	}
	if enc.Opacity != nil {
		add("opacity", &enc.Opacity.ChannelCommon)
	}
	if enc.Size != nil {
		add("size", &enc.Size.ChannelCommon)
	}
	if enc.Shape != nil {
		add("shape", &enc.Shape.ChannelCommon)
	}
	return out
}

// conditionAttrFor maps a channel name to the scene-IR attribute it
// drives. Channels without a direct visual attribute (theta, radius,
// shape today) return "" — the encoder skips them with a warning at
// some point; for now we treat as fill.
func conditionAttrFor(channel string) string {
	switch channel {
	case "color", "fill":
		return "fill"
	case "stroke":
		return "stroke"
	case "opacity":
		return "opacity"
	case "size":
		return "size"
	default:
		return "fill"
	}
}

// datumEnv builds the per-row value map the predicate evaluator sees.
// Datum.Fields is typically nil (D077 keeps the JSON payload small);
// we pull every column value at the row's index from the upstream
// table so test predicates can reference any field the spec used.
func datumEnv(m *scene.Mark, tbl *table.Table) map[string]any {
	env := map[string]any{}
	if m.Datum == nil {
		return env
	}
	env["__row__"] = m.Datum.RowID
	for k, v := range m.Datum.Fields {
		env[k] = v
	}
	if tbl == nil {
		return env
	}
	row := int(m.Datum.RowID)
	for _, name := range tbl.FieldNames() {
		if _, exists := env[name]; exists {
			continue
		}
		col, ok := tbl.Column(name)
		if !ok || row < 0 || row >= col.Len() {
			continue
		}
		env[name] = col.ValueAt(row)
	}
	return env
}

func lookupRowField(m *scene.Mark, tbl *table.Table, field string) any {
	if m == nil || m.Datum == nil || field == "" || tbl == nil {
		return nil
	}
	col, ok := tbl.Column(field)
	if !ok {
		return nil
	}
	row := int(m.Datum.RowID)
	if row < 0 || row >= col.Len() {
		return nil
	}
	return col.ValueAt(row)
}

func currentStyleValue(m *scene.Mark, attr string) any {
	switch attr {
	case "fill":
		if m.Style.Fill != nil {
			return m.Style.Fill.Hex()
		}
		return nil
	case "stroke":
		if m.Style.Stroke != nil {
			return m.Style.Stroke.Hex()
		}
		return nil
	case "opacity":
		return m.Style.Opacity
	default:
		return nil
	}
}

// applyStyleAttr writes value into style at attr. Hex strings parse
// via scene.ColorFromHex; floats land on Opacity / StrokeWidth.
func applyStyleAttr(style *scene.Style, attr string, value any) error {
	switch attr {
	case "fill":
		c, err := coerceColor(value)
		if err != nil {
			return err
		}
		style.Fill = c
	case "stroke":
		c, err := coerceColor(value)
		if err != nil {
			return err
		}
		style.Stroke = c
	case "opacity":
		f, err := coerceFloat(value)
		if err != nil {
			return err
		}
		style.Opacity = f
	case "size":
		// size doesn't have a direct Style slot (mark-specific). Skip
		// gracefully — the static value would need per-geom plumbing.
		return nil
	}
	return nil
}

// applyChannelBaseValue applies a mark-style channel's field-less
// literal Value (schema: encoding.schema.json's mark_channel/
// channel_base "value" property, "Constant literal value; alternative
// to field for a constant encoding") into style. attr is the same
// scene-IR attribute name conditionAttrFor maps the channel to
// (fill/stroke/opacity).
//
// This is the "otherwise" fallback applyConditionsToMark's docstring
// assumes is already resolved into Style before it runs. It wasn't:
// a field-less color/fill/stroke/opacity channel was decoded off the
// spec but never consumed by any per-mark encoder (colorChannel is
// only built when Field != ""), so a channel combining a bare `value`
// with a `condition` list rendered every non-matching row in the
// mark-type theme default instead of the spec's declared base color
// (P16 gallery sweep finding: conditions/test_predicate.svg and
// conditions/brush_highlight.svg both showed the default `#4c78a8`
// instead of their declared "#94a3b8"/"#cbd5e1" `value`). Applying it
// unconditionally — not just when a condition is also present — keeps
// the literal-value channel form correct standalone too, matching the
// schema's stated semantics.
//
// Intentionally narrow: only the mark-style channels the condition
// system already round-trips (color/fill/stroke/opacity). Position
// channels (x/y/x2/y2) and size/shape's literal-value forms are a
// larger, separate generalization (position values bypass scale
// resolution entirely; size has no direct Style slot per
// applyStyleAttr above) — out of scope here.
func applyChannelBaseValue(style *scene.Style, common *spec.ChannelCommon, attr string) error {
	if common == nil || common.Field != "" || common.Value == nil {
		return nil
	}
	return applyStyleAttr(style, attr, common.Value)
}

// applyMarkChannelBaseValues resolves enc's color/fill/stroke/opacity
// channels' field-less literal Value into style, in that order (color
// and fill both target the "fill" attr; a spec setting both is
// unusual but fill — the more specific channel — wins by running
// last). Called once per mark/layer before marks.Encode, alongside
// applyMarkDef, so a bare `{"value": "#hex"}` channel paints the mark
// even with no field-driven colorChannel. See applyChannelBaseValue.
func applyMarkChannelBaseValues(style *scene.Style, enc *spec.Encoding) error {
	if enc == nil {
		return nil
	}
	channels := []struct {
		ch   *spec.MarkChannel
		attr string
	}{
		{enc.Color, "fill"},
		{enc.Fill, "fill"},
		{enc.Stroke, "stroke"},
		{enc.Opacity, "opacity"},
	}
	for _, c := range channels {
		if c.ch == nil {
			continue
		}
		if err := applyChannelBaseValue(style, &c.ch.ChannelCommon, c.attr); err != nil {
			return prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("Channel base value could not be applied: %v.", err),
				map[string]any{"Field": "<value>", "Source": "<encoding>", "Reason": err.Error()},
			)
		}
	}
	return nil
}

func coerceColor(v any) (*scene.Color, error) {
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("expected hex string, got %T", v)
	}
	return scene.ColorFromHex(s)
}

func coerceFloat(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	}
	return 0, fmt.Errorf("expected number, got %T", v)
}
