package passes_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/frankbardon/pulse/encoding"
	"github.com/spf13/afero"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/plan/passes"
	"github.com/frankbardon/prism/resolve"
)

// tinySchema matches the committed testdata/cohorts/tiny.pulse:
// brand_id (categorical), score (f64), age (f64).
func tinySchema() *encoding.Schema {
	return &encoding.Schema{Fields: []encoding.Field{
		{Name: "brand_id", Type: encoding.FieldTypeCategoricalU32, Dictionary: encoding.NewDictionary()},
		{Name: "score", Type: encoding.FieldTypeF64},
		{Name: "age", Type: encoding.FieldTypeF64},
	}}
}

// pulseChainFusedRoot returns the single PulseChainNode in d.Roots(),
// failing the test if zero or more-than-one are present. Tests use
// this to assert the fusion outcome without depending on derived ids.
func pulseChainFusedRoot(t *testing.T, d *plan.DAG) *nodes.PulseChainNode {
	t.Helper()
	var found *nodes.PulseChainNode
	for _, id := range d.Roots() {
		n, ok := d.Node(id)
		if !ok {
			continue
		}
		ch, ok := n.(*nodes.PulseChainNode)
		if !ok {
			continue
		}
		if found != nil {
			t.Fatalf("expected exactly one PulseChainNode root; found at least two (%s, %s)", found.ID(), ch.ID())
		}
		found = ch
	}
	if found == nil {
		t.Fatal("no PulseChainNode in DAG roots after fusion")
	}
	return found
}

func TestPrismPulseChainFusionFiresOnAggChain(t *testing.T) {
	schema := tinySchema()
	src, _ := srcWithSchema(t, "/a.pulse", schema)
	agg := nodes.NewGroupAggregate("agg1", src.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "score_mean"}},
	)

	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.AddNode(agg)
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(agg.ID())
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	out, changed, err := passes.PulseChainFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("expected fusion to fire on Source→GroupAggregate")
	}
	ch := pulseChainFusedRoot(t, out)

	// The chain node must own both stages (source + agg).
	stages := ch.StageIDs()
	if len(stages) != 1 || stages[0] != agg.ID() {
		t.Fatalf("StageIDs = %v, want [agg1]", stages)
	}
	// The fused chain must be the new sink (the original agg was the sink).
	sinks := out.Sinks()
	if len(sinks) != 1 || sinks[0] != ch.ID() {
		t.Fatalf("Sinks = %v, want [%s]", sinks, ch.ID())
	}
	// Source + agg are gone; only the chain node remains.
	if out.Size() != 1 {
		t.Fatalf("DAG size = %d, want 1", out.Size())
	}

	// ChainRequest must carry one Group + one Aggregation.
	req := ch.ChainRequest()
	if len(req.Stages) != 1 {
		t.Fatalf("Stages = %d, want 1", len(req.Stages))
	}
	stage := req.Stages[0].Request
	if len(stage.Groups) != 1 || stage.Groups[0].Field != "brand_id" {
		t.Fatalf("Groups = %v, want [brand_id]", stage.Groups)
	}
	if len(stage.Aggregations) != 1 || stage.Aggregations[0].Field != "score" || stage.Aggregations[0].Label != "score_mean" {
		t.Fatalf("Aggregations = %v", stage.Aggregations)
	}
}

func TestPrismPulseChainFusionAbsorbsFilterCalcSort(t *testing.T) {
	schema := tinySchema()
	src, _ := srcWithSchema(t, "/a.pulse", schema)
	fil := nodes.NewFilter("fil1", src.ID(), "score > 0")
	calc := nodes.NewCalculate("calc1", fil.ID(), "score * 2", "score2")
	agg := nodes.NewGroupAggregate("agg1", calc.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "sum", Field: "score2", As: "score2_sum"}},
	)
	srt := nodes.NewSort("srt1", agg.ID(), []nodes.SortKey{{Field: "score2_sum", Order: "desc"}})

	b := plan.NewBuilder()
	for _, n := range []plan.Node{src, fil, calc, agg, srt} {
		_ = b.AddNode(n)
	}
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(srt.ID())
	d, _ := b.Build()

	out, changed, err := passes.PulseChainFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("expected fusion on filter+calc+agg+sort chain")
	}
	ch := pulseChainFusedRoot(t, out)
	if len(ch.StageIDs()) != 4 {
		t.Fatalf("StageIDs = %v, want 4 nodes absorbed", ch.StageIDs())
	}
	req := ch.ChainRequest().Stages[0].Request
	if len(req.Filterers) != 1 || req.Filterers[0].Expression != "score > 0" {
		t.Fatalf("Filterers = %v", req.Filterers)
	}
	if len(req.Attributes) != 1 || req.Attributes[0].Label != "score2" {
		t.Fatalf("Attributes = %v", req.Attributes)
	}
	if len(req.Sort) != 1 || req.Sort[0].Field != "score2_sum" || !req.Sort[0].Desc {
		t.Fatalf("Sort = %v", req.Sort)
	}
}

func TestPrismPulseChainFusionSkipsBareSource(t *testing.T) {
	schema := tinySchema()
	src, _ := srcWithSchema(t, "/a.pulse", schema)
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(src.ID())
	d, _ := b.Build()

	_, changed, err := passes.PulseChainFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if changed {
		t.Fatal("fusion should skip a bare source (no GroupAggregate, no win)")
	}
}

func TestPrismPulseChainFusionSkipsFilterOnly(t *testing.T) {
	schema := tinySchema()
	src, _ := srcWithSchema(t, "/a.pulse", schema)
	fil := nodes.NewFilter("fil1", src.ID(), "score > 0")
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.AddNode(fil)
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(fil.ID())
	d, _ := b.Build()

	_, changed, err := passes.PulseChainFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if changed {
		t.Fatal("fusion should require at least one GroupAggregate (win condition)")
	}
}

func TestPrismPulseChainFusionSkipsBranchingSource(t *testing.T) {
	schema := tinySchema()
	src, _ := srcWithSchema(t, "/a.pulse", schema)
	agg1 := nodes.NewGroupAggregate("agg1", src.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "score_mean"}},
	)
	agg2 := nodes.NewGroupAggregate("agg2", src.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "sum", Field: "score", As: "score_sum"}},
	)
	b := plan.NewBuilder()
	for _, n := range []plan.Node{src, agg1, agg2} {
		_ = b.AddNode(n)
	}
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(agg1.ID())
	_ = b.MarkSink(agg2.ID())
	d, _ := b.Build()

	_, changed, err := passes.PulseChainFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if changed {
		t.Fatal("fusion must skip when source has multiple dependents (branching)")
	}
}

func TestPrismPulseChainFusionRejectsModeAlias(t *testing.T) {
	schema := tinySchema()
	src, _ := srcWithSchema(t, "/a.pulse", schema)
	agg := nodes.NewGroupAggregate("agg1", src.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mode", Field: "score", As: "score_mode"}},
	)
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.AddNode(agg)
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(agg.ID())
	d, _ := b.Build()

	_, changed, err := passes.PulseChainFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if changed {
		t.Fatal("fusion must reject `mode` (Pulse chain gate excludes AGG_MODE)")
	}
}

func TestPrismPulseChainFusionRejectsDeferredAlias(t *testing.T) {
	schema := tinySchema()
	src, _ := srcWithSchema(t, "/a.pulse", schema)
	agg := nodes.NewGroupAggregate("agg1", src.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "lift", Field: "score", As: "score_lift"}},
	)
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.AddNode(agg)
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(agg.ID())
	d, _ := b.Build()

	_, changed, err := passes.PulseChainFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if changed {
		t.Fatal("fusion must reject `lift` (deferred from Pulse — client-side only)")
	}
}

func TestPrismPulseChainFusionSkipsCohortRef(t *testing.T) {
	schema := tinySchema()
	src, _ := srcWithSchema(t, "/a.pulse", schema)
	// Replace the source ref with a cohort:<id> form by constructing a
	// new SourceNode against the same fs but a non-eligible ref.
	cohortSrc := nodes.New("cohort:tiny", afero.NewMemMapFs(), resolve.New(nil))
	agg := nodes.NewGroupAggregate("agg1", cohortSrc.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "score_mean"}},
	)
	_ = src // silence unused
	b := plan.NewBuilder()
	_ = b.AddNode(cohortSrc)
	_ = b.AddNode(agg)
	_ = b.MarkRoot(cohortSrc.ID())
	_ = b.MarkSink(agg.ID())
	d, _ := b.Build()

	_, changed, err := passes.PulseChainFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if changed {
		t.Fatal("fusion must skip cohort:<id> refs in v1")
	}
}

// TestPrismPulseChainExecuteParity executes a fused chain against the
// committed tiny.pulse fixture and asserts the per-brand mean(score)
// matches the in-memory backend's result for the same plan shape.
func TestPrismPulseChainExecuteParity(t *testing.T) {
	root := repoRootForPasses(t)
	cohort := filepath.Join(root, "testdata", "cohorts", "tiny.pulse")
	fs := afero.NewOsFs()
	src := nodes.New(cohort, fs, resolve.New(nil))
	agg := nodes.NewGroupAggregate("agg1", src.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "score_mean"}},
	)

	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.AddNode(agg)
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(agg.ID())
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	out, changed, err := passes.PulseChainFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("expected fusion to fire")
	}
	ch := pulseChainFusedRoot(t, out)
	tbl, err := ch.Execute(t.Context(), nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if tbl.NumRows() != 4 {
		t.Fatalf("rows = %d, want 4 (brand groups)", tbl.NumRows())
	}
	// Field order matches ChainOutputSchema: groupby first, then aggs.
	wantFields := []string{"brand_id", "score_mean"}
	got := tbl.FieldNames()
	if len(got) != len(wantFields) {
		t.Fatalf("FieldNames = %v, want %v", got, wantFields)
	}
	for i, w := range wantFields {
		if got[i] != w {
			t.Fatalf("FieldNames[%d] = %q, want %q", i, got[i], w)
		}
	}
	// All four brands present.
	brandCol, _ := tbl.Column("brand_id")
	seen := map[string]bool{}
	for i := 0; i < tbl.NumRows(); i++ {
		seen[brandCol.ValueAt(i).(string)] = true
	}
	for _, want := range []string{"alpha", "beta", "gamma", "delta"} {
		if !seen[want] {
			t.Errorf("brand %q missing from chain output", want)
		}
	}
}

// TestPrismPulseChainFingerprintStable ensures two equivalent fusions
// share an id so the table cache can hit across runs.
func TestPrismPulseChainFingerprintStable(t *testing.T) {
	schema := tinySchema()
	a, _ := srcWithSchema(t, "/a.pulse", schema)
	b1, _ := srcWithSchema(t, "/a.pulse", schema)
	agg := nodes.NewGroupAggregate("agg1", a.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "v"}},
	)
	agg2 := nodes.NewGroupAggregate("agg1", b1.ID(),
		[]string{"brand_id"},
		[]nodes.AggOp{{Op: "mean", Field: "score", As: "v"}},
	)
	first := buildAndFuse(t, a, agg)
	second := buildAndFuse(t, b1, agg2)
	if first.ID() != second.ID() {
		t.Fatalf("equivalent fusions have different ids: %s vs %s", first.ID(), second.ID())
	}
	if first.Fingerprint() != second.Fingerprint() {
		t.Fatalf("fingerprints differ for equivalent fusions: %s vs %s", first.Fingerprint(), second.Fingerprint())
	}
	if !strings.HasPrefix(string(first.ID()), "pulse_chain:") {
		t.Errorf("chain id should be prefixed `pulse_chain:`, got %s", first.ID())
	}
}

func buildAndFuse(t *testing.T, src *nodes.SourceNode, agg *nodes.GroupAggregateNode) *nodes.PulseChainNode {
	t.Helper()
	b := plan.NewBuilder()
	_ = b.AddNode(src)
	_ = b.AddNode(agg)
	_ = b.MarkRoot(src.ID())
	_ = b.MarkSink(agg.ID())
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	out, changed, err := passes.PulseChainFusionPass{}.Apply(d)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("fusion did not fire")
	}
	return pulseChainFusedRoot(t, out)
}
