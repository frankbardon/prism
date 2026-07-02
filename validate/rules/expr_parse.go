package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
)

// This helper preserves the expr-lang parse shim used by the condition
// `test` rule (PRISM_SPEC_026). The transform-level calculate-string
// rule (formerly PRISM_SPEC_006) was retired in E2-S2 once calculate
// became a structured expression; conditions still carry a `test`
// string, so the parser shim lives here until E2-S3 removes expr-lang
// from the condition path too.
//
// TODO(E2-S3): drop expr-lang from conditions and this helper with it.

var selectionInExpr = regexp.MustCompile(`(?i)selection\s*[:=]\s*[a-z_][a-z0-9_]*`)

// tryParse compiles a condition test expression, stripping the
// selection:<name> shorthand first so expressions that mix it with
// Pulse syntax still validate.
func tryParse(src string) error {
	cleaned := strings.TrimSpace(selectionInExpr.ReplaceAllString(src, "true"))
	if cleaned == "" {
		return fmt.Errorf("empty expression")
	}
	_, err := expr.Compile(cleaned, expr.AllowUndefinedVariables())
	return err
}
