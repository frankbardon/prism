package rules

import (
	"github.com/frankbardon/prism/encode/format"
	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// FormatStringValid implements PRISM_SPEC_011: every channel.format,
// axis.format, and legend.format must parse as a valid d3-format
// specifier (subset supported in encode/format).
type FormatStringValid struct{}

// Code returns PRISM_SPEC_011.
func (FormatStringValid) Code() string { return "PRISM_SPEC_011" }

// Check walks every channel inspecting Format strings + nested axis /
// legend format strings.
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
	return out
}
