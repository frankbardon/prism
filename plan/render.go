package plan

import (
	"fmt"
	"reflect"
)

// Labeled is the optional capability a Node implements when it wants
// the renderers to print something richer than its Go type name. The
// renderers fall back to reflect-based detection when a node does not
// satisfy this interface.
type Labeled interface {
	// Kind returns a short type name like "FilterNode" or "JoinNode".
	Kind() string
	// Summary returns a one-line parameter summary used in DOT labels
	// (e.g. "expr: score > 50"). Empty is acceptable.
	Summary() string
}

// kindOf reports the runtime type name of n. If n implements Labeled,
// the user-supplied Kind() wins; otherwise we strip the package path
// and the leading * pointer indicator so labels say "FilterNode" not
// "*nodes.FilterNode".
func kindOf(n Node) string {
	if l, ok := n.(Labeled); ok {
		return l.Kind()
	}
	t := reflect.TypeOf(n)
	if t == nil {
		return "Node"
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

// summaryOf renders the one-line parameter summary for n. Uses the
// node's Labeled.Summary() when available; otherwise falls back to a
// type-name-only label. The per-node summary logic intentionally lives
// on each node itself (rather than a giant switch in this file) so
// adding a new node type does not require touching the renderer.
func summaryOf(n Node) string {
	if l, ok := n.(Labeled); ok {
		return l.Summary()
	}
	return ""
}

// shortID returns the last 8 characters of a NodeID. Useful for DOT
// labels because raw hashed ids are unreadable at full length.
func shortID(id NodeID) string {
	s := string(id)
	if len(s) <= 16 {
		return s
	}
	return s[len(s)-8:]
}

// renderLabel formats a multi-line node label for the DOT renderer.
// Lines: kind \n short-id \n summary (omitted if empty).
func renderLabel(n Node) string {
	kind := kindOf(n)
	short := shortID(n.ID())
	sum := summaryOf(n)
	if sum == "" {
		return fmt.Sprintf("%s\\n%s", kind, short)
	}
	return fmt.Sprintf("%s\\n%s\\n%s", kind, short, escapeDotLabel(sum))
}

// escapeDotLabel quotes characters that would break a DOT label.
// Newlines become `\n`; double quotes become `\"`; backslashes double.
func escapeDotLabel(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

// escapeDotID escapes a NodeID so it can appear as a quoted DOT
// identifier. Less aggressive than escapeDotLabel — the spec only
// requires escaping unbalanced quotes and backslashes.
func escapeDotID(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
