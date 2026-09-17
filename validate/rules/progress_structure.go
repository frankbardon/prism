package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ProgressStructure implements PRISM_SPEC_061: the shape invariants of
// a `progress` mark.
//
// A progress mark draws one metric row per table row — a value bar on a
// full-scale track — so it needs two position channels, one discrete
// (the metric labels) and one quantitative (the value). The two
// mark-def knobs it adds carry numeric domains that are meaningless
// outside their range, and both fail silently rather than loudly at
// encode time, which is exactly the class of hole this rule closes:
//
//   - `thickness` is a fraction of the category band, so it must be
//     greater than 0 and at most 1. A 0 draws nothing and a 2 draws a
//     row overlapping its neighbours.
//   - `total` is the measure ceiling the track runs to. As a literal it
//     must be a positive number; as a string it names a data field
//     (checked for existence by PRISM_SPEC_001, not here).
//
// Structural only: it never inspects data or scales. Whether an
// explicitly requested orientation is drawable against the spec's
// actual scales stays an encode-time question (PRISM_ENCODE_001, via
// marks.MarkOrientation), and whether `orient` is vocabulary a mark
// draws at all stays PRISM_SPEC_046's.
type ProgressStructure struct{}

// Code returns PRISM_SPEC_061.
func (ProgressStructure) Code() string { return "PRISM_SPEC_061" }

// Check walks every progress mark in the spec tree.
func (ProgressStructure) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, n := range walkProgressMarks(s) {
		out = append(out, checkProgressNode(n.Path, n.Spec)...)
	}
	return out
}

// progressNode is one progress-marked spec node, tagged with the path
// it was found at for error context.
type progressNode struct {
	Path string
	Spec *spec.Spec
}

// walkProgressMarks collects every spec node whose mark is `progress`,
// including layer / concat / facet / repeat children.
func walkProgressMarks(s *spec.Spec) []progressNode {
	if s == nil {
		return nil
	}
	var out []progressNode
	collect := func(prefix string, sub *spec.Spec) {
		if sub == nil || sub.Mark == nil || sub.Mark.TypeName() != "progress" {
			return
		}
		out = append(out, progressNode{Path: prefix, Spec: sub})
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

// checkProgressNode applies the three structural checks to one node.
func checkProgressNode(path string, s *spec.Spec) []*errors.AppError {
	var out []*errors.AppError
	details := func(extra map[string]any) map[string]any {
		d := map[string]any{"Mark": "progress", "Path": path}
		for k, v := range extra {
			d[k] = v
		}
		return d
	}

	enc := s.Encoding
	xBound := enc != nil && enc.X != nil && enc.X.Field != ""
	yBound := enc != nil && enc.Y != nil && enc.Y.Field != ""
	if !xBound || !yBound {
		missing := "x"
		if xBound {
			missing = "y"
		}
		out = append(out, errors.New("PRISM_SPEC_061",
			fmt.Sprintf("A progress mark needs both x and y bound; %q has no field.", missing),
			details(map[string]any{"Channel": missing}),
		))
	}

	def := s.Mark.Def
	if def == nil {
		return out
	}
	if def.Thickness != nil {
		if t := *def.Thickness; t <= 0 || t > 1 {
			out = append(out, errors.New("PRISM_SPEC_061",
				fmt.Sprintf("A progress mark's thickness is a fraction of the category band; %v is outside (0, 1].", t),
				details(map[string]any{"Property": "thickness", "Value": t}),
			))
		}
	}
	if n, isNumber := progressTotalNumber(def.Total); isNumber && n <= 0 {
		out = append(out, errors.New("PRISM_SPEC_061",
			fmt.Sprintf("A progress mark's total is the measure ceiling its track runs to; %v is not positive.", n),
			details(map[string]any{"Property": "total", "Value": n}),
		))
	}
	return out
}

// progressTotalNumber reports whether raw is a numeric literal (as
// opposed to a field name or absent) and, if so, its value. JSON
// decoding lands numbers as float64; the integer cases cover a spec
// built in Go rather than decoded.
func progressTotalNumber(raw any) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}
