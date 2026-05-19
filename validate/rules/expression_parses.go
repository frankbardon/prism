package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ExpressionParses implements PRISM_SPEC_006: every Pulse expression
// (filter predicates, calculate expressions) must parse successfully.
//
// TODO(pulse): switch from expr-lang/expr to Pulse's wrapper as soon as
// Pulse exposes a public expression parser. Pulse 0.8.4 uses
// expr-lang/expr v1.17.8 internally (see processing/filterer.go) but
// does not export the compiler/options. Until then this rule depends
// directly on the same expr-lang/expr version so the syntax surface
// matches what Pulse runs at execution time.
type ExpressionParses struct{}

// Code returns PRISM_SPEC_006.
func (ExpressionParses) Code() string { return "PRISM_SPEC_006" }

var selectionInExpr = regexp.MustCompile(`(?i)selection\s*[:=]\s*[a-z_][a-z0-9_]*`)

// Check inspects every filter and calculate expression and tries to
// parse it. The selection:<name> shorthand used by SelectionRef is
// stripped before parsing so expressions that mix it with Pulse syntax
// still validate as expressions.
func (ExpressionParses) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	var out []*errors.AppError
	for i, t := range s.Transform {
		site := fmt.Sprintf("transform[%d]", i)
		switch {
		case t.Filter != nil:
			if err := tryParse(t.Filter.Filter); err != nil {
				out = append(out, expressionError(t.Filter.Filter, err, site+".filter"))
			}
		case t.Calculate != nil:
			if err := tryParse(t.Calculate.Calculate); err != nil {
				out = append(out, expressionError(t.Calculate.Calculate, err, site+".calculate"))
			}
		}
	}
	return out
}

func tryParse(src string) error {
	cleaned := strings.TrimSpace(selectionInExpr.ReplaceAllString(src, "true"))
	if cleaned == "" {
		return fmt.Errorf("empty expression")
	}
	_, err := expr.Compile(cleaned, expr.AllowUndefinedVariables())
	return err
}

func expressionError(srcExpr string, parseErr error, site string) *errors.AppError {
	return errors.New("PRISM_SPEC_006",
		fmt.Sprintf("Expression at %s failed to parse: %v.", site, parseErr),
		map[string]any{
			"Expression": srcExpr,
			"Site":       site,
			"Reason":     parseErr.Error(),
		},
	)
}
