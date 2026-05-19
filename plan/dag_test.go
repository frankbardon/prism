package plan_test

import (
	"context"
	"errors"
	"testing"

	"github.com/frankbardon/pulse/encoding"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// fakeNode is a minimal in-test plan.Node so DAG tests do not depend
// on the nodes package (would be an import cycle).
type fakeNode struct {
	id     plan.NodeID
	inputs []plan.NodeID
}

func (n *fakeNode) ID() plan.NodeID       { return n.id }
func (n *fakeNode) Inputs() []plan.NodeID { return n.inputs }
func (n *fakeNode) Fingerprint() string   { return "fake:" + string(n.id) }
func (n *fakeNode) Schema(_ []*encoding.Schema) (*encoding.Schema, error) {
	return &encoding.Schema{Fields: []encoding.Field{{Name: "x", Type: encoding.FieldTypeF64}}}, nil
}
func (n *fakeNode) Execute(_ context.Context, _ []*table.Table) (*table.Table, error) {
	return nil, nil
}

func mkNode(id string, inputs ...string) *fakeNode {
	ins := make([]plan.NodeID, len(inputs))
	for i, s := range inputs {
		ins[i] = plan.NodeID(s)
	}
	return &fakeNode{id: plan.NodeID(id), inputs: ins}
}

func TestPrismBuilderHappyPath(t *testing.T) {
	b := plan.NewBuilder()
	if err := b.AddNode(mkNode("src")); err != nil {
		t.Fatalf("AddNode src: %v", err)
	}
	if err := b.AddNode(mkNode("f", "src")); err != nil {
		t.Fatalf("AddNode f: %v", err)
	}
	if err := b.MarkRoot("src"); err != nil {
		t.Fatalf("MarkRoot: %v", err)
	}
	if err := b.MarkSink("f"); err != nil {
		t.Fatalf("MarkSink: %v", err)
	}
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.Size() != 2 {
		t.Errorf("Size=%d, want 2", d.Size())
	}
	if len(d.Roots()) != 1 || d.Roots()[0] != "src" {
		t.Errorf("Roots=%v", d.Roots())
	}
	if len(d.Sinks()) != 1 || d.Sinks()[0] != "f" {
		t.Errorf("Sinks=%v", d.Sinks())
	}
	deps := d.Dependents("src")
	if len(deps) != 1 || deps[0] != "f" {
		t.Errorf("Dependents(src)=%v", deps)
	}
}

func TestPrismBuilderDuplicateID(t *testing.T) {
	b := plan.NewBuilder()
	_ = b.AddNode(mkNode("x"))
	if err := b.AddNode(mkNode("x")); err == nil {
		t.Fatal("expected duplicate id error, got nil")
	}
}

func TestPrismBuilderMissingInput(t *testing.T) {
	b := plan.NewBuilder()
	_ = b.AddNode(mkNode("f", "missing"))
	_ = b.MarkSink("f")
	_, err := b.Build()
	if err == nil {
		t.Fatal("expected missing-input error, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_PLAN_003" {
		t.Errorf("expected PRISM_PLAN_003, got %v", err)
	}
}

func TestPrismBuilderNoSink(t *testing.T) {
	b := plan.NewBuilder()
	_ = b.AddNode(mkNode("src"))
	_ = b.MarkRoot("src")
	_, err := b.Build()
	if err == nil {
		t.Fatal("expected no-sink error, got nil")
	}
}

func TestPrismDAGWithNodeStructuralSharing(t *testing.T) {
	b := plan.NewBuilder()
	a := mkNode("a")
	c := mkNode("c", "a")
	_ = b.AddNode(a)
	_ = b.AddNode(c)
	_ = b.MarkSink("c")
	d, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Replace c; a should remain pointer-equal in the new DAG.
	d2 := d.WithNode(mkNode("c", "a"))
	old, _ := d.Node("a")
	new, _ := d2.Node("a")
	if old != new {
		t.Errorf("structural sharing failed: a pointer changed across WithNode")
	}
	if d.Size() != d2.Size() {
		t.Errorf("size changed: %d vs %d", d.Size(), d2.Size())
	}
}

func TestPrismDAGWithoutNode(t *testing.T) {
	b := plan.NewBuilder()
	_ = b.AddNode(mkNode("a"))
	_ = b.AddNode(mkNode("b", "a"))
	_ = b.MarkRoot("a")
	_ = b.MarkSink("b")
	d, _ := b.Build()
	d2 := d.WithoutNode("a")
	if _, ok := d2.Node("a"); ok {
		t.Errorf("WithoutNode did not remove a")
	}
	if len(d2.Roots()) != 0 {
		t.Errorf("Roots after WithoutNode(a) = %v, want empty", d2.Roots())
	}
}
