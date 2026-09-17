package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// TransformExecutable implements PRISM_SPEC_067: a transform the spec
// grammar accepts but no backend can execute is rejected at validate
// rather than left to fail at execute time.
//
// spec/transform_union.go accepts sixteen discriminator keys. A spec
// naming one of them decodes, passes shape validation, builds a plan
// node — and then, for the ones nobody implemented, dies inside
// plan.Execute with PRISM_COMPILE_001 naming an internal node kind
// ("PivotNode") the author never wrote. That made "validates" and
// "draws" independent claims, which is the thing this rule exists to
// close: validate must not call a spec valid when the engine cannot
// run it.
//
// This rule moves an existing failure earlier. It creates none — every
// spec it rejects already fails at execute today.
//
// # Why the list is what it is, and how it is kept honest
//
// validate/ may not import plan/ or compile/ (the no-execute structural
// ban, CLAUDE.md "What NOT to Do"), so the capability cannot be read
// off the backend at runtime and has to be stated here. A stated list
// rots: the story that asked for this rule named join, union and pivot
// as the un-executable set, and two of those three were already wrong
// when it was written.
//
//   - `join` and `union` never migrated to the in-memory backend, but
//     they never needed to — JoinNode and UnionNode carry their own
//     Execute bodies (plan/nodes/join_execute.go, union_execute.go) and
//     plan.Execute calls node.Execute. They run.
//   - `unpivot` gained both halves — the compile/inmem executor AND the
//     SetBackend wiring a backend-routed node needs — shortly before
//     this rule landed.
//
// So the list is verified rather than asserted:
// internal/gates/transform_executable_sync_test.go drives one minimal
// spec per transform variant through the real planner and the real
// in-memory backend and requires this rule's verdict to match what
// actually happens. It enumerates the variants by reflecting over
// spec.Transform, so a seventeenth transform cannot slip past it
// unclassified, and it fails just as loudly when a transform named
// here starts working as when one omitted here stops.
type TransformExecutable struct{}

// transformExecutors records, per transform discriminator key, whether
// executing a spec that names it produces a table.
//
// Executable means both halves are present: an implementation (either a
// compile/inmem executor reached through compile/inmem.Backend.Compile,
// or an Execute body on the plan node itself) AND, for the
// backend-routed kind, the SetBackend wiring plan/build installs. An
// executor with no wiring is dead code and the transform still fails —
// which is why the gate probes end to end instead of reading either
// half on its own.
//
// Every key in spec's transform discriminator set must appear here.
var transformExecutors = map[string]bool{
	"filter":     true, // compile/inmem/filter.go
	"calculate":  true, // compile/inmem/calculate.go
	"aggregate":  true, // compile/inmem/group_aggregate.go
	"bin":        true, // compile/inmem/bin.go
	"window":     true, // compile/inmem/window.go
	"join":       true, // plan/nodes/join_execute.go (node-level, never migrated)
	"union":      true, // plan/nodes/union_execute.go (node-level, never migrated)
	"pivot":      false,
	"unpivot":    true, // compile/inmem/unpivot.go + UnpivotNode.SetBackend
	"sample":     true, // compile/inmem/sample.go
	"sort":       true, // compile/inmem/sort.go
	"limit":      true, // compile/inmem/limit.go
	"crosstab":   true, // compile/inmem/crosstab.go
	"regression": true, // compile/inmem/regression.go
	"timeunit":   true, // compile/inmem/timeunit.go
	"stack":      true, // compile/inmem/stack.go
}

// Code returns PRISM_SPEC_067.
func (TransformExecutable) Code() string { return "PRISM_SPEC_067" }

// Check walks every transform in the spec tree, including layer /
// concat / hconcat / vconcat / facet / repeat children, and reports one
// error per transform whose backend has no dispatch case.
func (TransformExecutable) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	var out []*errors.AppError
	walkTransformsForExecutor(s, "", &out)
	return out
}

// walkTransformsForExecutor recurses the composition tree. prefix is a
// dotted path slug ("", "layer[1].", "concat[0].layer[2].") so the
// error points at the transform the author wrote.
func walkTransformsForExecutor(s *spec.Spec, prefix string, out *[]*errors.AppError) {
	if s == nil {
		return
	}
	for i, t := range s.Transform {
		name := transformVariantName(t)
		if name == "" {
			// No variant set, or a variant this rule has not been
			// taught. Reporting nothing is the safe half of the
			// trade — a false positive on a working transform is the
			// mirror image of the bug being fixed — and the gate in
			// internal/gates is what stops an unknown variant from
			// staying unknown.
			continue
		}
		if transformExecutors[name] {
			continue
		}
		*out = append(*out, errors.New(
			"PRISM_SPEC_067",
			fmt.Sprintf("Transform %q at %stransform[%d] is accepted by the spec grammar but no backend can execute it.", name, prefix, i),
			map[string]any{
				"Transform":  name,
				"Path":       fmt.Sprintf("%stransform[%d]", prefix, i),
				"Index":      i,
				"Executable": strings.Join(executableTransformList(), ", "),
			},
		))
	}
	for i, layer := range s.Layer {
		walkTransformsForExecutor(layer, fmt.Sprintf("%slayer[%d].", prefix, i), out)
	}
	for i, child := range s.Concat {
		walkTransformsForExecutor(child, fmt.Sprintf("%sconcat[%d].", prefix, i), out)
	}
	for i, child := range s.HConcat {
		walkTransformsForExecutor(child, fmt.Sprintf("%shconcat[%d].", prefix, i), out)
	}
	for i, child := range s.VConcat {
		walkTransformsForExecutor(child, fmt.Sprintf("%svconcat[%d].", prefix, i), out)
	}
	walkTransformsForExecutor(s.ChildSpec, prefix+"spec.", out)
}

// transformVariantName returns the discriminator key of whichever
// variant is populated on t, or "" when none is.
//
// This mirrors spec.Transform's own union switch. It is restated here
// rather than read from spec/ because the discriminator table is
// unexported; the gate's reflection over spec.Transform is what keeps
// the two in step.
func transformVariantName(t spec.Transform) string {
	switch {
	case t.Filter != nil:
		return "filter"
	case t.Calculate != nil:
		return "calculate"
	case t.Aggregate != nil:
		return "aggregate"
	case t.Bin != nil:
		return "bin"
	case t.Window != nil:
		return "window"
	case t.Join != nil:
		return "join"
	case t.Union != nil:
		return "union"
	case t.Pivot != nil:
		return "pivot"
	case t.Unpivot != nil:
		return "unpivot"
	case t.Sample != nil:
		return "sample"
	case t.Sort != nil:
		return "sort"
	case t.Limit != nil:
		return "limit"
	case t.Crosstab != nil:
		return "crosstab"
	case t.Regression != nil:
		return "regression"
	case t.TimeUnit != nil:
		return "timeunit"
	case t.Stack != nil:
		return "stack"
	}
	return ""
}

// executableTransformList returns the transforms that do execute, in a
// stable order, for the error's fixup text.
func executableTransformList() []string {
	out := make([]string, 0, len(transformExecutors))
	for name, ok := range transformExecutors {
		if ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
