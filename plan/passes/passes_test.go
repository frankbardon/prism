package passes_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/frankbardon/pulse/encoding"
	"github.com/spf13/afero"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/plan/passes"
	"github.com/frankbardon/prism/resolve"
)

// repoRootForPasses returns the absolute repo root, derived from this
// test file's location so the tiny.pulse fixture resolves regardless
// of `go test ./...` cwd.
func repoRootForPasses(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(here), "..", "..")
}

// memFS produces a memory-backed afero with a stub .pulse file at
// /a.pulse. For unit tests we only need SourceNode.OutputSchema() to
// succeed; the bytes themselves are never read.
func memFSWithSchema(t *testing.T, name string, schema *encoding.Schema) afero.Fs {
	t.Helper()
	// Build a minimal Pulse cohort: header + schema + 0 records.
	fs := afero.NewMemMapFs()
	f, err := fs.Create(name)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	if err := encoding.WriteHeader(f); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if err := encoding.WriteSchema(f, schema); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	return fs
}

func srcWithSchema(t *testing.T, ref string, schema *encoding.Schema) (*nodes.SourceNode, afero.Fs) {
	t.Helper()
	fs := memFSWithSchema(t, ref, schema)
	return nodes.New(ref, fs, resolve.New(nil)), fs
}

func TestPrismDedupSourcesNoop(t *testing.T) {
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "v", Type: encoding.FieldTypeF64},
	}}
	src, _ := srcWithSchema(t, "/a.pulse", schema)
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
	leftSchema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "score", Type: encoding.FieldTypeF64},
	}}
	rightSchema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "label", Type: encoding.FieldTypeCategoricalU8},
	}}
	left, _ := srcWithSchema(t, "/left.pulse", leftSchema)
	right, _ := srcWithSchema(t, "/right.pulse", rightSchema)

	b := plan.NewBuilder()
	_ = b.AddNode(left)
	_ = b.AddNode(right)
	_ = b.MarkRoot(left.ID())
	_ = b.MarkRoot(right.ID())
	join := nodes.NewJoin("j1", left.ID(), right.ID(), []string{"brand_id"}, nodes.JoinInner, 0)
	_ = b.AddNode(join)
	// Filter references `score` — exclusively in the left schema.
	filt := nodes.NewFilter("f1", join.ID(), "score > 0.5")
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
	leftSchema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "score", Type: encoding.FieldTypeF64},
	}}
	rightSchema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "label", Type: encoding.FieldTypeCategoricalU8},
	}}
	left, _ := srcWithSchema(t, "/leftR.pulse", leftSchema)
	right, _ := srcWithSchema(t, "/rightR.pulse", rightSchema)
	b := plan.NewBuilder()
	_ = b.AddNode(left)
	_ = b.AddNode(right)
	_ = b.MarkRoot(left.ID())
	_ = b.MarkRoot(right.ID())
	join := nodes.NewJoin("j2", left.ID(), right.ID(), []string{"brand_id"}, nodes.JoinInner, 0)
	_ = b.AddNode(join)
	filt := nodes.NewFilter("f2", join.ID(), "label == 'alpha'")
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
	leftSchema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "score", Type: encoding.FieldTypeF64},
	}}
	rightSchema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "label", Type: encoding.FieldTypeCategoricalU8},
	}}
	left, _ := srcWithSchema(t, "/leftM.pulse", leftSchema)
	right, _ := srcWithSchema(t, "/rightM.pulse", rightSchema)
	b := plan.NewBuilder()
	_ = b.AddNode(left)
	_ = b.AddNode(right)
	_ = b.MarkRoot(left.ID())
	_ = b.MarkRoot(right.ID())
	join := nodes.NewJoin("j3", left.ID(), right.ID(), []string{"brand_id"}, nodes.JoinInner, 0)
	_ = b.AddNode(join)
	// Filter references columns from both sides.
	filt := nodes.NewFilter("f3", join.ID(), "score > 0.5 and label != ''")
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
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "score", Type: encoding.FieldTypeF64},
		{Name: "age", Type: encoding.FieldTypeU8},
		{Name: "region", Type: encoding.FieldTypeCategoricalU8},
	}}
	src, _ := srcWithSchema(t, "/proj.pulse", schema)
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
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "score", Type: encoding.FieldTypeF64},
	}}
	src, _ := srcWithSchema(t, "/agg-fuse.pulse", schema)
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
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "region", Type: encoding.FieldTypeCategoricalU8},
		{Name: "score", Type: encoding.FieldTypeF64},
	}}
	src, _ := srcWithSchema(t, "/agg-noop.pulse", schema)
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
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "v", Type: encoding.FieldTypeF64},
	}}
	src, _ := srcWithSchema(t, "/sample.pulse", schema)
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
// runtime PRISM_RENDER_MAX_MARKS ceiling. Uses the committed 1000-row
// tiny.pulse fixture with the env var lowered to 100 so the Source
// crosses the threshold deterministically.
func TestPrismSampleInjectionFiresAboveLimit(t *testing.T) {
	t.Setenv("PRISM_RENDER_MAX_MARKS", "100")
	root := repoRootForPasses(t)
	cohort := filepath.Join(root, "testdata", "cohorts", "tiny.pulse")
	src := nodes.New(cohort, afero.NewOsFs(), resolve.New(nil))

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
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "score", Type: encoding.FieldTypeF64},
		{Name: "extra", Type: encoding.FieldTypeF64},
	}}
	src, _ := srcWithSchema(t, "/stress.pulse", schema)
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
	schema := &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU8},
		{Name: "score", Type: encoding.FieldTypeF64},
	}}
	src, _ := srcWithSchema(t, "/proj-noop.pulse", schema)
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
