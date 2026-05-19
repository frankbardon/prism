package inmem

import (
	"context"
	"errors"
	"math"
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// approxEqual tolerates Welford-style float drift on small samples.
func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// helperInlineTable returns a 4-row table with brand_id + score +
// age — the schema every per-op smoke test reuses.
func helperInlineTable(t *testing.T) *table.Table {
	t.Helper()
	tbl, _, err := table.FromInline("test",
		[]map[string]any{
			{"brand_id": "alpha", "score": 0.3, "age": 21.0},
			{"brand_id": "alpha", "score": 0.7, "age": 35.0},
			{"brand_id": "beta", "score": 0.4, "age": 41.0},
			{"brand_id": "beta", "score": 0.8, "age": 28.0},
		},
		[]spec.FieldSpec{
			{Name: "brand_id", Type: "string"},
			{Name: "score", Type: "float64"},
			{Name: "age", Type: "float64"},
		})
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	return tbl
}

func TestPrismInMemBackendDispatchFilter(t *testing.T) {
	in := helperInlineTable(t)
	b := New()
	n := nodes.NewFilter("filter:1", "src", "score > 0.5")
	out, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if got := out.NumRows(); got != 2 {
		t.Errorf("rows = %d, want 2", got)
	}
}

func TestPrismInMemBackendDispatchProject(t *testing.T) {
	in := helperInlineTable(t)
	b := New()
	n := nodes.NewProject("p:1", "src", []string{"brand_id", "score"})
	out, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if got := len(out.FieldNames()); got != 2 {
		t.Errorf("fields = %d, want 2", got)
	}
}

func TestPrismInMemBackendDispatchSort(t *testing.T) {
	in := helperInlineTable(t)
	b := New()
	n := nodes.NewSort("s:1", "src", []nodes.SortKey{{Field: "score", Order: "desc"}})
	out, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	col, _ := out.Column("score")
	if col.ValueAt(0).(float64) < col.ValueAt(out.NumRows()-1).(float64) {
		t.Errorf("desc sort: head=%v tail=%v", col.ValueAt(0), col.ValueAt(out.NumRows()-1))
	}
}

func TestPrismInMemBackendDispatchLimit(t *testing.T) {
	in := helperInlineTable(t)
	b := New()
	n := nodes.NewLimit("l:1", "src", 2, 1)
	out, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if got := out.NumRows(); got != 2 {
		t.Errorf("rows = %d, want 2", got)
	}
}

func TestPrismInMemBackendDispatchSample(t *testing.T) {
	in := helperInlineTable(t)
	b := New()
	seed := int64(42)
	n := nodes.NewSample("sm:1", "src", 2, &seed)
	out, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if got := out.NumRows(); got != 2 {
		t.Errorf("rows = %d, want 2", got)
	}
	// Determinism: same seed twice → same result.
	out2, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile 2: %v", err)
	}
	c1, _ := out.Column("score")
	c2, _ := out2.Column("score")
	for i := 0; i < c1.Len(); i++ {
		if c1.ValueAt(i) != c2.ValueAt(i) {
			t.Errorf("deterministic seed produced different sample at %d", i)
		}
	}
}

func TestPrismInMemBackendDispatchCalculate(t *testing.T) {
	in := helperInlineTable(t)
	b := New()
	n := nodes.NewCalculate("c:1", "src", "score * 2", "doubled")
	out, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	col, ok := out.Column("doubled")
	if !ok {
		t.Fatal("missing 'doubled' column")
	}
	if got := col.ValueAt(0).(float64); got != 0.6 {
		t.Errorf("doubled[0] = %v, want 0.6", got)
	}
}

func TestPrismInMemBackendDispatchGroupAggregate(t *testing.T) {
	in := helperInlineTable(t)
	b := New()
	n := nodes.NewGroupAggregate("ga:1", "src",
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "avg"}},
	)
	out, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if got := out.NumRows(); got != 2 {
		t.Fatalf("rows = %d, want 2 (alpha + beta)", got)
	}

	// alpha mean = (0.3 + 0.7) / 2 = 0.5; beta mean = (0.4 + 0.8) / 2 = 0.6
	brandCol, _ := out.Column("brand_id")
	avgCol, _ := out.Column("avg")
	for i := 0; i < out.NumRows(); i++ {
		brand := brandCol.ValueAt(i).(string)
		avg := avgCol.ValueAt(i).(float64)
		switch brand {
		case "alpha":
			if !approxEqual(avg, 0.5) {
				t.Errorf("alpha avg = %v, want ≈0.5", avg)
			}
		case "beta":
			if !approxEqual(avg, 0.6) {
				t.Errorf("beta avg = %v, want ≈0.6", avg)
			}
		default:
			t.Errorf("unexpected brand %q", brand)
		}
	}
}

func TestPrismInMemBackendDispatchBin(t *testing.T) {
	in := helperInlineTable(t)
	b := New()
	n := nodes.NewBin("b:1", "src", "score", "score_bin", nodes.BinParams{Auto: true})
	out, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if got := out.NumRows(); got != 4 {
		t.Errorf("rows = %d, want 4", got)
	}
	col, ok := out.Column("score_bin")
	if !ok || col.Len() != 4 {
		t.Errorf("missing or wrong-length score_bin column")
	}
}

func TestPrismInMemBackendDispatchWindow(t *testing.T) {
	in := helperInlineTable(t)
	b := New()
	n := nodes.NewWindow("w:1", "src",
		[]nodes.WindowOp{{Op: "row_number", As: "rn"}},
		nil,
		[]nodes.SortKey{{Field: "score", Order: "asc"}},
		nil,
	)
	out, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	col, ok := out.Column("rn")
	if !ok || col.Len() != 4 {
		t.Errorf("missing or wrong-length rn column")
	}
}

func TestPrismInMemBackendStubFallthrough(t *testing.T) {
	b := New()
	// Pick a node type the backend does not implement (JoinNode).
	n := nodes.NewJoin("j:1", "l", "r", []string{"brand_id"}, nodes.JoinInner, 0)
	_, err := b.Compile(context.Background(), n, nil)
	if err == nil {
		t.Fatal("expected PRISM_COMPILE_001, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AppError, got %T", err)
	}
	if ae.Code != "PRISM_COMPILE_001" {
		t.Errorf("code = %q, want PRISM_COMPILE_001", ae.Code)
	}
}

func TestPrismFilterCompile002OnBadExpr(t *testing.T) {
	in := helperInlineTable(t)
	b := New()
	n := nodes.NewFilter("filter:bad", "src", "score >")
	_, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err == nil {
		t.Fatal("expected PRISM_COMPILE_002, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AppError, got %T", err)
	}
	if ae.Code != "PRISM_COMPILE_002" {
		t.Errorf("code = %q, want PRISM_COMPILE_002", ae.Code)
	}
}
