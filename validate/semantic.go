package validate

import (
	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
)

// SemanticRule is one Prism spec rule that needs Go-side reasoning
// (cross-field, Pulse-schema-aware, expression-aware, etc.). Rules return
// zero or more AppError values; a non-empty slice means the rule fired.
type SemanticRule interface {
	// Code returns the canonical PRISM_SPEC_* identifier this rule emits.
	Code() string
	// Check runs the rule against the typed spec. It receives a
	// SchemaLookup so rules that need dataset field metadata can resolve
	// against the registered sources (real Pulse-backed implementation
	// arrives in P02; P01 ships a static stub).
	Check(s *spec.Spec, schemas SchemaLookup) []*errors.AppError
}

// SemanticValidator runs an ordered set of SemanticRules against a spec.
// Construction is cheap; reuse across many specs.
type SemanticValidator struct {
	rules []SemanticRule
}

// NewSemanticValidator returns a SemanticValidator wired with the given
// rules in order. A nil/empty rules slice yields a validator that always
// returns zero errors.
func NewSemanticValidator(rules ...SemanticRule) *SemanticValidator {
	cp := make([]SemanticRule, len(rules))
	copy(cp, rules)
	return &SemanticValidator{rules: cp}
}

// Validate runs every rule against s using schemas. Errors from every
// rule are concatenated in rule order. Rules do not short-circuit each
// other; a failing rule does not stop subsequent rules.
func (v *SemanticValidator) Validate(s *spec.Spec, schemas SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	if schemas == nil {
		schemas = EmptyLookup{}
	}
	var out []*errors.AppError
	for _, r := range v.rules {
		out = append(out, r.Check(s, schemas)...)
	}
	return out
}

// Rules returns the rule list in order (defensive copy).
func (v *SemanticValidator) Rules() []SemanticRule {
	cp := make([]SemanticRule, len(v.rules))
	copy(cp, v.rules)
	return cp
}
