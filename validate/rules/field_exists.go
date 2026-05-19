// Package rules holds the individual SemanticRule implementations that
// fire PRISM_SPEC_* error codes.
package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// FieldExists implements PRISM_SPEC_001: every encoded field must exist
// in the source dataset's schema (or be the output of a transform within
// the spec). With an EmptyLookup (no real Pulse source bound), the rule
// is a no-op — the resolver in P02 turns this into a hard check.
type FieldExists struct{}

// Code returns PRISM_SPEC_001.
func (FieldExists) Code() string { return "PRISM_SPEC_001" }

// Check inspects every encoding channel's "field" reference against the
// schema for the dataset bound by the leaf spec.
func (FieldExists) Check(s *spec.Spec, schemas validate.SchemaLookup) []*errors.AppError {
	dataset, schema, known := datasetForSpec(s, schemas)
	if !known {
		return nil
	}

	transformOutputs := collectTransformOutputs(s.Transform)

	var out []*errors.AppError
	for channel, field := range encodingFields(s.Encoding) {
		if field == "" {
			continue
		}
		if _, ok := schema.Field(field); ok {
			continue
		}
		if transformOutputs[field] {
			continue
		}
		available := joinSorted(append(schema.FieldNames(), keysOf(transformOutputs)...))
		out = append(out, errors.New(
			"PRISM_SPEC_001",
			fmt.Sprintf("Field %q not in source schema for dataset %q (channel %s).", field, dataset, channel),
			map[string]any{
				"Field":     field,
				"Dataset":   dataset,
				"Channel":   channel,
				"Available": available,
			},
		))
	}
	return out
}

// datasetForSpec resolves the dataset name for the leaf spec and looks it
// up in schemas. Returns (name, schema, known) where known is false if
// the spec is not bound to anything resolvable.
func datasetForSpec(s *spec.Spec, schemas validate.SchemaLookup) (string, *validate.PulseSchemaShim, bool) {
	if s == nil || s.Data == nil {
		return "", nil, false
	}
	name := ""
	switch {
	case s.Data.Name != "":
		name = s.Data.Name
	case s.Data.Source != "":
		name = s.Data.Source
	case len(s.Data.Values) > 0:
		// Inline values; only resolvable if the spec also names them.
		return "", nil, false
	}
	if name == "" {
		return "", nil, false
	}
	schema, ok := schemas.Schema(name)
	if !ok {
		return name, nil, false
	}
	return name, schema, true
}

// encodingFields returns channel-name → field-name for every channel that
// declares a literal field reference. Channels with only "value" (constant
// encoding) are skipped.
func encodingFields(enc *spec.Encoding) map[string]string {
	out := map[string]string{}
	if enc == nil {
		return out
	}
	add := func(ch string, f string) {
		if f != "" {
			out[ch] = f
		}
	}
	if enc.X != nil {
		add("x", enc.X.Field)
	}
	if enc.Y != nil {
		add("y", enc.Y.Field)
	}
	if enc.X2 != nil {
		add("x2", enc.X2.Field)
	}
	if enc.Y2 != nil {
		add("y2", enc.Y2.Field)
	}
	if enc.Theta != nil {
		add("theta", enc.Theta.Field)
	}
	if enc.Radius != nil {
		add("radius", enc.Radius.Field)
	}
	if enc.Color != nil {
		add("color", enc.Color.Field)
	}
	if enc.Fill != nil {
		add("fill", enc.Fill.Field)
	}
	if enc.Stroke != nil {
		add("stroke", enc.Stroke.Field)
	}
	if enc.Opacity != nil {
		add("opacity", enc.Opacity.Field)
	}
	if enc.Size != nil {
		add("size", enc.Size.Field)
	}
	if enc.Shape != nil {
		add("shape", enc.Shape.Field)
	}
	if enc.Text != nil {
		add("text", enc.Text.Field)
	}
	if enc.Row != nil {
		add("row", enc.Row.Field)
	}
	if enc.Column != nil {
		add("column", enc.Column.Field)
	}
	return out
}

// collectTransformOutputs returns the set of "as" names declared by any
// transform in the chain (these effectively introduce new field names).
func collectTransformOutputs(ts []spec.Transform) map[string]bool {
	out := map[string]bool{}
	for _, t := range ts {
		switch {
		case t.Calculate != nil && t.Calculate.As != "":
			out[t.Calculate.As] = true
		case t.Aggregate != nil:
			for _, agg := range t.Aggregate.Aggregate {
				if agg.As != "" {
					out[agg.As] = true
				}
			}
		case t.Bin != nil && t.Bin.As != "":
			out[t.Bin.As] = true
		case t.Window != nil:
			for _, w := range t.Window.Window {
				if w.As != "" {
					out[w.As] = true
				}
			}
		case t.Pivot != nil:
			// pivot adds columns we cannot enumerate without data; skip.
		case t.Unpivot != nil && len(t.Unpivot.As) == 2:
			out[t.Unpivot.As[0]] = true
			out[t.Unpivot.As[1]] = true
		}
	}
	return out
}

func joinSorted(items []string) string {
	seen := map[string]bool{}
	for _, s := range items {
		seen[s] = true
	}
	uniq := make([]string, 0, len(seen))
	for s := range seen {
		uniq = append(uniq, s)
	}
	sort.Strings(uniq)
	return strings.Join(uniq, ", ")
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
