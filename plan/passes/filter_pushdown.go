package passes

import (
	"strings"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
)

// FilterPushdownPass moves FilterNodes that sit immediately downstream
// of a JoinNode to whichever side of the join exclusively supplies the
// columns the filter references. Filters touching columns from both
// sides stay where they are.
//
// Identifier extraction (P07): we lex the expr string and pull every
// substring that matches `[A-Za-z_][A-Za-z0-9_]*` and isn't a reserved
// word. The set is over-approximate (numeric-suffixed literals, etc.)
// but conservative — the pass only pushes when EVERY identifier maps
// exclusively to one side, so spurious identifiers cause the pass to
// bail safely. When P14 ships a proper Pulse expression parser, swap
// `extractIdentifiers` for the typed AST walk; the rest of the pass is
// unchanged.
type FilterPushdownPass struct{}

// Name implements plan.Pass.
func (FilterPushdownPass) Name() string { return "filter_pushdown" }

// Apply implements plan.Pass.
func (FilterPushdownPass) Apply(d *plan.DAG) (*plan.DAG, bool, error) {
	if d == nil {
		return d, false, nil
	}
	out := d
	changed := false
	for _, id := range d.Nodes() {
		n, ok := out.Node(id)
		if !ok {
			continue
		}
		fn, ok := n.(*nodes.FilterNode)
		if !ok {
			continue
		}
		if len(fn.Inputs()) != 1 {
			continue
		}
		upID := fn.Inputs()[0]
		up, ok := out.Node(upID)
		if !ok {
			continue
		}
		jn, ok := up.(*nodes.JoinNode)
		if !ok {
			continue
		}
		cols := extractIdentifiers(fn.Expr())
		if len(cols) == 0 {
			continue
		}
		leftNode, lok := out.Node(jn.Inputs()[0])
		rightNode, rok := out.Node(jn.Inputs()[1])
		if !lok || !rok {
			continue
		}
		leftSchema, err := leftNode.Schema(nil)
		if err != nil {
			continue
		}
		rightSchema, err := rightNode.Schema(nil)
		if err != nil {
			continue
		}
		leftCols := schemaColSetFromEncoding(leftSchema)
		rightCols := schemaColSetFromEncoding(rightSchema)

		onlyLeft, onlyRight := true, true
		for _, c := range cols {
			_, inLeft := leftCols[c]
			_, inRight := rightCols[c]
			if !inLeft {
				onlyLeft = false
			}
			if !inRight {
				onlyRight = false
			}
		}
		switch {
		case onlyLeft && !onlyRight:
			out = pushFilterUnderJoin(out, fn, jn, "left")
			changed = true
		case onlyRight && !onlyLeft:
			out = pushFilterUnderJoin(out, fn, jn, "right")
			changed = true
		}
	}
	return out, changed, nil
}

// pushFilterUnderJoin rewires the DAG so the filter sits below the
// join on the named side. The original filter id is reused so any
// downstream reference still resolves; the join is reconstructed
// with the filter as its new input on that side.
func pushFilterUnderJoin(
	d *plan.DAG, fn *nodes.FilterNode, jn *nodes.JoinNode, side string,
) *plan.DAG {
	leftIn, rightIn := jn.Inputs()[0], jn.Inputs()[1]
	var newFilterInput, newJoinLeft, newJoinRight plan.NodeID
	switch side {
	case "left":
		newFilterInput = leftIn
		newJoinLeft = fn.ID()
		newJoinRight = rightIn
	case "right":
		newFilterInput = rightIn
		newJoinLeft = leftIn
		newJoinRight = fn.ID()
	default:
		return d
	}
	rebuiltFilter := nodes.NewFilter(fn.ID(), newFilterInput, fn.Expr())
	rebuiltJoin := nodes.NewJoin(jn.ID(), newJoinLeft, newJoinRight,
		jn.On(), jn.JoinKind(), 0)
	out := d.WithNode(rebuiltFilter)
	out = out.WithNode(rebuiltJoin)
	return out
}

// extractIdentifiers returns a deduplicated slice of identifier-like
// tokens from expr. Reserved words and numeric tokens are filtered
// out. Quoted string literals (single- or double-quoted) are skipped
// entirely so a filter like `label == 'alpha'` only extracts `label`.
// The function is intentionally tolerant: input that fails to parse
// still produces a (possibly empty) result.
func extractIdentifiers(expr string) []string {
	seen := map[string]struct{}{}
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		token := cur.String()
		cur.Reset()
		if !isIdentLike(token) || isReserved(token) {
			return
		}
		seen[token] = struct{}{}
	}
	inQuote := byte(0) // 0 = not in a quoted literal; otherwise the quote char.
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		if inQuote != 0 {
			if c == inQuote {
				inQuote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			flush()
			inQuote = c
			continue
		}
		switch {
		case c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9'):
			cur.WriteByte(c)
		default:
			flush()
		}
	}
	flush()
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

func isIdentLike(s string) bool {
	if s == "" {
		return false
	}
	r := s[0]
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isReserved(s string) bool {
	switch s {
	case "true", "false", "and", "or", "not", "in", "nil", "null":
		return true
	}
	return false
}
