package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/frankbardon/pulse"
	"github.com/frankbardon/pulse/encoding"
	pulsetypes "github.com/frankbardon/pulse/types"
	"github.com/spf13/afero"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

const (
	regXMinLabel    = "_prism_xmin"
	regXMaxLabel    = "_prism_xmax"
	regInterceptKey = "(intercept)"
)

// RegressionNode fits an OLS regression over the source cohort and
// emits the two endpoints of the fitted trend line. Like CrosstabNode
// it is its own leaf — it opens the .pulse cohort directly, since Pulse
// has no in-memory cohort constructor.
//
// One Pulse Request carries both REG_OLS (for the coefficients) and
// min/max aggregations over the predictor (for the x-domain). The node
// synthesises two rows — (xmin, y(xmin)) and (xmax, y(xmax)) — so a
// `line` mark over (predictor, fitted) draws the regression line. For
// OLS every fitted point is collinear, so two endpoints suffice and
// match Vega-Lite's sampled regression output. v1 supports a single
// predictor — the only shape that maps to a 2-D trend line.
type RegressionNode struct {
	id        plan.NodeID
	ref       string
	fs        afero.Fs
	body      spec.RegressionBody
	outSchema *encoding.Schema
	predictor string
	fittedAs  string
}

// NewRegression constructs a RegressionNode. inSchema is the source
// cohort's schema; the output schema is {predictor, fitted}, two rows.
func NewRegression(id plan.NodeID, ref string, fs afero.Fs, inSchema *encoding.Schema, t *spec.RegressionTransform) (*RegressionNode, error) {
	if inSchema == nil {
		return nil, fmt.Errorf("regression: nil input schema")
	}
	body := t.Regression
	if body.Target == "" {
		return nil, prismerrors.New(
			"PRISM_SPEC_035",
			"regression.target is required.",
			map[string]any{},
		)
	}
	if len(body.Predictors) != 1 {
		return nil, prismerrors.New(
			"PRISM_SPEC_035",
			fmt.Sprintf("regression.predictors must list exactly one field for a trend line (got %d).", len(body.Predictors)),
			map[string]any{"Target": body.Target, "Predictors": body.Predictors},
		)
	}
	fittedAs := body.As
	if fittedAs == "" {
		fittedAs = "fitted"
	}
	predictor := body.Predictors[0]
	out := &encoding.Schema{Fields: []encoding.Field{
		{Name: predictor, Type: encoding.FieldTypeF64},
		{Name: fittedAs, Type: encoding.FieldTypeF64},
	}}
	return &RegressionNode{
		id:        id,
		ref:       ref,
		fs:        fs,
		body:      body,
		outSchema: out,
		predictor: predictor,
		fittedAs:  fittedAs,
	}, nil
}

// DeriveRegressionID hashes the source ref together with the canonical
// body shape so two equivalent regression nodes hash identically.
func DeriveRegressionID(ref string, t *spec.RegressionTransform) plan.NodeID {
	h := sha256.New()
	h.Write([]byte(ref))
	h.Write([]byte{0})
	h.Write([]byte(regressionBodyKey(t.Regression)))
	return plan.NodeID("regression:" + hex.EncodeToString(h.Sum(nil)[:8]))
}

// ID implements plan.Node.
func (n *RegressionNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node. Regression is a leaf — Pulse fits the
// cohort directly.
func (n *RegressionNode) Inputs() []plan.NodeID { return nil }

// Schema implements plan.Node. Pre-computed at construction; converted
// to the native shape via the E1 shim (the node still builds a Pulse
// schema internally until the leaf executors migrate in E4).
func (n *RegressionNode) Schema(_ []*table.Schema) (*table.Schema, error) {
	return table.FromPulseSchema(n.outSchema), nil
}

// Fingerprint implements plan.Node.
func (n *RegressionNode) Fingerprint() string {
	return fingerprintFor("RegressionNode", n.ref, regressionBodyKey(n.body))
}

// Ref returns the source ref so plan-visualisation tooling can show the
// underlying cohort. Mirrors SourceNode.Ref().
func (n *RegressionNode) Ref() string { return n.ref }

// FS returns the afero filesystem this node was constructed with.
func (n *RegressionNode) FS() afero.Fs { return n.fs }

// Body returns the regression body for renderer + test inspection.
func (n *RegressionNode) Body() spec.RegressionBody { return n.body }

// Kind implements plan.Labeled.
func (n *RegressionNode) Kind() string { return "RegressionNode" }

// Summary implements plan.Labeled. "fitted: y ~ x".
func (n *RegressionNode) Summary() string {
	return fmt.Sprintf("%s: %s ~ %s", n.fittedAs, n.body.Target, n.predictor)
}

// Execute implements plan.Node. Runs one Pulse Process carrying REG_OLS
// plus min/max aggregations over the predictor, then emits the two
// fitted trend-line endpoints.
func (n *RegressionNode) Execute(ctx context.Context, _ []*table.Table) (*table.Table, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if n.fs == nil {
		return nil, fmt.Errorf("regression: fs is nil")
	}
	p, err := pulse.New(pulse.Options{FS: n.fs})
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse failed to open %s for regression: %v.", n.ref, err),
			map[string]any{"Ref": n.ref, "Reason": err.Error()},
			err,
		)
	}
	req := &pulsetypes.Request{
		Cohort: &pulsetypes.Cohort{Filename: n.ref},
		Aggregations: []*pulsetypes.Aggregation{
			{Type: pulsetypes.AGG_MIN, Field: n.predictor, Label: regXMinLabel},
			{Type: pulsetypes.AGG_MAX, Field: n.predictor, Label: regXMaxLabel},
		},
		Regressions: []*pulsetypes.RegressionSpec{{
			Type:       pulsetypes.REG_OLS,
			Target:     n.body.Target,
			Predictors: []string{n.predictor},
		}},
	}
	resp, err := p.Process(ctx, req)
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_PLAN_REGRESSION_PROCESS",
			fmt.Sprintf("Pulse failed to fit regression on %s: %v.", n.ref, err),
			map[string]any{"Ref": n.ref, "Reason": err.Error()},
			err,
		)
	}
	if len(resp.Regressions) == 0 || resp.Regressions[0].Coefficients == nil {
		return nil, prismerrors.New(
			"PRISM_PLAN_REGRESSION_PROCESS",
			fmt.Sprintf("Pulse returned no regression coefficients for %s.", n.ref),
			map[string]any{"Ref": n.ref},
		)
	}
	coef := resp.Regressions[0].Coefficients
	intercept := coef[regInterceptKey]
	slope, ok := coef[n.predictor]
	if !ok {
		return nil, prismerrors.New(
			"PRISM_PLAN_REGRESSION_PROCESS",
			fmt.Sprintf("Pulse regression result missing coefficient for predictor %q.", n.predictor),
			map[string]any{"Ref": n.ref, "Predictor": n.predictor},
		)
	}
	if len(resp.Data) == 0 {
		return nil, prismerrors.New(
			"PRISM_PLAN_REGRESSION_PROCESS",
			fmt.Sprintf("Pulse returned no predictor range for %s.", n.ref),
			map[string]any{"Ref": n.ref},
		)
	}
	xmin, _ := coerceFloatRow(resp.Data[0][regXMinLabel])
	xmax, _ := coerceFloatRow(resp.Data[0][regXMaxLabel])

	// Two endpoints of the fitted line; reuse the long-row materialiser.
	rows := []map[string]any{
		{n.predictor: xmin, n.fittedAs: intercept + slope*xmin},
		{n.predictor: xmax, n.fittedAs: intercept + slope*xmax},
	}
	return tableFromLongRows(rows, n.outSchema, n.id)
}

// tableFromLongRows materialises long-form rows (Pulse Response.Data)
// into a typed *table.Table using the Pulse schema. Regression is still
// Pulse-backed (converted in a later slice); the crosstab path has moved
// to the pure-Go in-memory materialiser (compile/inmem/crosstab.go).
func tableFromLongRows(rows []map[string]any, schema *encoding.Schema, id plan.NodeID) (*table.Table, error) {
	if schema == nil {
		return nil, fmt.Errorf("regression: nil output schema")
	}
	cols := make(map[string]table.Column, len(schema.Fields))
	for i := range schema.Fields {
		f := &schema.Fields[i]
		switch table.KindFromPulseFieldType(f.Type) {
		case table.KindString:
			cols[f.Name] = make(table.StringColumn, 0, len(rows))
		case table.KindFloat:
			cols[f.Name] = make(table.FloatColumn, 0, len(rows))
		case table.KindInt:
			cols[f.Name] = make(table.IntColumn, 0, len(rows))
		case table.KindBool:
			cols[f.Name] = make(table.BoolColumn, 0, len(rows))
		case table.KindDate:
			cols[f.Name] = make(table.DateColumn, 0, len(rows))
		default:
			return nil, fmt.Errorf("regression: unsupported field type %s for %q", f.Type, f.Name)
		}
	}
	for _, row := range rows {
		for i := range schema.Fields {
			f := &schema.Fields[i]
			raw, present := row[f.Name]
			switch table.KindFromPulseFieldType(f.Type) {
			case table.KindString:
				s := ""
				if present && raw != nil {
					s = coerceString(raw)
				}
				cols[f.Name] = append(cols[f.Name].(table.StringColumn), s)
			case table.KindFloat:
				v := 0.0
				if present && raw != nil {
					v, _ = coerceFloatRow(raw)
				}
				cols[f.Name] = append(cols[f.Name].(table.FloatColumn), v)
			case table.KindInt:
				v := int64(0)
				if present && raw != nil {
					if f64, ok := coerceFloatRow(raw); ok {
						v = int64(f64)
					}
				}
				cols[f.Name] = append(cols[f.Name].(table.IntColumn), v)
			case table.KindBool:
				b := false
				if present && raw != nil {
					b, _ = raw.(bool)
				}
				cols[f.Name] = append(cols[f.Name].(table.BoolColumn), b)
			case table.KindDate:
				v := int64(0)
				if present && raw != nil {
					if f64, ok := coerceFloatRow(raw); ok {
						v = int64(f64)
					}
				}
				cols[f.Name] = append(cols[f.Name].(table.DateColumn), v)
			}
		}
	}
	hash := "regression:" + string(id)
	return table.NewTable(table.FromPulseSchema(schema), cols, len(rows), hash)
}

// regressionBodyKey returns a stable canonical key for fingerprinting.
func regressionBodyKey(b spec.RegressionBody) string {
	as := b.As
	if as == "" {
		as = "fitted"
	}
	return fmt.Sprintf("target=%s;pred=%s;as=%s", b.Target, strings.Join(b.Predictors, ","), as)
}
