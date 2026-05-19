package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// RepeatSubstitution implements PRISM_SPEC_012 at validate time.
// When a `repeat` block's child spec carries an encoding channel
// with `{"field": {"repeat": <axis>}}` substitution that references
// an axis the parent did NOT declare on the repeat block, the rule
// fires before the builder runs.
//
// The build-time substitution walker (plan/build/composite.go) also
// catches this case — defense-in-depth, mirroring the layered scale
// rule (P08) — but firing at validate keeps the error visible
// through `prism validate` without driving the full plan pipeline.
type RepeatSubstitution struct{}

// Code returns PRISM_SPEC_012.
func (RepeatSubstitution) Code() string { return "PRISM_SPEC_012" }

// Check walks every `repeat` block in the spec (top-level + nested
// inside facet / repeat / layer / concat children) and asserts that
// each substitution's axis is declared on its nearest-enclosing
// repeat block.
func (RepeatSubstitution) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	walkRepeats(s, nil, &out)
	return out
}

// walkRepeats recurses through composition children. `bindings` is
// the active set of repeat axes declared by enclosing repeat blocks.
func walkRepeats(s *spec.Spec, bindings map[string]bool, out *[]*errors.AppError) {
	if s == nil {
		return
	}
	// Update bindings when this spec carries a repeat block.
	innerBindings := bindings
	if s.Repeat != nil {
		innerBindings = map[string]bool{}
		for k, v := range bindings {
			innerBindings[k] = v
		}
		if len(s.Repeat.Row) > 0 {
			innerBindings["row"] = true
		}
		if len(s.Repeat.Column) > 0 {
			innerBindings["column"] = true
		}
	}

	// Check this spec's encoding for substitutions referencing
	// undeclared axes.
	if s.Encoding != nil {
		checkEncoding(s.Encoding, innerBindings, out)
	}

	// Recurse into composition children using the (possibly extended)
	// bindings so nested specs see their ancestors' repeat axes.
	if s.ChildSpec != nil {
		walkRepeats(s.ChildSpec, innerBindings, out)
	}
	for _, c := range s.Layer {
		walkRepeats(c, innerBindings, out)
	}
	for _, c := range s.HConcat {
		walkRepeats(c, innerBindings, out)
	}
	for _, c := range s.VConcat {
		walkRepeats(c, innerBindings, out)
	}
	for _, c := range s.Concat {
		walkRepeats(c, innerBindings, out)
	}
}

func checkEncoding(enc *spec.Encoding, bindings map[string]bool, out *[]*errors.AppError) {
	positions := []*spec.PositionChannel{
		enc.X, enc.Y, enc.X2, enc.Y2, enc.Theta, enc.Radius,
	}
	for _, ch := range positions {
		if ch == nil || ch.FieldRef == nil {
			continue
		}
		if !bindings[ch.FieldRef.Axis] {
			*out = append(*out, buildRepeatSubstitutionError(ch.FieldRef, bindings))
		}
	}
	marks := []*spec.MarkChannel{
		enc.Color, enc.Fill, enc.Stroke, enc.Opacity, enc.Size, enc.Shape,
	}
	for _, ch := range marks {
		if ch == nil || ch.FieldRef == nil {
			continue
		}
		if !bindings[ch.FieldRef.Axis] {
			*out = append(*out, buildRepeatSubstitutionError(ch.FieldRef, bindings))
		}
	}
}

func buildRepeatSubstitutionError(ref *spec.RepeatRef, bindings map[string]bool) *errors.AppError {
	declared := make([]string, 0, len(bindings))
	for k := range bindings {
		declared = append(declared, k)
	}
	sort.Strings(declared)
	declaredStr := strings.Join(declared, ", ")
	if declaredStr == "" {
		declaredStr = "(none)"
	}
	return errors.New(
		"PRISM_SPEC_012",
		fmt.Sprintf("Repeat substitution {repeat: %q} references axis %q but the parent repeat block declares only %s.",
			ref.Axis, ref.Axis, declaredStr),
		map[string]any{
			"Ref":      fmt.Sprintf("{repeat: %q}", ref.Axis),
			"Axis":     ref.Axis,
			"Declared": declaredStr,
		},
	)
}
