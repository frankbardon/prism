package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// MarkOrientSupported implements PRISM_SPEC_046: a declared
// `mark.orient` must be a value some mark actually draws.
//
// Before E9-S1 the field was read by exactly one encoder (tree), so on
// every other mark it was silently discarded — an author asking for a
// horizontal bar got a vertical one with no diagnostic. E9-S1 makes
// `orient` real for the baseline-anchored cartesian marks and this
// rule closes the rest of the hole:
//
//   - `radial` is named by the vocabulary but implemented by no mark,
//     so it is rejected outright rather than quietly drawing a
//     cartesian mark.
//   - `orient` on a mark with no orientation geometry is rejected too,
//     because ignoring it is exactly the failure this rule exists to
//     end.
//
// Whether an *explicitly* requested orientation is drawable against
// the spec's actual scales is an encode-time question (the band has to
// be on the right axis), answered by PRISM_ENCODE_001 —
// `encode/marks/orient.go` MarkOrientation. This rule is purely about
// vocabulary.
type MarkOrientSupported struct{}

// Code returns PRISM_SPEC_046.
func (MarkOrientSupported) Code() string { return "PRISM_SPEC_046" }

// orientValues is the `mark.orient` vocabulary the schema admits.
var orientValues = map[string]bool{
	"vertical":   true,
	"horizontal": true,
	"radial":     true,
}

// orientAwareMarks maps a mark type to the orient values its encoder
// reads. It is the validate-side twin of the encoders themselves —
// `encode/marks/orient.go` (the cartesian families, where orient swaps
// the category and measure axes) and `encode/marks/tree.go` (tree /
// dendrogram / network, where it selects the direction the layout
// grows). A mark absent from this map draws no orientation at all, and
// a value absent from a mark's list is not implemented for it.
//
// The cartesian set grew in E9-S2 to cover every mark that grows from
// a baseline or sits in a band: area (and its sparkarea wrapper), tick,
// boxplot, violin, winloss and sparkbar, which is a thin wrapper over
// the bar encoder and so has honoured orient since E9-S1. `bullet` is
// absent on purpose — it keeps its own `orientation` field, a
// whole-mark rotation with the opposite default rather than an axis
// swap. `heatmap` is absent too: a heatmap is banded on both axes at
// once, so it has no category/measure split to swap.
//
// No entry lists "radial": nothing implements it. Keeping it out of
// every list — rather than out of the schema enum — is what lets this
// rule answer with a real message and fixups instead of an opaque
// enum miss.
var orientAwareMarks = map[string][]string{
	"bar":        {"vertical", "horizontal"},
	"rect":       {"vertical", "horizontal"},
	"area":       {"vertical", "horizontal"},
	"tick":       {"vertical", "horizontal"},
	"boxplot":    {"vertical", "horizontal"},
	"violin":     {"vertical", "horizontal"},
	"winloss":    {"vertical", "horizontal"},
	"sparkbar":   {"vertical", "horizontal"},
	"sparkarea":  {"vertical", "horizontal"},
	"progress":   {"vertical", "horizontal"},
	"tree":       {"vertical", "horizontal"},
	"dendrogram": {"vertical", "horizontal"},
	"network":    {"vertical", "horizontal"},
}

// Check walks every mark in the spec tree that declares `orient`.
func (MarkOrientSupported) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, m := range walkMarkOrients(s) {
		details := map[string]any{"Mark": m.Mark, "Orient": m.Orient, "Path": m.Path}
		switch {
		case !orientValues[m.Orient]:
			details["Allowed"] = "horizontal, vertical"
			out = append(out, errors.New("PRISM_SPEC_046",
				fmt.Sprintf("Mark property \"orient\" has unknown value %q.", m.Orient),
				details,
			))
		case len(orientAwareMarks[m.Mark]) == 0:
			details["Allowed"] = strings.Join(orientAwareMarkList(), ", ")
			out = append(out, errors.New("PRISM_SPEC_046",
				fmt.Sprintf("Mark type %q draws no orientation, so %q would be ignored.", m.Mark, "orient"),
				details,
			))
		case !markDrawsOrient(m.Mark, m.Orient):
			details["Allowed"] = strings.Join(orientAwareMarks[m.Mark], ", ")
			out = append(out, errors.New("PRISM_SPEC_046",
				fmt.Sprintf("Mark type %q does not implement orient %q.", m.Mark, m.Orient),
				details,
			))
		}
	}
	return out
}

// markOrient is one (mark type, orient value) pair found in a spec
// tree, tagged with the node it came from.
type markOrient struct {
	Path   string
	Mark   string
	Orient string
}

// walkMarkOrients collects every mark declaring a non-empty `orient`
// across the spec tree, including layer / concat / facet / repeat
// children. A spec with no `orient` anywhere yields nothing, so the
// rule never fires on existing specs.
func walkMarkOrients(s *spec.Spec) []markOrient {
	if s == nil {
		return nil
	}
	var out []markOrient
	collect := func(prefix string, sub *spec.Spec) {
		if sub == nil || sub.Mark == nil || sub.Mark.Def == nil || sub.Mark.Def.Orient == "" {
			return
		}
		out = append(out, markOrient{
			Path:   prefix,
			Mark:   sub.Mark.TypeName(),
			Orient: sub.Mark.Def.Orient,
		})
	}
	collect("", s)
	for i, l := range s.Layer {
		collect(prefixf("layer[%d]", i), l)
	}
	for i, c := range s.Concat {
		collect(prefixf("concat[%d]", i), c)
	}
	for i, c := range s.HConcat {
		collect(prefixf("hconcat[%d]", i), c)
	}
	for i, c := range s.VConcat {
		collect(prefixf("vconcat[%d]", i), c)
	}
	collect("spec", s.ChildSpec)
	return out
}

// markDrawsOrient reports whether mark implements the named orient.
func markDrawsOrient(mark, orient string) bool {
	for _, v := range orientAwareMarks[mark] {
		if v == orient {
			return true
		}
	}
	return false
}

// orientAwareMarkList returns every mark that reads `orient`, sorted
// for stable error text.
func orientAwareMarkList() []string {
	out := make([]string, 0, len(orientAwareMarks))
	for m := range orientAwareMarks {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
