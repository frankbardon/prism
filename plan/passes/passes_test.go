package passes_test

import (
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/plan/passes"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// fieldSpec is a small helper for declaring inline schema fields.
type fieldSpec struct {
	name, typ string
}

// srcWithFields builds a SourceNode whose OutputSchema derives from a
// single synthetic inline row carrying the named fields. The optimizer
// passes consult only the schema's field names (not row values), so one
// representative row per field suffices. The Pulse-free ingestion path
// (E4) materialises the schema through the resolver's inline seam — no
// `.pulse` bytes are read.
func srcWithFields(t *testing.T, ref string, fields []fieldSpec) (*nodes.SourceNode, afero.Fs) {
	t.Helper()
	row := map[string]any{}
	specFields := make([]spec.FieldSpec, 0, len(fields))
	for _, f := range fields {
		specFields = append(specFields, spec.FieldSpec{Name: f.name, Type: f.typ})
		switch f.typ {
		case "f64", "u8":
			row[f.name] = 0.0
		default:
			row[f.name] = "x"
		}
	}
	data := resolve.MapDataResolver{ref: {Values: []map[string]any{row}, Fields: specFields}}
	fs := afero.NewMemMapFs()
	return nodes.New(ref, fs, resolve.NewWithData(nil, data)), fs
}

func TestPrismDedupSourcesNoop(t *testing.T) {
	src, _ := srcWithFields(t, "a", []fieldSpec{{"v", "f64"}})
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(src.ID())
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	out, changed, err := passes.DedupSourcesPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if changed {
		t.Errorf("noop case should not report changed")
	}
	if out.Size() != d.Size() {
		t.Errorf("size changed: %d vs %d", out.Size(), d.Size())
	}
}

// The dedup pass type-asserts on *nodes.SourceNode and the builder
// already shares ids for matching refs, so we only test the no-op
// baseline + rely on the rewire code review for correctness when a
// future layer-builder produces distinct-id same-ref Sources.

// TestPrismFilterPushdownLeftSide builds a Filter(brand_id="alpha")
// over a Join(L on brand_id) and asserts the pass moves the filter
// under the left input.
func TestPrismFilterPushdownLeftSide(t *testing.T) {
	leftFields := []fieldSpec{{"brand_id", "categorical_u8"}, {"score", "f64"}}
	rightFields := []fieldSpec{{"brand_id", "categorical_u8"}, {"label", "categorical_u8"}}
	left, _ := srcWithFields(t, "left", leftFields)
	right, _ := srcWithFields(t, "right", rightFields)

	b := plan.NewBuilder()
	_ = b.AddNode(left)
	_ = b.AddNode(right)
	_ = b.MarkRoot(left.ID())
	_ = b.MarkRoot(right.ID())
	join := nodes.NewJoin("j1", left.ID(), right.ID(), []string{"brand_id"}, nodes.JoinInner, 0)
	_ = b.AddNode(join)
	// Filter references `score` — exclusively in the left schema.
	filt := nodes.NewFilter("f1", join.ID(), spec.Predicate{Op: spec.PredGt, Field: "score", Value: 0.5})
	_ = b.AddNode(filt)
	_ = b.MarkSink(filt.ID())
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	out, changed, err := passes.FilterPushdownPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("FilterPushdownPass should report changed for left-only filter")
	}
	// After the pass, the filter's input is the left source, and the
	// join's left input is the filter.
	fAfter, _ := out.Node("f1")
	if got := fAfter.Inputs()[0]; got != left.ID() {
		t.Errorf("filter input=%q; want %q (left source)", got, left.ID())
	}
	jAfter, _ := out.Node("j1")
	if got := jAfter.Inputs()[0]; got != "f1" {
		t.Errorf("join left input=%q; want f1 (pushed-down filter)", got)
	}
	if got := jAfter.Inputs()[1]; got != right.ID() {
		t.Errorf("join right input=%q; want %q", got, right.ID())
	}
}

// TestPrismFilterPushdownRightSide is the symmetric case.
func TestPrismFilterPushdownRightSide(t *testing.T) {
	leftFields := []fieldSpec{{"brand_id", "categorical_u8"}, {"score", "f64"}}
	rightFields := []fieldSpec{{"brand_id", "categorical_u8"}, {"label", "categorical_u8"}}
	left, _ := srcWithFields(t, "leftR", leftFields)
	right, _ := srcWithFields(t, "rightR", rightFields)
	b := plan.NewBuilder()
	_ = b.AddNode(left)
	_ = b.AddNode(right)
	_ = b.MarkRoot(left.ID())
	_ = b.MarkRoot(right.ID())
	join := nodes.NewJoin("j2", left.ID(), right.ID(), []string{"brand_id"}, nodes.JoinInner, 0)
	_ = b.AddNode(join)
	filt := nodes.NewFilter("f2", join.ID(), spec.Predicate{Op: spec.PredEq, Field: "label", Value: "alpha"})
	_ = b.AddNode(filt)
	_ = b.MarkSink(filt.ID())
	d, _ := b.Build()

	out, changed, err := passes.FilterPushdownPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("expected change for right-only filter")
	}
	fAfter, _ := out.Node("f2")
	if got := fAfter.Inputs()[0]; got != right.ID() {
		t.Errorf("filter input=%q; want %q (right source)", got, right.ID())
	}
	jAfter, _ := out.Node("j2")
	if got := jAfter.Inputs()[1]; got != "f2" {
		t.Errorf("join right input=%q; want f2", got)
	}
}

// TestPrismFilterPushdownMixedColumnsNoOp asserts a filter referencing
// both sides stays where it is.
func TestPrismFilterPushdownMixedColumnsNoOp(t *testing.T) {
	leftFields := []fieldSpec{{"brand_id", "categorical_u8"}, {"score", "f64"}}
	rightFields := []fieldSpec{{"brand_id", "categorical_u8"}, {"label", "categorical_u8"}}
	left, _ := srcWithFields(t, "leftM", leftFields)
	right, _ := srcWithFields(t, "rightM", rightFields)
	b := plan.NewBuilder()
	_ = b.AddNode(left)
	_ = b.AddNode(right)
	_ = b.MarkRoot(left.ID())
	_ = b.MarkRoot(right.ID())
	join := nodes.NewJoin("j3", left.ID(), right.ID(), []string{"brand_id"}, nodes.JoinInner, 0)
	_ = b.AddNode(join)
	// Filter references columns from both sides.
	filt := nodes.NewFilter("f3", join.ID(), spec.Predicate{And: []spec.Predicate{
		{Op: spec.PredGt, Field: "score", Value: 0.5},
		{Op: spec.PredNe, Field: "label", Value: ""},
	}})
	_ = b.AddNode(filt)
	_ = b.MarkSink(filt.ID())
	d, _ := b.Build()

	_, changed, err := passes.FilterPushdownPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if changed {
		t.Errorf("mixed-column filter should NOT be pushed down")
	}
}

// TestPrismProjectionPruning injects a Project below the Source when
// the GroupAggregate downstream uses only 2 of the source's 4 columns.
func TestPrismProjectionPruning(t *testing.T) {
	src, _ := srcWithFields(t, "proj", []fieldSpec{
		{"brand_id", "categorical_u8"},
		{"score", "f64"},
		{"age", "u8"},
		{"region", "categorical_u8"},
	})
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.MarkRoot(src.ID())
	ga := nodes.NewGroupAggregate("ga1", src.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "score_mean"}},
	)
	_ = b.AddNode(ga)
	_ = b.MarkSink(ga.ID())
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	out, changed, err := passes.ProjectionPruningPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("ProjectionPruning should report changed when only 2/4 cols are used")
	}
	// Expect: a new ProjectNode exists in the DAG. The GroupAggregate's
	// input should be the new project (not the source).
	gaAfter, _ := out.Node("ga1")
	gaInput := gaAfter.Inputs()[0]
	if gaInput == src.ID() {
		t.Errorf("GroupAggregate still consumes source directly; expected pruning project")
	}
	projNode, _ := out.Node(gaInput)
	proj, ok := projNode.(*nodes.ProjectNode)
	if !ok {
		t.Fatalf("expected ProjectNode at GA input, got %T", projNode)
	}
	if len(proj.Fields()) != 2 {
		t.Errorf("project fields=%v; want exactly 2 (brand_id, score)", proj.Fields())
	}
	want := map[string]bool{"brand_id": true, "score": true}
	for _, f := range proj.Fields() {
		if !want[f] {
			t.Errorf("project includes unexpected field %q", f)
		}
	}
}

// TestPrismAggregateFusion merges two sibling GroupAggregates sharing
// an input + groupby into a single node with the union of aggs.
func TestPrismAggregateFusion(t *testing.T) {
	src, _ := srcWithFields(t, "agg-fuse", []fieldSpec{
		{"brand_id", "categorical_u8"},
		{"score", "f64"},
	})
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.MarkRoot(src.ID())
	ga1 := nodes.NewGroupAggregate("ga-1", src.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "mean"}},
	)
	_ = b.AddNode(ga1)
	ga2 := nodes.NewGroupAggregate("ga-2", src.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "count", Field: "score", As: "n"}},
	)
	_ = b.AddNode(ga2)
	_ = b.MarkSink(ga1.ID())
	_ = b.MarkSink(ga2.ID())
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	out, changed, err := passes.AggregateFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("AggregateFusionPass should report changed for two same-groupby sibling aggs")
	}
	// Both originals should be gone; one merged node should exist with
	// 2 aggs.
	if _, ok := out.Node("ga-1"); ok {
		t.Errorf("ga-1 should have been removed")
	}
	if _, ok := out.Node("ga-2"); ok {
		t.Errorf("ga-2 should have been removed")
	}
	var merged *nodes.GroupAggregateNode
	for _, id := range out.Nodes() {
		n, _ := out.Node(id)
		if ga, ok := n.(*nodes.GroupAggregateNode); ok {
			merged = ga
		}
	}
	if merged == nil {
		t.Fatal("no merged GroupAggregateNode found")
	}
	if len(merged.Aggs()) != 2 {
		t.Errorf("merged aggs=%v; want 2", merged.Aggs())
	}
}

// TestPrismAggregateFusionDifferentGroupbyNoop confirms two GAs with
// different groupby keys are not merged.
func TestPrismAggregateFusionDifferentGroupbyNoop(t *testing.T) {
	src, _ := srcWithFields(t, "agg-noop", []fieldSpec{
		{"brand_id", "categorical_u8"},
		{"region", "categorical_u8"},
		{"score", "f64"},
	})
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.MarkRoot(src.ID())
	ga1 := nodes.NewGroupAggregate("ga-1n", src.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "m"}},
	)
	_ = b.AddNode(ga1)
	ga2 := nodes.NewGroupAggregate("ga-2n", src.ID(),
		[]string{"region"},
		[]nodes.AggOp{{Op: "count", Field: "score", As: "n"}},
	)
	_ = b.AddNode(ga2)
	_ = b.MarkSink(ga1.ID())
	_ = b.MarkSink(ga2.ID())
	d, _ := b.Build()

	_, changed, err := passes.AggregateFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if changed {
		t.Errorf("Different groupby keys must not be fused")
	}
}

// TestPrismSampleInjectionSkipsSmallSource asserts SampleInjection
// leaves a Source alone when its row count fits under
// PRISM_RENDER_MAX_MARKS. The in-memory cohort has 0 records, so the
// pass must report no change regardless of the marks ceiling.
func TestPrismSampleInjectionSkipsSmallSource(t *testing.T) {
	src, _ := srcWithFields(t, "sample", []fieldSpec{{"v", "f64"}})
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(src.ID())
	d, _ := b.Build()
	out, changed, err := passes.SampleInjectionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if changed {
		t.Errorf("SampleInjectionPass should skip a 0-row source")
	}
	if out.Size() != d.Size() {
		t.Errorf("size changed: %d vs %d", out.Size(), d.Size())
	}
}

// TestPrismSampleInjectionFiresAboveLimit asserts SampleInjection
// injects a SampleNode below a Source whose row count exceeds the
// runtime PRISM_RENDER_MAX_MARKS ceiling. Backs the Source with an
// inline DataResolver serving 250 rows and lowers the ceiling to 100 so
// the Source crosses the threshold deterministically (Pulse-free, E4).
func TestPrismSampleInjectionFiresAboveLimit(t *testing.T) {
	t.Setenv("PRISM_RENDER_MAX_MARKS", "100")
	rows := make([]map[string]any, 250)
	for i := range rows {
		rows[i] = map[string]any{"v": float64(i)}
	}
	data := resolve.MapDataResolver{"big": {
		Values: rows,
		Fields: []spec.FieldSpec{{Name: "v", Type: "f64"}},
	}}
	src := nodes.New("big", afero.NewMemMapFs(), resolve.NewWithData(nil, data))

	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(src.ID())
	d, _ := b.Build()

	out, changed, err := passes.SampleInjectionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatalf("expected injection (tiny.pulse has 1000 rows; max=100)")
	}
	if out.Size() != d.Size()+1 {
		t.Fatalf("size = %d, want %d (one Sample added)", out.Size(), d.Size()+1)
	}

	// Re-run: pass must be idempotent because the Source already has a
	// SampleNode dependent. No second injection.
	out2, changed2, err := passes.SampleInjectionPass{}.Apply(out)
	if err != nil {
		t.Fatalf("Apply 2: %v", err)
	}
	if changed2 {
		t.Errorf("second Apply re-injected; pass must be idempotent")
	}
	if out2.Size() != out.Size() {
		t.Errorf("size drift on second Apply: %d vs %d", out2.Size(), out.Size())
	}
}

// TestPrismOptimizerPassesTerminate runs the full pass list against a
// stress DAG (one source, many GAs sharing groupby) and asserts the
// fixed-point loop terminates within the optimizer's iteration cap.
func TestPrismOptimizerPassesTerminate(t *testing.T) {
	src, _ := srcWithFields(t, "stress", []fieldSpec{
		{"brand_id", "categorical_u8"},
		{"score", "f64"},
		{"extra", "f64"},
	})
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.MarkRoot(src.ID())
	// 10 sibling GAs all with the same groupby — they should all merge
	// into one over multiple fixpoint iterations.
	for i := 0; i < 10; i++ {
		ga := nodes.NewGroupAggregate(
			plan.NodeID("stress-ga-"+intToStrPasses(i)),
			src.ID(),
			[]string{"brand_id"},
			[]nodes.AggOp{{Op: "mean", Field: "score", As: "m" + intToStrPasses(i)}},
		)
		_ = b.AddNode(ga)
		_ = b.MarkSink(ga.ID())
	}
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	out, err := plan.Optimize(d, plan.DefaultPasses)
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	if out == nil {
		t.Fatal("Optimize returned nil")
	}
}

// intToStrPasses is a tiny stdlib-free itoa for test ids.
func intToStrPasses(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// TestPrismProjectionPruningNoop confirms that when every Source column
// is used downstream, no Project is injected.
func TestPrismProjectionPruningNoop(t *testing.T) {
	src, _ := srcWithFields(t, "proj-noop", []fieldSpec{
		{"brand_id", "categorical_u8"},
		{"score", "f64"},
	})
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.MarkRoot(src.ID())
	ga := nodes.NewGroupAggregate("ga2", src.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "score_mean"}},
	)
	_ = b.AddNode(ga)
	_ = b.MarkSink(ga.ID())
	d, _ := b.Build()

	_, changed, err := passes.ProjectionPruningPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if changed {
		t.Errorf("ProjectionPruning should be no-op when every column is needed")
	}
}
