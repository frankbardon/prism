package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/format"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// BuildTooltips materialises one *scene.Tooltip per row from a
// TooltipChannel binding. Returns nil when ch is nil (caller leaves
// Mark.Tooltip nil — per-mark renderers skip the <title> child).
//
// Each TooltipLine is formatted as "<field>: <formatted_value>" where
// the formatter is the channel's `format` specifier through
// encode/format (matching axis-label formatting), falling back to
// fmt.Sprintf("%v", value) when no format is set.
//
// Single tooltip: 1 line per row.
// Multi tooltip:  N lines per row (one per Multi entry).
//
// Missing fields render as "<field>: <missing>" — tooltips are
// diagnostic, never blocking. See D063.
func BuildTooltips(tbl *table.Table, ch *spec.TooltipChannel, rowCount int) []*scene.Tooltip {
	if ch == nil || rowCount == 0 {
		return nil
	}
	var entries []spec.TextChannel
	switch {
	case ch.Multi != nil:
		entries = ch.Multi
	case ch.Single != nil:
		entries = []spec.TextChannel{*ch.Single}
	default:
		return nil
	}
	if len(entries) == 0 {
		return nil
	}
	out := make([]*scene.Tooltip, rowCount)
	for i := 0; i < rowCount; i++ {
		lines := make([]scene.TooltipLine, 0, len(entries))
		for _, e := range entries {
			lines = append(lines, scene.TooltipLine{
				Label: tooltipLineLabel(tbl, e, i),
			})
		}
		out[i] = &scene.Tooltip{Lines: lines}
	}
	return out
}

// tooltipLineLabel renders one "<field>: <value>" entry for row i.
// Honors `format` when set (numeric or time formatter via
// encode/format). On missing field, returns "<field>: <missing>".
func tooltipLineLabel(tbl *table.Table, e spec.TextChannel, row int) string {
	field := e.Field
	if field == "" {
		// Value-only tooltip (rare): use Value as the literal.
		if e.Value != nil {
			return fmt.Sprintf("%v", e.Value)
		}
		return ""
	}
	col, ok := tbl.Column(field)
	if !ok {
		return fmt.Sprintf("%s: <missing>", field)
	}
	if row >= col.Len() {
		return fmt.Sprintf("%s: <missing>", field)
	}
	value := col.ValueAt(row)
	var formatted string
	if e.Format != "" {
		spec, err := format.Parse(e.Format)
		if err == nil {
			formatted = spec.Apply(value)
		} else {
			formatted = fmt.Sprintf("%v", value)
		}
	} else {
		formatted = fmt.Sprintf("%v", value)
	}
	return fmt.Sprintf("%s: %s", field, formatted)
}

// AttachTooltips attaches tooltips[i] to marks[i].Tooltip in 1:1
// order. When len(tooltips) != len(marks), attaches as far as the
// shorter slice allows (single-mark types like line/area receive
// tooltips[0]; per-row types receive 1:1). Mutates marks in place.
func AttachTooltips(marks []scene.Mark, tooltips []*scene.Tooltip) {
	if len(tooltips) == 0 || len(marks) == 0 {
		return
	}
	if len(marks) == 1 && len(tooltips) > 1 {
		// Single-mark form (line / area): attach the first row's
		// tooltip; per-series hover lands in P12.
		marks[0].Tooltip = tooltips[0]
		return
	}
	n := len(marks)
	if len(tooltips) < n {
		n = len(tooltips)
	}
	for i := 0; i < n; i++ {
		marks[i].Tooltip = tooltips[i]
	}
}
