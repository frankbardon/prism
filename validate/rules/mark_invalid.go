package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// MarkInvalidSupported implements PRISM_SPEC_062: a declared
// `mark.invalid` must be a value this version understands, on a mark
// whose encoder actually honours it.
//
// `invalid` decides what happens to a row carrying a null in a
// scale-bound channel. "filter" — the default and the only pre-v0.16
// behaviour — drops the row, which also takes its category out of the
// scale domain. "break" keeps the row so the category holds its slot
// on the axis, draws no mark at it, and splits a path either side.
//
// Only the marks listed below implement the mask. Accepting "break" on
// a mark that ignores it would silently give the author "filter" while
// their spec says otherwise — precisely the silent-no-op class the
// inert-field work exists to end — so it is rejected with the list of
// marks that do support it.
type MarkInvalidSupported struct{}

// Code returns PRISM_SPEC_062.
func (MarkInvalidSupported) Code() string { return "PRISM_SPEC_062" }

// invalidAwareMarks are the mark types whose encoders consult
// marks.Inputs.Skip. Adding a mark here without wiring the mask into
// its encoder reintroduces the silent no-op this rule prevents.
//
// The set is the cartesian per-row and path marks, which are exactly
// the ones that can receive a null in a scale-bound channel and still
// have somewhere meaningful to put the gap. Marks that bring their own
// geometry — polar, histogram, the specialty and geo families — never
// hand a raw field value to a scale, so the null policy does not reach
// them at all and neither mode means anything there.
var invalidAwareMarks = map[string]bool{
	"line":  true,
	"area":  true,
	"point": true,
	"bar":   true,
	"rule":  true,
	"text":  true,
}

func invalidAwareMarkList() []string {
	out := make([]string, 0, len(invalidAwareMarks))
	for m := range invalidAwareMarks {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

type markInvalid struct {
	Path    string
	Mark    string
	Invalid string
}

// Check walks every mark in the spec tree that declares `invalid`.
func (MarkInvalidSupported) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, m := range walkMarkInvalids(s) {
		details := map[string]any{"Mark": m.Mark, "Invalid": m.Invalid, "Path": m.Path}
		switch {
		case !spec.MarkInvalidValid(m.Invalid):
			details["Allowed"] = strings.Join([]string{spec.MarkInvalidFilter, spec.MarkInvalidBreak}, ", ")
			out = append(out, errors.New("PRISM_SPEC_062",
				fmt.Sprintf("Mark property \"invalid\" has unknown value %q.", m.Invalid),
				details,
			))
		case m.Invalid == spec.MarkInvalidBreak && !invalidAwareMarks[m.Mark]:
			details["Allowed"] = strings.Join(invalidAwareMarkList(), ", ")
			out = append(out, errors.New("PRISM_SPEC_062",
				fmt.Sprintf("Mark type %q does not implement \"invalid\": %q, so it would be ignored.", m.Mark, spec.MarkInvalidBreak),
				details,
			))
		}
	}
	return out
}

// walkMarkInvalids collects every mark in the tree carrying an
// `invalid`, mirroring walkMarkOrients: one level per composition
// operator, because a mark def lives on a leaf and a composition
// parent has none of its own.
func walkMarkInvalids(s *spec.Spec) []markInvalid {
	if s == nil {
		return nil
	}
	var out []markInvalid
	collect := func(prefix string, sub *spec.Spec) {
		if sub == nil || sub.Mark == nil || sub.Mark.Def == nil || sub.Mark.Def.Invalid == "" {
			return
		}
		out = append(out, markInvalid{
			Path:    prefix,
			Mark:    sub.Mark.TypeName(),
			Invalid: sub.Mark.Def.Invalid,
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
