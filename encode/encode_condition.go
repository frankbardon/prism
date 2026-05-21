package encode

import (
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

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
		// Compile test expressions once; reuse across rows.
		programs := make([]*vm.Program, len(ch.Cond.Entries()))
		for i, entry := range ch.Cond.Entries() {
			if entry.Test == "" {
				continue
			}
			prog, err := expr.Compile(entry.Test, expr.AllowUndefinedVariables())
			if err != nil {
				return prismerrors.New(
					"PRISM_ENCODE_001",
					fmt.Sprintf("Condition test on channel %s entry[%d] failed to compile: %v.", ch.Name, i, err),
					map[string]any{"Channel": ch.Name, "Entry": i, "Expression": entry.Test, "Reason": err.Error()},
				)
			}
			programs[i] = prog
		}
		attr := conditionAttrFor(ch.Name)
		for mi := range markList {
			if err := applyConditionsToMark(&markList[mi], tbl, ch, programs, attr); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyConditionsToMark evaluates each condition entry against the
// mark's datum row. The first entry that matches wins:
//   - test-form: evaluate expression; truthy → bake the entry's value
//     into the mark's Style; later entries skipped.
//   - selection-form: append a ConditionalAttr with the channel's
//     pre-resolved fallback as Otherwise; later entries skipped.
//
// Selection-driven entries can stack with one static entry that
// preceded them only when the static entry did not match — they live
// in the Conditions slice in declaration order. The first matching
// static entry short-circuits the whole list.
func applyConditionsToMark(m *scene.Mark, tbl *table.Table, ch conditionChannel, programs []*vm.Program, attr string) error {
	if m == nil {
		return nil
	}
	env := datumEnv(m, tbl)
	otherwise := currentStyleValue(m, attr)
	for i, entry := range ch.Cond.Entries() {
		switch {
		case entry.Test != "":
			prog := programs[i]
			if prog == nil {
				continue
			}
			out, err := expr.Run(prog, env)
			if err != nil {
				return prismerrors.New(
					"PRISM_ENCODE_001",
					fmt.Sprintf("Condition test on channel %s entry[%d] failed at runtime: %v.", ch.Name, i, err),
					map[string]any{"Channel": ch.Name, "Entry": i, "Expression": entry.Test, "Reason": err.Error()},
				)
			}
			if truthy(out) {
				if entry.Value != nil {
					if err := applyStyleAttr(m, attr, entry.Value); err != nil {
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

// datumEnv builds the per-row env map the expression evaluator sees.
// Datum.Fields is typically nil (D077 keeps the JSON payload small);
// we pull every column value at the row's index from the upstream
// table so test expressions can reference any field the spec used.
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

// applyStyleAttr writes value into m.Style at attr. Hex strings parse
// via scene.ColorFromHex; floats land on Opacity / StrokeWidth.
func applyStyleAttr(m *scene.Mark, attr string, value any) error {
	switch attr {
	case "fill":
		c, err := coerceColor(value)
		if err != nil {
			return err
		}
		m.Style.Fill = c
	case "stroke":
		c, err := coerceColor(value)
		if err != nil {
			return err
		}
		m.Style.Stroke = c
	case "opacity":
		f, err := coerceFloat(value)
		if err != nil {
			return err
		}
		m.Style.Opacity = f
	case "size":
		// size doesn't have a direct Style slot (mark-specific). Skip
		// gracefully — the static value would need per-geom plumbing.
		return nil
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

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case nil:
		return false
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	case string:
		return x != ""
	}
	return true
}
