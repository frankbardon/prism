package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// RegressionStructure implements PRISM_SPEC_035 — the static shape check
// for a regression transform: it must declare a target and at least one
// predictor.
//
// It no longer enforces a chain-position constraint: regression accepts
// derived input (it may follow another transform), so the former
// "must be the first transform" clause was retired when regression
// gained derived-input support.
type RegressionStructure struct{}

// Code returns PRISM_SPEC_035.
func (RegressionStructure) Code() string { return "PRISM_SPEC_035" }

// Check walks every spec node and reports regression transforms that
// fail the shape rules.
func (RegressionStructure) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
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
		// Shape: target + at least one predictor required. Regression now
		// accepts derived input, so there is no chain-position check.
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
