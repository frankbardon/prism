package plan_test

import (
	"errors"
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
)

func TestPrismDAGCycleDetection(t *testing.T) {
	// a -> b -> a (a tight cycle). The regular Builder.Build path
	// would reject these because a depends on b which has not been
	// declared yet — but we want to verify the cycle detection runs
	// in TopoLevels too, since optimizer passes could in theory
	// produce a cyclic intermediate. Use the test-only Unchecked
	// helpers to skip Build's validation.
	b := plan.NewBuilder()
	b.AddNodeUnchecked(mkNode("a", "b"))
	b.AddNodeUnchecked(mkNode("b", "a"))
	d := b.BuildUnchecked()

	_, err := d.TopoLevels()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AppError, got %T", err)
	}
	if ae.Code != "PRISM_PLAN_001" {
		t.Errorf("Code=%q, want PRISM_PLAN_001", ae.Code)
	}
	if got := ae.Context["Nodes"]; got != 2 {
		t.Errorf("Nodes context=%v, want 2", got)
	}
}

func TestPrismDAGSelfCycle(t *testing.T) {
	// Self-loop: a -> a.
	b := plan.NewBuilder()
	b.AddNodeUnchecked(mkNode("a", "a"))
	d := b.BuildUnchecked()
	_, err := d.TopoLevels()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}
