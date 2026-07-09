package inmem

import (
	"context"
	"math"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// executeRegression fits a single-predictor ordinary-least-squares
// regression over the materialised input table, pure-Go with no engine
// coupling. It reproduces the shape the former Pulse-backed REG_OLS leaf
// emitted: the two endpoints of the fitted trend line —
// (xmin, ŷ(xmin)) and (xmax, ŷ(xmax)) — so a `line` mark over
// (predictor, fitted) draws the regression line. For OLS every fitted
// point is collinear, so two endpoints suffice.
//
// The fit uses the closed-form estimator over the records where BOTH the
// target and the predictor are non-null (matching Pulse's REG_OLS
// pairwise-complete contract). The x-domain endpoints are the min/max of
// the non-null predictor values (matching the AGG_MIN/AGG_MAX the Pulse
// path carried alongside the fit). Coefficients are computed from the
// centred sufficient statistics for numerical stability; the result
// matches Pulse's REG_OLS output within floating-point tolerance.
func executeRegression(_ context.Context, n *nodes.RegressionNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}
	predictor := n.Predictor()
	target := n.Body().Target

	xCol, ok := in.Column(predictor)
	if !ok {
		return nil, prismerrors.New(
			"PRISM_PLAN_REGRESSION_PROCESS",
			"regression predictor "+predictor+" not found in the input table.",
			map[string]any{"Ref": n.Ref(), "Reason": "predictor column missing"},
		)
	}
	yCol, ok := in.Column(target)
	if !ok {
		return nil, prismerrors.New(
			"PRISM_PLAN_REGRESSION_PROCESS",
			"regression target "+target+" not found in the input table.",
			map[string]any{"Ref": n.Ref(), "Reason": "target column missing"},
		)
	}

	rows := in.NumRows()

	// Pairwise-complete sums for the OLS fit, plus the predictor min/max
	// over all non-null predictor values for the x-domain endpoints.
	var (
		count        int
		sumX, sumY   float64
		sumXX, sumXY float64
		haveXExtent  bool
		xMin, xMax   float64
	)
	for i := 0; i < rows; i++ {
		if !xCol.IsNull(i) {
			if x, ok := toFloat64(xCol.ValueAt(i)); ok {
				if !haveXExtent {
					xMin, xMax = x, x
					haveXExtent = true
				} else {
					if x < xMin {
						xMin = x
					}
					if x > xMax {
						xMax = x
					}
				}
			}
		}
		if xCol.IsNull(i) || yCol.IsNull(i) {
			continue
		}
		x, xok := toFloat64(xCol.ValueAt(i))
		y, yok := toFloat64(yCol.ValueAt(i))
		if !xok || !yok {
			continue
		}
		count++
		sumX += x
		sumY += y
		sumXX += x * x
		sumXY += x * y
	}

	if count < 2 || !haveXExtent {
		return nil, prismerrors.New(
			"PRISM_PLAN_REGRESSION_PROCESS",
			"regression needs at least two complete (predictor, target) records to fit a trend line.",
			map[string]any{"Ref": n.Ref(), "Reason": "insufficient complete records"},
		)
	}

	nF := float64(count)
	meanX := sumX / nF
	meanY := sumY / nF
	// Centred cross-products: Sxx = Σ(x-x̄)², Sxy = Σ(x-x̄)(y-ȳ).
	sxx := sumXX - nF*meanX*meanX
	sxy := sumXY - nF*meanX*meanY
	if sxx == 0 || math.IsNaN(sxx) {
		return nil, prismerrors.New(
			"PRISM_PLAN_REGRESSION_PROCESS",
			"regression predictor "+predictor+" has zero variance; the OLS slope is undefined.",
			map[string]any{"Ref": n.Ref(), "Reason": "degenerate predictor variance"},
		)
	}
	slope := sxy / sxx
	intercept := meanY - slope*meanX

	fittedAs := n.FittedAs()
	endpoints := []map[string]float64{
		{predictor: xMin, fittedAs: intercept + slope*xMin},
		{predictor: xMax, fittedAs: intercept + slope*xMax},
	}

	hash := hashChain(in.Hash(), n.Fingerprint())
	outSchema, err := n.Schema([]*table.Schema{in.Schema()})
	if err != nil {
		return nil, err
	}
	return materializeRegression(endpoints, outSchema, hash)
}

// materializeRegression drains the fitted endpoints into a *table.Table
// with the pre-derived {predictor, fitted} output schema. Both columns
// are F64 by construction.
func materializeRegression(rows []map[string]float64, schema *table.Schema, hash string) (*table.Table, error) {
	if schema == nil {
		return nil, prismerrors.New(
			"PRISM_PLAN_REGRESSION_PROCESS",
			"regression output schema is nil.",
			map[string]any{"Reason": "nil output schema"},
		)
	}
	cols := make(map[string]table.Column, len(schema.Fields))
	for i := range schema.Fields {
		f := &schema.Fields[i]
		col := make(table.FloatColumn, 0, len(rows))
		for _, r := range rows {
			col = append(col, r[f.Name])
		}
		cols[f.Name] = col
	}
	return table.NewTable(schema, cols, len(rows), hash)
}
