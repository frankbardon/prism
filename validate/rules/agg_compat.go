package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// AggCompat implements PRISM_SPEC_002: aggregate operations require the
// target field to be of a compatible measure type. Numeric aggregates
// (mean, sum, etc.) need quantitative or temporal fields; counting ops
// accept anything.
type AggCompat struct{}

// Code returns PRISM_SPEC_002.
func (AggCompat) Code() string { return "PRISM_SPEC_002" }

// Check inspects:
//   - encoding-channel inline aggregates (e.g. y: {field: x, aggregate: mean}),
//   - aggregate transform ops.
//
// As with PRISM_SPEC_001, the rule no-ops when the dataset schema is
// not resolvable (e.g. EmptyLookup or unbound inline values).
func (AggCompat) Check(s *spec.Spec, schemas validate.SchemaLookup) []*errors.AppError {
	dataset, schema, known := datasetForSpec(s, schemas)
	if !known {
		return nil
	}
	var out []*errors.AppError

	// 1. Channel-inline aggregates.
	for ch, ca := range channelAggregates(s.Encoding) {
		if ca.Field == "" || ca.Aggregate == "" {
			continue
		}
		field, ok := schema.Field(ca.Field)
		if !ok {
			// Unknown field is the job of PRISM_SPEC_001; don't double-fire here.
			continue
		}
		if compatibleAgg(ca.Aggregate, field.Type) {
			continue
		}
		out = append(out, errors.New("PRISM_SPEC_002",
			fmt.Sprintf("Aggregate op %q is not compatible with field %q (type %s, channel %s, dataset %s).",
				ca.Aggregate, ca.Field, field.Type, ch, dataset),
			map[string]any{
				"Op":         ca.Aggregate,
				"Field":      ca.Field,
				"FieldType":  field.Type,
				"Channel":    ch,
				"Dataset":    dataset,
				"Compatible": joinSorted(opsForType(field.Type)),
			},
		))
	}

	// 2. Aggregate-transform operations.
	for _, t := range s.Transform {
		if t.Aggregate == nil {
			continue
		}
		for _, op := range t.Aggregate.Aggregate {
			if op.Field == "" {
				continue
			}
			field, ok := schema.Field(op.Field)
			if !ok {
				continue
			}
			if compatibleAgg(op.Op, field.Type) {
				continue
			}
			out = append(out, errors.New("PRISM_SPEC_002",
				fmt.Sprintf("Aggregate op %q in transform is not compatible with field %q (type %s, dataset %s).",
					op.Op, op.Field, field.Type, dataset),
				map[string]any{
					"Op":         op.Op,
					"Field":      op.Field,
					"FieldType":  field.Type,
					"Channel":    "transform.aggregate",
					"Dataset":    dataset,
					"Compatible": joinSorted(opsForType(field.Type)),
				},
			))
		}
	}
	return out
}

// channelAggregate is the (field, aggregate) pair on a single channel.
type channelAggregate struct {
	Field     string
	Aggregate string
}

func channelAggregates(enc *spec.Encoding) map[string]channelAggregate {
	out := map[string]channelAggregate{}
	if enc == nil {
		return out
	}
	add := func(name, field, agg string) {
		out[name] = channelAggregate{Field: field, Aggregate: agg}
	}
	if enc.X != nil {
		add("x", enc.X.Field, enc.X.Aggregate)
	}
	if enc.Y != nil {
		add("y", enc.Y.Field, enc.Y.Aggregate)
	}
	if enc.X2 != nil {
		add("x2", enc.X2.Field, enc.X2.Aggregate)
	}
	if enc.Y2 != nil {
		add("y2", enc.Y2.Field, enc.Y2.Aggregate)
	}
	if enc.Theta != nil {
		add("theta", enc.Theta.Field, enc.Theta.Aggregate)
	}
	if enc.Radius != nil {
		add("radius", enc.Radius.Field, enc.Radius.Aggregate)
	}
	if enc.Color != nil {
		add("color", enc.Color.Field, enc.Color.Aggregate)
	}
	if enc.Fill != nil {
		add("fill", enc.Fill.Field, enc.Fill.Aggregate)
	}
	if enc.Stroke != nil {
		add("stroke", enc.Stroke.Field, enc.Stroke.Aggregate)
	}
	if enc.Opacity != nil {
		add("opacity", enc.Opacity.Field, enc.Opacity.Aggregate)
	}
	if enc.Size != nil {
		add("size", enc.Size.Field, enc.Size.Aggregate)
	}
	if enc.Shape != nil {
		add("shape", enc.Shape.Field, enc.Shape.Aggregate)
	}
	if enc.Text != nil {
		add("text", enc.Text.Field, enc.Text.Aggregate)
	}
	return out
}

// compatibleAgg reports whether op is compatible with a field of measureType.
func compatibleAgg(op, measureType string) bool {
	allowed := opsForType(measureType)
	for _, a := range allowed {
		if a == op {
			return true
		}
	}
	return false
}

// opsForType returns the aggregate ops considered compatible with a
// measure type. Counting / distinct / mode work on anything; numeric and
// quantile ops require quantitative or temporal.
func opsForType(measureType string) []string {
	universal := []string{"count", "distinct", "mode"}
	numeric := []string{
		"sum", "mean", "median", "min", "max", "stdev", "variance",
		"q1", "q3", "ci0", "ci1",
		"wmean", "ratio", "lift", "share",
	}
	switch strings.ToLower(measureType) {
	case "quantitative", "temporal":
		return append(append([]string{}, universal...), numeric...)
	default:
		// nominal, ordinal, or unknown — only counting-style ops.
		return universal
	}
}

func init() {
	// Keep opsForType output stable for tests / templated fixups.
	sort.Strings(opsForType("quantitative"))
}
