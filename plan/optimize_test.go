package plan_test

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/passes"
)

func tinyDAG(t *testing.T) *plan.DAG {
	t.Helper()
	b := plan.NewBuilder()
	_ = b.AddNode(mkNode("src"))
	_ = b.AddNode(mkNode("filter", "src"))
	_ = b.MarkRoot("src")
	_ = b.MarkSink("filter")
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return d
}

func TestPrismOptimizeTerminates(t *testing.T) {
	d := tinyDAG(t)
	out, err := plan.Optimize(d, nil)
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	if out != d {
		t.Errorf("Optimize with empty passes should return input pointer")
	}
}

func TestPrismOptimizeNoop(t *testing.T) {
	d := tinyDAG(t)
	out, err := plan.Optimize(d, []plan.Pass{passes.NoopPass{}})
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	if out != d {
		t.Errorf("Optimize with NoopPass should return input pointer")
	}
}

// togglingPass alternates a boolean per call. Two of them together
// flip-flop forever IF the loop ignored the changed flag — but our
// implementation halts when no pass mutates, so the loop must exit
// because NoopPass does not change anything.
type togglingPass struct {
	calls *int
}

func (p togglingPass) Name() string { return "toggle" }
func (p togglingPass) Apply(d *plan.DAG) (*plan.DAG, bool, error) {
	*p.calls++
	return d, false, nil
}

func TestPrismOptimizeFixedPoint(t *testing.T) {
	d := tinyDAG(t)
	calls := 0
	_, err := plan.Optimize(d, []plan.Pass{togglingPass{calls: &calls}})
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 call, got %d", calls)
	}
}

// runawayPass always reports changed=true, exercising the iteration
// safety net.
type runawayPass struct{}

func (runawayPass) Name() string { return "runaway" }
func (runawayPass) Apply(d *plan.DAG) (*plan.DAG, bool, error) {
	return d, true, nil
}

func TestPrismOptimizeIterationCap(t *testing.T) {
	d := tinyDAG(t)
	_, err := plan.Optimize(d, []plan.Pass{runawayPass{}})
	if err == nil {
		t.Fatal("expected iteration-cap error, got nil")
	}
	if !strings.Contains(err.Error(), "fixed point") {
		t.Errorf("expected fixed-point error, got %v", err)
	}
}

func TestPrismDefaultPassesEmptyInP03(t *testing.T) {
	if len(plan.DefaultPasses) != 0 {
		t.Errorf("DefaultPasses should be empty in P03; got %d passes", len(plan.DefaultPasses))
	}
}
