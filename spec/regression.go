package spec

// RegressionTransform fits an OLS regression over the source cohort and
// augments every row with the fitted value ŷ. Wraps Pulse's
// ATTR_REG_FITTED attribute: a streaming prepass folds the OLS
// sufficient statistics, then a second pass emits ŷᵢ = Xᵢ·β + β₀ per
// filter-passing record.
//
// Constraint: must be the first transform in the chain (or the only
// transform). Like crosstab, the attribute fit runs against the source
// cohort directly — Pulse has no in-memory cohort constructor — so it
// cannot consume the output of a prior Prism transform. The plan
// builder enforces this via PRISM_PLAN_REGRESSION_REQUIRES_SOURCE; the
// validate rule signals it statically via PRISM_SPEC_035.
//
// The output table is the source rows + one F64 fitted column, sorted
// ascending by the first predictor so a `line` mark over (predictor,
// fitted) renders a clean trend line. Layer it over a `point` scatter
// of (predictor, target) for the classic regression-overlay chart.
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
