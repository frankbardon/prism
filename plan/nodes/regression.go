package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/vfs"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// RegressionNode fits an OLS regression over its single upstream input
// table and emits the two endpoints of the fitted trend line.
// Execution is pure Go via the in-memory backend
// (compile/inmem/regression.go) — the node carries no engine coupling.
//
// The in-memory OLS fit produces the coefficients (intercept + slope);
// the predictor min/max supply the x-domain. The node synthesises two
// rows — (xmin, y(xmin)) and (xmax, y(xmax)) — so a `line` mark over
// (predictor, fitted) draws the regression line. For OLS every fitted
// point is collinear, so two endpoints suffice and match Vega-Lite's
// sampled regression output. v1 supports a single predictor — the only
// shape that maps to a 2-D trend line.
type RegressionNode struct {
	id        plan.NodeID
	input     plan.NodeID
	ref       string
	fs        vfs.Fs
	body      spec.RegressionBody
	predictor string
	fittedAs  string
	backend   plan.Backend
}

// NewRegression constructs a RegressionNode over the given upstream
// input. The upstream may be ANY node (a materialised leaf or a derived
// transform) — the pure-Go OLS fit reads whatever table the input
// yields. The constructor performs only schema-independent structural
// validation (target present, exactly one predictor); the {predictor,
// fitted} output schema is derived at execute time (see Schema).
func NewRegression(id, input plan.NodeID, ref string, fs vfs.Fs, t *spec.RegressionTransform) (*RegressionNode, error) {
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
	return &RegressionNode{
		id:        id,
		input:     input,
		ref:       ref,
		fs:        fs,
		body:      body,
		predictor: predictor,
		fittedAs:  fittedAs,
	}, nil
}

// DeriveRegressionID hashes the upstream input ref together with the
// canonical body shape so two equivalent regression nodes (same input,
// same body) hash identically. The ref is the upstream node id — a
// materialised leaf or any derived transform tip.
func DeriveRegressionID(ref string, t *spec.RegressionTransform) plan.NodeID {
	h := sha256.New()
	h.Write([]byte(ref))
	h.Write([]byte{0})
	h.Write([]byte(regressionBodyKey(t.Regression)))
	return plan.NodeID("regression:" + hex.EncodeToString(h.Sum(nil)[:8]))
}

// ID implements plan.Node.
func (n *RegressionNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node. Regression consumes the single upstream
// table it fits.
func (n *RegressionNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Derived at execute time — requires a
// single upstream input (mirrors GroupAggregateNode) and returns the
// native {predictor, fitted} two-row shape, both F64.
func (n *RegressionNode) Schema(in []*table.Schema) (*table.Schema, error) {
	if _, err := requireSingleInput("RegressionNode", in); err != nil {
		return nil, err
	}
	return &table.Schema{Fields: []table.Field{
		{Name: n.predictor, Type: table.FieldTypeF64},
		{Name: n.fittedAs, Type: table.FieldTypeF64},
	}}, nil
}

// Fingerprint implements plan.Node.
func (n *RegressionNode) Fingerprint() string {
	return fingerprintFor("RegressionNode", string(n.input), n.ref, regressionBodyKey(n.body))
}

// Ref returns the upstream input ref (a leaf ref or a derived transform
// tip id) so plan-visualisation tooling can show what the regression
// fits. Mirrors SourceNode.Ref().
func (n *RegressionNode) Ref() string { return n.ref }

// FS returns the afero filesystem this node was constructed with.
func (n *RegressionNode) FS() vfs.Fs { return n.fs }

// Body returns the regression body for renderer + test inspection.
func (n *RegressionNode) Body() spec.RegressionBody { return n.body }

// Predictor returns the resolved single predictor field name.
func (n *RegressionNode) Predictor() string { return n.predictor }

// FittedAs returns the resolved fitted-value output column name.
func (n *RegressionNode) FittedAs() string { return n.fittedAs }

// Kind implements plan.Labeled.
func (n *RegressionNode) Kind() string { return "RegressionNode" }

// Summary implements plan.Labeled. "fitted: y ~ x".
func (n *RegressionNode) Summary() string {
	return fmt.Sprintf("%s: %s ~ %s", n.fittedAs, n.body.Target, n.predictor)
}

// Execute implements plan.Node via the injected backend.
func (n *RegressionNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("RegressionNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute.
func (n *RegressionNode) SetBackend(b plan.Backend) { n.backend = b }

// regressionBodyKey returns a stable canonical key for fingerprinting.
func regressionBodyKey(b spec.RegressionBody) string {
	as := b.As
	if as == "" {
		as = "fitted"
	}
	return fmt.Sprintf("target=%s;pred=%s;as=%s", b.Target, strings.Join(b.Predictors, ","), as)
}
