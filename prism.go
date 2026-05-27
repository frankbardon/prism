// Package prism is the public Go entry point for the Prism
// visualization library. The full pipeline lives in subpackages
// (spec, validate, plan, plan/build, encode, render, …); this root
// package exposes the two highest-level operations callers reach for:
//
//   - Compile — run the pipeline up to but not including the
//     rasterising render stage. Returns a renderer-agnostic
//     CompiledPlan describing what would be drawn. Typically 10-50×
//     cheaper than a full Render (the encode + raster stages are the
//     expensive ones).
//
//   - Render — full pipeline → byte stream (SVG today; PDF gated by
//     build tag).
//
// Both helpers accept either a parsed *spec.Spec or raw JSON bytes
// and surface diagnostics (PRISM_WARN_* warnings) alongside any
// hard-error envelope.
package prism

import (
	"context"
	"fmt"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// CompileOptions controls the compile pipeline. Build, Exec, and
// Encode are passed through to the corresponding stages. When zero,
// sensible defaults are filled in (in-memory backend, single-worker
// executor, 800×600 layout).
type CompileOptions struct {
	Build  build.Options
	Exec   plan.ExecOpts
	Encode encode.EncodeOpts
}

// CompiledPlan is the structured output of Compile: the same
// information the renderer would consume, exposed as a stable JSON
// shape. Callers can inspect the plan, diff two plans against each
// other, or hand it to a renderer separately via RenderPlan.
//
// The Scene field carries the canonical IR; the flattened views
// (Marks, Scales, Data, Layout) summarise it for programmatic
// inspection so callers don't have to traverse the full tree.
type CompiledPlan struct {
	Scene       *scene.SceneDoc `json:"scene"`
	Marks       []CompiledMark  `json:"marks"`
	Scales      []CompiledScale `json:"scales"`
	Data        []DataBinding   `json:"data"`
	Layout      LayoutInfo      `json:"layout"`
	Diagnostics []Diagnostic    `json:"diagnostics"`
}

// CompiledMark summarises one layer's worth of marks. MarkIndex is the
// layer's index within its enclosing scene; InstanceCount is the
// number of individual mark geometries (one per data row in the
// general case).
type CompiledMark struct {
	SceneID       string         `json:"scene_id"`
	LayerID       string         `json:"layer_id"`
	MarkIndex     int            `json:"mark_index"`
	Type          scene.MarkType `json:"type"`
	InstanceCount int            `json:"instance_count"`
	Source        string         `json:"source,omitempty"`
}

// CompiledScale is the post-resolve scale descriptor for one channel.
// Domain values are typed `any` because categorical scales carry
// strings while quantitative scales carry numbers / times.
type CompiledScale struct {
	SceneID string          `json:"scene_id"`
	Channel scene.Channel   `json:"channel"`
	Type    scene.ScaleType `json:"type"`
	Domain  []any           `json:"domain"`
	Range   [2]float64      `json:"range"`
}

// DataBinding is one dataset reference resolved during compile.
// Resolved is always true when present in the CompiledPlan (compile
// failed otherwise); the field is retained for forward-compatibility
// with the data-reference variant which can produce data-pending
// states.
type DataBinding struct {
	Name     string `json:"name"`
	Hash     string `json:"hash,omitempty"`
	Resolved bool   `json:"resolved"`
}

// LayoutInfo carries the resolved chart-frame dimensions and the
// composition grid shape (rows/cols).
type LayoutInfo struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Rows   int     `json:"rows"`
	Cols   int     `json:"cols"`
}

// Diagnostic mirrors a SceneDoc warning. Code is a PRISM_WARN_* (or
// any future PRISM_* code emitted during compile); Path is the
// layer / channel context when known.
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

// Compile runs Validate → Plan → Execute → Encode and returns a
// CompiledPlan. The result is renderer-agnostic; pass it to
// RenderPlan to produce pixel bytes. Typical cost is dominated by
// the executor (data I/O + aggregation); the marshalled CompiledPlan
// itself is light. Callers that want only the plan summary can
// discard CompiledPlan.Scene to drop the full IR.
func Compile(ctx context.Context, s *spec.Spec, opts CompileOptions) (*CompiledPlan, error) {
	if s == nil {
		return nil, fmt.Errorf("prism.Compile: nil spec")
	}
	buildOpts := opts.Build
	if buildOpts.Backend == nil {
		buildOpts.Backend = inmem.New()
	}
	if buildOpts.Resolver == nil {
		buildOpts.Resolver = resolve.New(nil)
	}

	doc, err := runPipeline(ctx, s, buildOpts, opts.Exec, opts.Encode)
	if err != nil {
		return nil, err
	}
	return derivePlan(doc, s), nil
}

// CompileJSON decodes specJSON and delegates to Compile.
func CompileJSON(ctx context.Context, specJSON []byte, opts CompileOptions) (*CompiledPlan, error) {
	s, err := spec.DecodeBytes(specJSON)
	if err != nil {
		return nil, prismerrors.New("PRISM_SPEC_009",
			fmt.Sprintf("Spec failed to decode: %v.", err),
			map[string]any{"Schema": "(decode failed)"})
	}
	return Compile(ctx, s, opts)
}

// runPipeline mirrors cmd/prismwasm's executePipeline so both the
// CLI/Go path and the WASM path share one implementation here.
func runPipeline(
	ctx context.Context,
	s *spec.Spec,
	buildOpts build.Options,
	execOpts plan.ExecOpts,
	encOpts encode.EncodeOpts,
) (*scene.SceneDoc, error) {
	if build.IsComposite(s) {
		composite, err := build.BuildComposite(s, buildOpts)
		if err != nil {
			return nil, err
		}
		per := make([]map[plan.NodeID]*table.Table, len(composite.Children))
		for i, child := range composite.Children {
			res, err := plan.Execute(ctx, child.DAG, execOpts)
			if err != nil {
				return nil, err
			}
			if len(res.Errors) > 0 {
				return nil, res.Errors[0].Err
			}
			per[i] = res.Tables
		}
		return encode.EncodeComposite(s, composite, per, encOpts)
	}
	dag, tip, err := build.Build(s, buildOpts)
	if err != nil {
		return nil, err
	}
	res, err := plan.Execute(ctx, dag, execOpts)
	if err != nil {
		return nil, err
	}
	if len(res.Errors) > 0 {
		return nil, res.Errors[0].Err
	}
	return encode.Encode(s, res.Tables, tip, encOpts)
}

// derivePlan walks a SceneDoc once to populate the flattened
// CompiledPlan views. Order is deterministic: cells in grid order,
// layers in their declared order, axes in their declared order. The
// spec is consulted to recover named dataset bindings that don't
// surface on the SceneLayer.Source field (inline data, name-only
// references).
func derivePlan(doc *scene.SceneDoc, s *spec.Spec) *CompiledPlan {
	plan := &CompiledPlan{Scene: doc}
	if doc == nil {
		return plan
	}

	// Layout — primary frame dimensions come from the first cell's
	// outer Frame (post-layout). For empty grids fall back to zeros.
	if len(doc.Grid.Cells) > 0 {
		frame := doc.Grid.Cells[0].Scene.Frame
		plan.Layout.Width = frame.W
		plan.Layout.Height = frame.H
	}
	plan.Layout.Rows = doc.Grid.Layout.Rows
	plan.Layout.Cols = doc.Grid.Layout.Cols

	// Marks + scales, walked per cell.
	scaleSeen := make(map[string]struct{})
	for _, cell := range doc.Grid.Cells {
		sc := cell.Scene
		for i, layer := range sc.Layers {
			plan.Marks = append(plan.Marks, CompiledMark{
				SceneID:       sc.ID,
				LayerID:       layer.ID,
				MarkIndex:     i,
				Type:          layer.Mark,
				InstanceCount: len(layer.Marks),
				Source:        layer.Source,
			})
		}
		for _, axis := range sc.Axes {
			key := sc.ID + "|" + string(axis.Channel)
			if _, ok := scaleSeen[key]; ok {
				continue
			}
			scaleSeen[key] = struct{}{}
			plan.Scales = append(plan.Scales, CompiledScale{
				SceneID: sc.ID,
				Channel: axis.Channel,
				Type:    axis.Scale.Type,
				Domain:  append([]any(nil), axis.Scale.Domain...),
				Range:   axis.Scale.Range,
			})
		}
	}

	// Data bindings — distinct list: SceneDoc.Datasets (post-resolve
	// hashes) → spec.Datasets (named cohorts) → spec.Data root → layer
	// Sources observed in the scene. Each binding appears once.
	seenData := make(map[string]struct{})
	addBinding := func(name, hash string) {
		if name == "" {
			return
		}
		if _, ok := seenData[name]; ok {
			return
		}
		seenData[name] = struct{}{}
		plan.Data = append(plan.Data, DataBinding{
			Name:     name,
			Hash:     hash,
			Resolved: true,
		})
	}
	for _, ds := range doc.Datasets {
		addBinding(ds.Name, ds.Hash)
	}
	collectSpecData(s, addBinding)
	for _, cell := range doc.Grid.Cells {
		for _, layer := range cell.Scene.Layers {
			addBinding(layer.Source, "")
		}
	}

	// Diagnostics from SceneDoc warnings.
	for _, w := range doc.Warnings {
		plan.Diagnostics = append(plan.Diagnostics, Diagnostic{
			Code:    w.Code,
			Message: w.Message,
			Path:    w.Layer,
		})
	}

	if plan.Marks == nil {
		plan.Marks = []CompiledMark{}
	}
	if plan.Scales == nil {
		plan.Scales = []CompiledScale{}
	}
	if plan.Data == nil {
		plan.Data = []DataBinding{}
	}
	if plan.Diagnostics == nil {
		plan.Diagnostics = []Diagnostic{}
	}
	return plan
}

// collectSpecData walks the spec tree once and emits one (name,
// hash="") tuple per named data binding. Names come from the
// top-level data, the Datasets map, and every composite child's data.
func collectSpecData(s *spec.Spec, emit func(name, hash string)) {
	if s == nil {
		return
	}
	for name := range s.Datasets {
		emit(name, "")
	}
	if s.Data != nil {
		switch {
		case s.Data.Name != "":
			emit(s.Data.Name, "")
		case s.Data.Source != "":
			emit(s.Data.Source, "")
		}
	}
	for _, child := range s.Layer {
		collectSpecData(child, emit)
	}
	for _, child := range s.Concat {
		collectSpecData(child, emit)
	}
	for _, child := range s.HConcat {
		collectSpecData(child, emit)
	}
	for _, child := range s.VConcat {
		collectSpecData(child, emit)
	}
	if s.ChildSpec != nil {
		collectSpecData(s.ChildSpec, emit)
	}
}
