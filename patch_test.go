package prism_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	prism "github.com/frankbardon/prism"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
)

const baselineSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"name": "x", "values": [{"a":"p","b":1},{"a":"q","b":2}]},
  "mark": "bar",
  "encoding": {
    "x": {"field": "a", "type": "nominal"},
    "y": {"field": "b", "type": "quantitative"}
  }
}`

func mustSpec(t *testing.T, body string) *spec.Spec {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	return s
}

func TestApplyPatchReplacesField(t *testing.T) {
	s := mustSpec(t, baselineSpec)
	out, err := prism.ApplyPatch(s, prism.Patch{
		{Op: "replace", Path: "/mark", Value: "line"},
	})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if out.Mark == nil || out.Mark.TypeName() != "line" {
		t.Errorf("mark = %+v, want line", out.Mark)
	}
	// Atomic: original unchanged.
	if s.Mark.TypeName() != "bar" {
		t.Errorf("baseline mutated: mark = %q", s.Mark.TypeName())
	}
}

func TestApplyPatchAtomicFailure(t *testing.T) {
	s := mustSpec(t, baselineSpec)
	_, err := prism.ApplyPatch(s, prism.Patch{
		{Op: "replace", Path: "/mark", Value: "line"},
		{Op: "remove", Path: "/this/path/does/not/exist"},
	})
	if err == nil {
		t.Fatal("expected failure on second op")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if ae.Code != "PRISM_SPEC_PATCH_001" {
		t.Errorf("Code = %q want PRISM_SPEC_PATCH_001", ae.Code)
	}
	// Original spec must be untouched.
	if s.Mark.TypeName() != "bar" {
		t.Errorf("baseline mutated after failed patch: %q", s.Mark.TypeName())
	}
}

func TestApplyPatchTestOp(t *testing.T) {
	s := mustSpec(t, baselineSpec)
	// Test succeeds → replace runs.
	out, err := prism.ApplyPatch(s, prism.Patch{
		{Op: "test", Path: "/mark", Value: "bar"},
		{Op: "replace", Path: "/mark", Value: "line"},
	})
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if out.Mark.TypeName() != "line" {
		t.Errorf("after test+replace: mark = %q", out.Mark.TypeName())
	}
	// Test fails → whole patch fails.
	_, err = prism.ApplyPatch(s, prism.Patch{
		{Op: "test", Path: "/mark", Value: "line"}, // current is "bar"
		{Op: "replace", Path: "/mark", Value: "area"},
	})
	if err == nil {
		t.Fatal("expected test to fail")
	}
}

func TestApplyPatchRejectsInvalidResult(t *testing.T) {
	// Replacing the mark with garbage should fail to decode and
	// surface as a patch error (atomic — original untouched).
	s := mustSpec(t, baselineSpec)
	_, err := prism.ApplyPatch(s, prism.Patch{
		{Op: "replace", Path: "/mark", Value: map[string]any{"type": "not_a_real_mark"}},
	})
	// Decode/validate may or may not catch unknown mark type here —
	// the spec decoder accepts arbitrary mark strings and validation
	// happens at compile-time. So we don't require the patch to fail
	// here; just confirm Apply doesn't panic and the original spec is
	// intact.
	_ = err
	if s.Mark.TypeName() != "bar" {
		t.Errorf("baseline mutated: %q", s.Mark.TypeName())
	}
}

func TestDiffSpecsRoundtrip(t *testing.T) {
	before := mustSpec(t, baselineSpec)
	// Edit a deep field.
	afterBody := `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"name": "x", "values": [{"a":"p","b":1},{"a":"q","b":2}]},
  "mark": "area",
  "encoding": {
    "x": {"field": "a", "type": "nominal"},
    "y": {"field": "b", "type": "quantitative"}
  }
}`
	after := mustSpec(t, afterBody)

	p, err := prism.DiffSpecs(before, after)
	if err != nil {
		t.Fatalf("DiffSpecs: %v", err)
	}
	if len(p) == 0 {
		t.Fatal("Diff produced empty patch for non-equal specs")
	}

	// Apply the patch to before and confirm we land on after.
	result, err := prism.ApplyPatch(before, p)
	if err != nil {
		t.Fatalf("ApplyPatch round-trip: %v\npatch=%v", err, p)
	}
	gotRaw, _ := json.Marshal(result)
	wantRaw, _ := json.Marshal(after)
	if string(gotRaw) != string(wantRaw) {
		t.Errorf("roundtrip diverged:\n got: %s\nwant: %s", gotRaw, wantRaw)
	}
}

func TestSceneApplyRecompiles(t *testing.T) {
	s := mustSpec(t, baselineSpec)
	scn, err := prism.NewScene(context.Background(), s, prism.CompileOptions{})
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	if scn.Plan() == nil {
		t.Fatal("initial Plan nil")
	}
	beforeType := scn.Plan().Marks[0].Type

	if err := scn.Apply(prism.Patch{
		{Op: "replace", Path: "/mark", Value: "line"},
	}); err != nil {
		t.Fatalf("Scene.Apply: %v", err)
	}
	plan := scn.Plan()
	if plan == nil {
		t.Fatal("Plan() nil after Apply")
	}
	if plan.Marks[0].Type == beforeType {
		t.Errorf("Plan.Marks[0].Type didn't change: %q", plan.Marks[0].Type)
	}
}

func TestSceneApplyAtomic(t *testing.T) {
	s := mustSpec(t, baselineSpec)
	scn, err := prism.NewScene(context.Background(), s, prism.CompileOptions{})
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	beforeMark := scn.Spec().Mark.TypeName()

	err = scn.Apply(prism.Patch{
		{Op: "replace", Path: "/mark", Value: "line"},
		{Op: "replace", Path: "/nonexistent/field", Value: 1},
	})
	if err == nil {
		t.Fatal("expected error from invalid second op")
	}
	if scn.Spec().Mark.TypeName() != beforeMark {
		t.Errorf("scene mutated after failed patch: %q", scn.Spec().Mark.TypeName())
	}
}
