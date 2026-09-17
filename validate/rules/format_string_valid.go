package rules

import (
	"strconv"

	"github.com/frankbardon/prism/encode/format"
	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// FormatStringValid implements PRISM_SPEC_011: every channel.format,
// axis.format, legend.format and table columns[].format must parse as
// a valid d3-format specifier (subset supported in encode/format).
type FormatStringValid struct{}

// Code returns PRISM_SPEC_011.
func (FormatStringValid) Code() string { return "PRISM_SPEC_011" }

// Check walks every channel inspecting Format strings + nested axis /
// legend format strings + each table column's own format string.
func (FormatStringValid) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Encoding == nil {
		return nil
	}
	var out []*errors.AppError
	check := func(fmtStr, where string) {
		if fmtStr == "" {
			return
		}
		if _, err := format.Parse(fmtStr); err != nil {
			if ae, ok := err.(*errors.AppError); ok {
				ae.Context["Where"] = where
				out = append(out, ae)
			}
		}
	}
	enc := s.Encoding
	checkPosition := func(name string, ch *spec.PositionChannel) {
		if ch == nil {
			return
		}
		check(ch.Format, name+".format")
		if ch.Axis != nil {
			check(ch.Axis.Format, name+".axis.format")
		}
	}
	checkMark := func(name string, ch *spec.MarkChannel) {
		if ch == nil {
			return
		}
		check(ch.Format, name+".format")
		if ch.Legend != nil {
			check(ch.Legend.Format, name+".legend.format")
		}
	}
	checkPosition("x", enc.X)
	checkPosition("y", enc.Y)
	checkPosition("x2", enc.X2)
	checkPosition("y2", enc.Y2)
	checkMark("color", enc.Color)
	checkMark("fill", enc.Fill)
	checkMark("stroke", enc.Stroke)
	checkMark("opacity", enc.Opacity)
	checkMark("size", enc.Size)
	checkMark("shape", enc.Shape)
	if enc.Text != nil {
		check(enc.Text.Format, "text.format")
	}
	// Table columns (E7-S4). encoding.columns[] carries the same
	// ChannelCommon shape as every other channel, and encode/table.go
	// now applies the specifier through the same encode/format subset
	// this rule parses with — so an unparseable one has to be rejected
	// here rather than silently degrading to the raw value at render
	// time. Un-walked until E7-S4, which is how `$,.0f` (no currency
	// prefix in the subset) reached two committed fixtures.
	for i, col := range enc.Columns {
		check(col.Format, "columns["+strconv.Itoa(i)+"].format")
	}
	return out
}
