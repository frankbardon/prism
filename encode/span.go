package encode

import (
	"github.com/frankbardon/prism/encode/marks"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// Span-channel plumbing (E9-S3).
//
// `x2` / `y2` never resolve a scale of their own. They ride the scale
// their base channel resolved, which is what makes a span mean the
// same thing as the position it extends from and keeps both ends on
// one axis. Two helpers carry that contract:
//
//   - spanDomainValues widens the base channel's domain with the
//     companion column's values *before* the scale is built, so an
//     interval reaching past the base column's own range is not
//     clipped.
//   - spanChannel hands the mark encoders the companion field name
//     paired with the base channel's resolved scale.
//
// Both return the zero value when the span channel is absent, so a
// spec without x2 / y2 produces exactly the same scale and the same
// marks.Inputs it did before.

// spanDomainValues returns every value in ch's column so it can be
// folded into the base channel's domain. Returns nil when ch is
// unbound or its column is missing — a missing column surfaces later
// as PRISM_ENCODE_001 from the mark encoder, with the field name in
// the details.
func spanDomainValues(ch *spec.PositionChannel, tbl *table.Table) []any {
	if ch == nil || ch.Field == "" || tbl == nil {
		return nil
	}
	col, ok := tbl.Column(ch.Field)
	if !ok {
		return nil
	}
	out := make([]any, col.Len())
	for i := 0; i < col.Len(); i++ {
		out[i] = col.ValueAt(i)
	}
	return out
}

// spanChannel binds ch to the base channel's already-resolved scale.
// A nil scale (polar / specialty / geo marks resolve none) yields the
// zero Channel, which every mark encoder reads as "no span bound".
func spanChannel(ch *spec.PositionChannel, base marks.Scale) marks.Channel {
	if ch == nil || ch.Field == "" || base == nil {
		return marks.Channel{}
	}
	return marks.Channel{Field: ch.Field, Scale: base}
}
