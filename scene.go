package prism

import (
	"context"
	"fmt"

	"github.com/frankbardon/prism/spec"
)

// Scene is a stateful wrapper around a Prism spec + its last
// CompiledPlan. Use it when the consumer wants to evolve a chart
// incrementally via patches: each Apply mutates the in-memory spec
// and re-compiles, exposing the new plan via Plan().
//
// Scene is not safe for concurrent use; serialise Apply calls
// externally if multiple goroutines drive a single scene.
type Scene struct {
	ctx  context.Context
	spec *spec.Spec
	opts CompileOptions
	plan *CompiledPlan
}

// NewScene compiles an initial spec and returns a Scene bound to it.
// The returned Scene can be patched via Apply / ApplyAndRender.
func NewScene(ctx context.Context, s *spec.Spec, opts CompileOptions) (*Scene, error) {
	if s == nil {
		return nil, fmt.Errorf("prism.NewScene: nil spec")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	plan, err := Compile(ctx, s, opts)
	if err != nil {
		return nil, err
	}
	return &Scene{
		ctx:  ctx,
		spec: s,
		opts: opts,
		plan: plan,
	}, nil
}

// Spec returns the scene's current *spec.Spec. The returned value is
// shared with the scene; callers should not mutate it directly —
// patches must travel through Apply / ApplyAndRender so the
// compiled plan stays in sync.
func (s *Scene) Spec() *spec.Spec { return s.spec }

// Plan returns the most recent CompiledPlan.
func (s *Scene) Plan() *CompiledPlan { return s.plan }

// Apply transforms the scene's spec by the given RFC 6902 patch and
// re-compiles. The mutation is atomic: if any operation fails, or
// the resulting spec fails to decode / re-compile, the scene's
// state is unchanged and the error envelope explains why.
func (s *Scene) Apply(p Patch) error {
	next, err := ApplyPatch(s.spec, p)
	if err != nil {
		return err
	}
	plan, err := Compile(s.ctx, next, s.opts)
	if err != nil {
		return err
	}
	s.spec = next
	s.plan = plan
	return nil
}

// ApplyAndRender applies the patch and returns the updated plan in
// one call. It is shorthand for Apply followed by Plan(). The
// "render" half of the name follows the upgrade-spec API; the
// returned object is the structured CompiledPlan — callers wanting
// pixels should hand it to a Renderer.
func (s *Scene) ApplyAndRender(p Patch) (*CompiledPlan, error) {
	if err := s.Apply(p); err != nil {
		return nil, err
	}
	return s.plan, nil
}
