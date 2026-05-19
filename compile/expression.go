// Package compile holds the cross-backend types used by Prism's
// compile stage: the Backend interface alias, the friendly aggregate
// alias map, and the expression-passthrough shim that wraps
// expr-lang/expr exactly the way Pulse's processing/filterer.go does.
//
// The in-memory implementation lives at compile/inmem; future Pulse /
// Arrow / DuckDB backends drop into the same Backend interface
// without touching plan/.
package compile

import (
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
)

// Backend is re-exported from plan/ so consumers can address it via
// `compile.Backend` (the natural import for compile-stage callers).
// See plan/backend.go for the canonical definition.
type Backend = plan.Backend

// ExprProgram wraps a compiled expr-lang program plus the original
// source for diagnostics. Use CompileExpression to build one.
type ExprProgram struct {
	program *vm.Program
	source  string
}

// Source returns the original expression string. Useful for error
// context where the program was compiled in one place and evaluated
// in another.
func (p *ExprProgram) Source() string { return p.source }

// CompileExpression parses src via expr-lang/expr (the same parser
// Pulse uses internally; see D022) and returns a runnable program.
// Compile errors wrap as PRISM_COMPILE_002 so the executor surfaces
// them consistently with runtime errors.
func CompileExpression(src string) (*ExprProgram, error) {
	prog, err := expr.Compile(src, expr.AllowUndefinedVariables())
	if err != nil {
		return nil, prismerrors.New(
			"PRISM_COMPILE_002",
			fmt.Sprintf("Expression failed to compile: %s.", err.Error()),
			map[string]any{
				"Expression": src,
				"Reason":     err.Error(),
				"Site":       "compile",
			},
		)
	}
	return &ExprProgram{program: prog, source: src}, nil
}

// Eval evaluates the program against env and returns the raw result.
// Runtime errors wrap as PRISM_COMPILE_002 with the env's row-index
// context (when supplied by the caller as env["__row__"]).
func (p *ExprProgram) Eval(env map[string]any) (any, error) {
	out, err := expr.Run(p.program, env)
	if err != nil {
		return nil, prismerrors.New(
			"PRISM_COMPILE_002",
			fmt.Sprintf("Expression failed at runtime: %s.", err.Error()),
			map[string]any{
				"Expression": p.source,
				"Reason":     err.Error(),
				"Site":       fmt.Sprintf("row=%v", env["__row__"]),
			},
		)
	}
	return out, nil
}

// EvalBool evaluates the program and coerces the result to bool.
// Used by Filter. Non-bool results error out as PRISM_COMPILE_002.
func (p *ExprProgram) EvalBool(env map[string]any) (bool, error) {
	out, err := p.Eval(env)
	if err != nil {
		return false, err
	}
	b, ok := out.(bool)
	if !ok {
		return false, prismerrors.New(
			"PRISM_COMPILE_002",
			fmt.Sprintf("Expression must evaluate to a boolean; got %T.", out),
			map[string]any{
				"Expression": p.source,
				"Reason":     fmt.Sprintf("non-bool result: %T", out),
				"Site":       fmt.Sprintf("row=%v", env["__row__"]),
			},
		)
	}
	return b, nil
}

// EvalFloat evaluates the program and coerces the result to float64.
// Used by Calculate. int / int64 / float32 are promoted; everything
// else errors out as PRISM_COMPILE_002.
func (p *ExprProgram) EvalFloat(env map[string]any) (float64, error) {
	out, err := p.Eval(env)
	if err != nil {
		return 0, err
	}
	switch v := out.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	}
	return 0, prismerrors.New(
		"PRISM_COMPILE_002",
		fmt.Sprintf("Expression must evaluate to a number; got %T.", out),
		map[string]any{
			"Expression": p.source,
			"Reason":     fmt.Sprintf("non-numeric result: %T", out),
			"Site":       fmt.Sprintf("row=%v", env["__row__"]),
		},
	)
}
