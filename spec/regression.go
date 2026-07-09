package spec

// RegressionTransform fits an OLS regression over the source table and
// emits the two endpoints of the fitted trend line. The fit is pure-Go
// and in-memory (compile/inmem/regression.go): a single pass folds the
// OLS sufficient statistics, then the closed-form estimator yields the
// slope + intercept and the endpoints ŷ = β₀ + β·x at the predictor
// min/max.
//
// Constraint: must be the first transform in the chain (or the only
// transform). Like crosstab, the fit runs against the source table
// directly, so it cannot consume the output of a prior Prism transform.
// The plan builder enforces this via
// PRISM_PLAN_REGRESSION_REQUIRES_SOURCE; the validate rule signals it
// statically via PRISM_SPEC_035.
//
// The output table is two rows — (min(x), ŷ) and (max(x), ŷ) — over the
// {predictor, fitted} schema. Because every OLS fitted point is
// collinear, two endpoints draw the full line: layer a `line` mark over
// (predictor, fitted) on top of a `point` scatter of (predictor, target)
// for the classic regression-overlay chart.
type RegressionTransform struct {
	Regression RegressionBody `json:"regression"`
	Data       string         `json:"data,omitempty"`
	As         string         `json:"as,omitempty"`
}

// RegressionBody is the body of the regression transform.
type RegressionBody struct {
	// Target is the dependent variable (y). Required.
	Target string `json:"target"`
	// Predictors lists the independent variables (x). At least one is
	// required; multiple predictors fit a multiple regression.
	Predictors []string `json:"predictors"`
	// As is the fitted-value output column name. Defaults to "fitted".
	As string `json:"as,omitempty"`
}
