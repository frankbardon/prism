package table

import (
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
)

func TestFromInlineInferredHappyPath(t *testing.T) {
	values := []map[string]any{
		{"brand_id": "alpha", "score": 0.42},
		{"brand_id": "beta", "score": 0.71},
		{"brand_id": "gamma", "score": 0.58},
	}
	tbl, schema, err := FromInline("brand_scores", values, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	if tbl.NumRows() != 3 {
		t.Fatalf("NumRows = %d, want 3", tbl.NumRows())
	}
	if schema == nil {
		t.Fatal("schema is nil")
	}
	if got := tbl.FieldNames(); len(got) != 2 {
		t.Fatalf("FieldNames = %v, want 2 entries", got)
	}
	scoreCol, ok := tbl.Column("score")
	if !ok || scoreCol.Kind() != KindFloat {
		t.Fatalf("score column missing or wrong kind: %v %v", ok, scoreCol)
	}
	if scoreCol.ValueAt(1).(float64) != 0.71 {
		t.Fatalf("score[1] = %v, want 0.71", scoreCol.ValueAt(1))
	}
}

func TestFromInlineExplicitFields(t *testing.T) {
	values := []map[string]any{
		{"id": 1.0, "label": "a"},
		{"id": 2.0, "label": "b"},
	}
	fields := []spec.FieldSpec{
		{Name: "id", Type: "int"},
		{Name: "label", Type: "string"},
	}
	tbl, _, err := FromInline("ds", values, fields)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	if got := tbl.FieldNames(); got[0] != "id" || got[1] != "label" {
		t.Fatalf("FieldNames = %v, want [id label]", got)
	}
}

func TestFromInlineTypeMismatch(t *testing.T) {
	values := []map[string]any{
		{"x": "alpha"},
		{"x": 42.0},
	}
	_, _, err := FromInline("ds", values, nil)
	if err == nil {
		t.Fatalf("expected mismatch error")
	}
	ae, ok := err.(*prismerrors.AppError)
	if !ok {
		t.Fatalf("expected *AppError, got %T", err)
	}
	if ae.Code != "PRISM_RESOLVE_INLINE_TYPE_MISMATCH" {
		t.Fatalf("got %s, want PRISM_RESOLVE_INLINE_TYPE_MISMATCH", ae.Code)
	}
}

func TestFromInlineHashStable(t *testing.T) {
	values := []map[string]any{
		{"brand_id": "alpha", "score": 0.42},
		{"brand_id": "beta", "score": 0.71},
	}
	a, _, err := FromInline("ds", values, nil)
	if err != nil {
		t.Fatalf("FromInline a: %v", err)
	}
	b, _, err := FromInline("ds", values, nil)
	if err != nil {
		t.Fatalf("FromInline b: %v", err)
	}
	if a.Hash() != b.Hash() {
		t.Fatalf("hash mismatch: %s vs %s", a.Hash(), b.Hash())
	}

	// Perturb one value -> hash differs.
	perturbed := []map[string]any{
		{"brand_id": "alpha", "score": 0.99},
		{"brand_id": "beta", "score": 0.71},
	}
	c, _, err := FromInline("ds", perturbed, nil)
	if err != nil {
		t.Fatalf("FromInline c: %v", err)
	}
	if a.Hash() == c.Hash() {
		t.Fatalf("hash should differ after perturbation, both = %s", a.Hash())
	}
}

// TestPrismTableHashStability is the PHASE.md test gate. It asserts the
// canonical contract: identical (schema, rows) input -> identical Hash().
// Wrapped here as a thin alias over TestFromInlineHashStable so the
// gate name is greppable in CI logs.
func TestPrismTableHashStability(t *testing.T) {
	values := []map[string]any{
		{"brand_id": "alpha", "score": 0.42},
		{"brand_id": "beta", "score": 0.71},
		{"brand_id": "gamma", "score": 0.58},
	}
	a, _, err := FromInline("brand_scores", values, nil)
	if err != nil {
		t.Fatalf("FromInline a: %v", err)
	}
	b, _, err := FromInline("brand_scores", values, nil)
	if err != nil {
		t.Fatalf("FromInline b: %v", err)
	}
	if a.Hash() != b.Hash() {
		t.Fatalf("Hash() unstable: %s vs %s", a.Hash(), b.Hash())
	}
}

// TestPrismTableRowLimit is the PHASE.md test gate. PRISM_TABLE_MAX_ROWS
// = 10; building an 11-row Table must return PRISM_RESOLVE_007. Already
// covered by TestNewTableRowLimit; named explicitly here for greppability.
func TestPrismTableRowLimit(t *testing.T) {
	TestNewTableRowLimit(t)
}

func TestFromInlineEmptyRejected(t *testing.T) {
	_, _, err := FromInline("ds", nil, nil)
	if err == nil {
		t.Fatalf("expected error for empty inline call")
	}
}
