package rules

import (
	"github.com/frankbardon/prism/spec"
)

// spanBinding is one (mark, base channel, span channel) triple found
// somewhere in a spec tree. Path is a dotted slug naming the spec node
// the pair was found on ("" for the root, "layer[1]", "concat[0]", …)
// so an error can point at the right leaf.
type spanBinding struct {
	Path string
	Mark string
	// Name is the base channel's name — "x" or "y". The span channel
	// is always Name + "2".
	Name string
	Base *spec.PositionChannel
	Span *spec.PositionChannel
}

// SpanName returns the span channel's name ("x2" / "y2").
func (b spanBinding) SpanName() string { return b.Name + "2" }

// walkSpanBindings collects every bound span channel across the spec
// tree, including layer / concat / facet / repeat children. Only
// nodes that actually declare x2 or y2 produce an entry, so a spec
// without span channels yields nothing and neither span rule fires.
func walkSpanBindings(s *spec.Spec) []spanBinding {
	if s == nil {
		return nil
	}
	var out []spanBinding
	collect := func(prefix string, sub *spec.Spec) {
		out = append(out, spanBindingsAt(prefix, sub)...)
	}
	collect("", s)
	for i, l := range s.Layer {
		if l == nil {
			continue
		}
		collect(prefixf("layer[%d]", i), l)
	}
	for i, c := range s.Concat {
		if c == nil {
			continue
		}
		collect(prefixf("concat[%d]", i), c)
	}
	for i, c := range s.HConcat {
		if c == nil {
			continue
		}
		collect(prefixf("hconcat[%d]", i), c)
	}
	for i, c := range s.VConcat {
		if c == nil {
			continue
		}
		collect(prefixf("vconcat[%d]", i), c)
	}
	if s.ChildSpec != nil {
		collect("spec", s.ChildSpec)
	}
	return out
}

// spanBindingsAt returns the span bindings declared directly on s.
func spanBindingsAt(prefix string, s *spec.Spec) []spanBinding {
	if s == nil || s.Encoding == nil {
		return nil
	}
	mark := ""
	if s.Mark != nil {
		mark = s.Mark.TypeName()
	}
	var out []spanBinding
	pairs := []struct {
		name       string
		base, span *spec.PositionChannel
	}{
		{"x", s.Encoding.X, s.Encoding.X2},
		{"y", s.Encoding.Y, s.Encoding.Y2},
	}
	for _, p := range pairs {
		if p.span == nil {
			continue
		}
		out = append(out, spanBinding{
			Path: prefix,
			Mark: mark,
			Name: p.name,
			Base: p.base,
			Span: p.span,
		})
	}
	return out
}

// spanCapableMarks maps a mark type to the span channels it can
// actually draw. A mark absent from the map draws none.
//
// bar / rect range on either axis (the second axis keeps its band
// slot); rule turns into an interval segment on either or both; area
// takes y2 as an explicit lower edge but has no meaning for x2 — its
// x sequence is the path it traces, not an extent. Every other mark
// — line, point, tick, text, image, the composite and specialty
// families, and everything polar or geographic — has no ranged form,
// so a span channel there is an author error rather than a silent
// no-op (E9-S3).
var spanCapableMarks = map[string][]string{
	"bar":  {"x2", "y2"},
	"rect": {"x2", "y2"},
	"rule": {"x2", "y2"},
	"area": {"y2"},
}

// markDrawsSpan reports whether mark renders the named span channel.
func markDrawsSpan(mark, span string) bool {
	for _, c := range spanCapableMarks[mark] {
		if c == span {
			return true
		}
	}
	return false
}

// spanCapableMarkList returns the mark types that draw the named span
// channel, in a stable order for error text.
func spanCapableMarkList(span string) []string {
	var out []string
	for _, m := range []string{"area", "bar", "rect", "rule"} {
		if markDrawsSpan(m, span) {
			out = append(out, m)
		}
	}
	return out
}

// channelBound reports whether ch names a field — either directly or
// through an unsubstituted {"repeat": …} reference, which the plan
// builder rewrites into a field name per repeat cell.
func channelBound(ch *spec.PositionChannel) bool {
	if ch == nil {
		return false
	}
	return ch.Field != "" || ch.FieldRef != nil
}
