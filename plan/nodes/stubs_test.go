package nodes_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
)

// stubInputSchema returns a small schema all stubs reuse. Keeps the
// assertions focused on the node behaviour rather than schema plumbing.
func stubInputSchema() *encoding.Schema {
	return &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "score", Type: encoding.FieldTypeF64},
		{Name: "age", Type: encoding.FieldTypeU8},
	}}
}

// assertNotImplemented confirms Execute returns the canonical
// PRISM_COMPILE_001 AppError with the right NodeType context.
func assertNotImplemented(t *testing.T, kind string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s.Execute: expected PRISM_COMPILE_001, got nil error", kind)
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("%s.Execute: expected *AppError, got %T (%v)", kind, err, err)
	}
	if ae.Code != "PRISM_COMPILE_001" {
		t.Fatalf("%s.Execute: code=%q, want PRISM_COMPILE_001", kind, ae.Code)
	}
	if got := ae.Context["NodeType"]; got != kind {
		t.Errorf("%s.Execute: NodeType context = %v, want %q", kind, got, kind)
	}
}

func TestPrismFilterNodeStub(t *testing.T) {
	n := nodes.NewFilter("filter:1", "src:1", "score > 0.5")
	if n.ID() != "filter:1" {
		t.Fatalf("ID=%q", n.ID())
	}
	if got := n.Inputs(); len(got) != 1 || got[0] != "src:1" {
		t.Fatalf("Inputs=%v", got)
	}
	out, err := n.Schema([]*encoding.Schema{stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(out.Fields) != 3 {
		t.Errorf("Schema fields=%d, want 3 (filter is shape-preserving)", len(out.Fields))
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "FilterNode", err)
	// Fingerprint determinism + sensitivity.
	m := nodes.NewFilter("filter:1", "src:1", "score > 0.5")
	if n.Fingerprint() != m.Fingerprint() {
		t.Errorf("fingerprint mismatch for identical filters")
	}
	d := nodes.NewFilter("filter:1", "src:1", "score < 0.5")
	if n.Fingerprint() == d.Fingerprint() {
		t.Errorf("fingerprint collision for different exprs")
	}
}

func TestPrismProjectNodeStub(t *testing.T) {
	n := nodes.NewProject("p:1", "src:1", []string{"brand_id", "score"})
	out, err := n.Schema([]*encoding.Schema{stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(out.Fields) != 2 {
		t.Errorf("projection fields=%d, want 2", len(out.Fields))
	}
	if out.Fields[0].Name != "brand_id" || out.Fields[1].Name != "score" {
		t.Errorf("projection order wrong: %+v", out.Fields)
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "ProjectNode", err)

	bad := nodes.NewProject("p:2", "src:1", []string{"nonexistent"})
	_, err = bad.Schema([]*encoding.Schema{stubInputSchema()})
	if err == nil {
		t.Fatal("expected PRISM_PLAN_003 for missing field, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_PLAN_003" {
		t.Errorf("expected PRISM_PLAN_003, got %v", err)
	}
}

func TestPrismGroupAggregateNodeStub(t *testing.T) {
	n := nodes.NewGroupAggregate("ga:1", "src:1",
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "mean_score"}},
	)
	out, err := n.Schema([]*encoding.Schema{stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(out.Fields) != 2 {
		t.Fatalf("Schema fields=%d, want 2", len(out.Fields))
	}
	if out.Fields[0].Name != "brand_id" || out.Fields[1].Name != "mean_score" {
		t.Errorf("Schema field order wrong: %+v", out.Fields)
	}
	if out.Fields[1].Type != encoding.FieldTypeF64 {
		t.Errorf("aggregate result type = %v, want F64", out.Fields[1].Type)
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "GroupAggregateNode", err)
}

func TestPrismCalculateNodeStub(t *testing.T) {
	n := nodes.NewCalculate("c:1", "src:1", "score * 2", "doubled")
	out, err := n.Schema([]*encoding.Schema{stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(out.Fields) != 4 || out.Fields[3].Name != "doubled" {
		t.Errorf("Schema fields=%+v", out.Fields)
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "CalculateNode", err)
}

func TestPrismWindowNodeStub(t *testing.T) {
	n := nodes.NewWindow("w:1", "src:1",
		[]nodes.WindowOp{{Op: "rank", As: "rk"}},
		[]string{"brand_id"},
		[]nodes.SortKey{{Field: "score", Order: "desc"}},
		nil,
	)
	out, err := n.Schema([]*encoding.Schema{stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(out.Fields) != 4 || out.Fields[3].Name != "rk" {
		t.Errorf("Schema fields=%+v", out.Fields)
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "WindowNode", err)
}

func TestPrismSortNodeStub(t *testing.T) {
	n := nodes.NewSort("s:1", "src:1", []nodes.SortKey{{Field: "score", Order: "desc"}})
	out, err := n.Schema([]*encoding.Schema{stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(out.Fields) != 3 {
		t.Errorf("sort preserves schema, fields=%d", len(out.Fields))
	}
	if !strings.Contains(n.SortLabel(), "score:desc") {
		t.Errorf("SortLabel=%q", n.SortLabel())
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "SortNode", err)
}

func TestPrismLimitNodeStub(t *testing.T) {
	n := nodes.NewLimit("l:1", "src:1", 10, 5)
	out, err := n.Schema([]*encoding.Schema{stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(out.Fields) != 3 {
		t.Errorf("limit preserves schema, fields=%d", len(out.Fields))
	}
	if n.Limit() != 10 || n.Offset() != 5 {
		t.Errorf("Limit/Offset = %d/%d", n.Limit(), n.Offset())
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "LimitNode", err)
}

func TestPrismBinNodeStub(t *testing.T) {
	n := nodes.NewBin("b:1", "src:1", "score", "score_bin", nodes.BinParams{Auto: true})
	out, err := n.Schema([]*encoding.Schema{stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(out.Fields) != 4 || out.Fields[3].Name != "score_bin" {
		t.Errorf("Schema fields=%+v", out.Fields)
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "BinNode", err)
}

func TestPrismJoinNodeStub(t *testing.T) {
	left := stubInputSchema()
	right := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "label", Type: encoding.FieldTypeCategoricalU8},
	}}
	n := nodes.NewJoin("j:1", "left", "right", []string{"brand_id"}, nodes.JoinInner, 1000)
	if got := n.Inputs(); len(got) != 2 || got[0] != "left" || got[1] != "right" {
		t.Fatalf("Inputs=%v", got)
	}
	out, err := n.Schema([]*encoding.Schema{left, right})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	names := make([]string, len(out.Fields))
	for i, f := range out.Fields {
		names[i] = f.Name
	}
	if strings.Join(names, ",") != "brand_id,score,age,label" {
		t.Errorf("join schema fields=%v, want [brand_id score age label]", names)
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "JoinNode", err)
}

func TestPrismUnionNodeStub(t *testing.T) {
	n := nodes.NewUnion("u:1", []plan.NodeID{"a", "b"})
	out, err := n.Schema([]*encoding.Schema{stubInputSchema(), stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(out.Fields) != 3 {
		t.Errorf("union schema fields=%d, want 3 (first-input shape)", len(out.Fields))
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "UnionNode", err)
}

func TestPrismPivotNodeStub(t *testing.T) {
	n := nodes.NewPivot("pv:1", "src:1", "brand_id", "score", []string{"age"}, "sum")
	out, err := n.Schema([]*encoding.Schema{stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	// P03 conservative default — input shape verbatim (TODO P04).
	if len(out.Fields) != 3 {
		t.Errorf("pivot stub schema fields=%d, want 3 (conservative)", len(out.Fields))
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "PivotNode", err)
}

func TestPrismUnpivotNodeStub(t *testing.T) {
	n := nodes.NewUnpivot("up:1", "src:1", []string{"score", "age"}, []string{"metric", "value"})
	out, err := n.Schema([]*encoding.Schema{stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	// Drops `score` and `age`, keeps `brand_id`, appends `metric` + `value`.
	if len(out.Fields) != 3 {
		t.Fatalf("unpivot schema fields=%d, want 3, got %+v", len(out.Fields), out.Fields)
	}
	names := []string{out.Fields[0].Name, out.Fields[1].Name, out.Fields[2].Name}
	if names[0] != "brand_id" || names[1] != "metric" || names[2] != "value" {
		t.Errorf("unpivot field order wrong: %v", names)
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "UnpivotNode", err)
}

func TestPrismSampleNodeStub(t *testing.T) {
	seed := int64(42)
	n := nodes.NewSample("sm:1", "src:1", 100, &seed)
	out, err := n.Schema([]*encoding.Schema{stubInputSchema()})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if len(out.Fields) != 3 {
		t.Errorf("sample preserves schema, fields=%d", len(out.Fields))
	}
	if n.N() != 100 {
		t.Errorf("N=%d", n.N())
	}
	_, err = n.Execute(context.Background(), nil)
	assertNotImplemented(t, "SampleNode", err)
}
