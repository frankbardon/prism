package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// RegressionPosition implements PRISM_SPEC_035:
//
//   - A regression transform must declare target + at least one
//     predictor.
//   - It may only appear as the first transform on a chain — like
//     crosstab, the OLS prepass fits the source cohort directly (Pulse
//     has no in-memory cohort constructor), so it cannot consume a prior
//     Prism transform's output. The plan builder enforces this at build
//     time via PRISM_PLAN_REGRESSION_REQUIRES_SOURCE; this rule surfaces
//     it statically before any I/O.
type RegressionPosition struct{}

// Code returns PRISM_SPEC_035.
func (RegressionPosition) Code() string { return "PRISM_SPEC_035" }

// Check walks every spec node and reports regression transforms that
// fail position or shape rules.
func (RegressionPosition) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	var out []*errors.AppError
	walkRegression(s, "", &out)
	return out
}

func walkRegression(s *spec.Spec, prefix string, out *[]*errors.AppError) {
	if s == nil {
		return
	}
	for i, t := range s.Transform {
		if t.Regression == nil {
			continue
		}
		path := fmt.Sprintf("%stransform[%d].regression", prefix, i)
		// Position: must be the first transform on the chain (or
		// reference a registered dataset via its `data` alias — that is
		// also leaf-bound).
		if i > 0 && t.Regression.Data == "" {
			*out = append(*out, errors.New(
				"PRISM_SPEC_035",
				fmt.Sprintf("regression at %s must be the first transform on the chain.", path),
				map[string]any{"Path": path, "Index": i},
			))
		}
		if t.Regression.Regression.Target == "" {
			*out = append(*out, errors.New(
				"PRISM_SPEC_035",
				fmt.Sprintf("regression.target at %s is required.", path),
				map[string]any{"Path": path},
			))
		}
		if len(t.Regression.Regression.Predictors) == 0 {
			*out = append(*out, errors.New(
				"PRISM_SPEC_035",
				fmt.Sprintf("regression.predictors at %s requires at least one field.", path),
				map[string]any{"Path": path},
			))
		}
	}
	for i, layer := range s.Layer {
		walkRegression(layer, fmt.Sprintf("%slayer[%d].", prefix, i), out)
	}
	for i, child := range s.Concat {
		walkRegression(child, fmt.Sprintf("%sconcat[%d].", prefix, i), out)
	}
	for i, child := range s.HConcat {
		walkRegression(child, fmt.Sprintf("%shconcat[%d].", prefix, i), out)
	}
	for i, child := range s.VConcat {
		walkRegression(child, fmt.Sprintf("%svconcat[%d].", prefix, i), out)
	}
	if s.ChildSpec != nil {
		walkRegression(s.ChildSpec, prefix+"spec.", out)
	}
}
