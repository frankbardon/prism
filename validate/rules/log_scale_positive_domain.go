package rules

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// LogScalePositiveDomain implements PRISM_SPEC_010: a channel that
// declares scale.type=log must bind a field whose values (when
// available via inline data) are strictly positive. The rule walks
// every position + mark channel; for each that carries scale.type=log
// and a field name resolvable through inline data.values, it inspects
// the values and flags any non-positive entry.
//
// When the dataset is external (Pulse-backed, GCS, etc.) the rule
// no-ops — the encoder's per-row guard surfaces the same code at
// render time via scale.LogScale.Apply / encode.resolveLog.
type LogScalePositiveDomain struct{}

// Code returns PRISM_SPEC_010.
func (LogScalePositiveDomain) Code() string { return "PRISM_SPEC_010" }

// Check inspects every channel with scale.type=log. Returns
// PRISM_SPEC_010 for each field whose inline values include a zero
// or negative.
func (LogScalePositiveDomain) Check(s *spec.Spec, schemas validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Encoding == nil {
		return nil
	}
	rows := inlineRows(s)
	if rows == nil {
		// No inline values; defer to encoder-time guard.
		return nil
	}
	var out []*errors.AppError
	for ch, cs := range channelScales(s.Encoding) {
		if cs.ScaleType != "log" || cs.Field == "" {
			continue
		}
		for i, row := range rows {
			raw, ok := row[cs.Field]
			if !ok {
				continue
			}
			v, ok := scale.ToFloat(raw)
			if !ok {
				continue
			}
			if v <= 0 {
				out = append(out, errors.New("PRISM_SPEC_010",
					fmt.Sprintf("Log scale on channel %q binds field %q whose row %d has non-positive value %v.", ch, cs.Field, i, v),
					map[string]any{
						"Channel": ch,
						"Field":   cs.Field,
						"Row":     i,
						"Value":   v,
					},
				))
				break // one error per channel is enough
			}
		}
	}
	return out
}

// inlineRows returns the inline data.values for the spec, or nil if
// the spec has no inline data.
func inlineRows(s *spec.Spec) []map[string]any {
	if s == nil || s.Data == nil || len(s.Data.Values) == 0 {
		return nil
	}
	return s.Data.Values
}
