// Package rpc carries the Twirp service implementation for Prism.
//
// service.proto defines five RPCs (Plot, Validate, Scene, Plan,
// ListDatasets) under package prism.v1. Generated bindings live in
// service.pb.go + service.twirp.go (D083). PrismServer satisfies the
// generated Prism interface and reuses the same pipeline primitives
// the CLI calls: spec.DecodeBytes, plan/build.Build(Composite),
// plan.Execute, encode.Encode(Composite), the format's Renderer.
//
// Selection-state synthesis is NOT part of the Twirp surface; that
// logic lives in the /prism/scene compatibility wrapper (D084)
// because Twirp envelopes do not carry per-call selection state.
//
// Errors returned from every handler flow through ErrorInterceptor
// (interceptor.go / D085), which translates *errors.AppError and
// Pulse *CodedError to Twirp status codes by prefix.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/validatorutil"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/render/svg"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/validate"
	_ "github.com/frankbardon/prism/validate/rules"
)

// PrismServer implements the generated prism.v1.Prism Twirp service.
// Field zero values produce a working server (in-memory backend,
// empty registry, OS file system).
type PrismServer struct {
	// DatasetRegistry exposes the server-side aliases the Twirp
	// ListDatasets RPC enumerates. Optional; the zero value is an
	// EmptyDatasetRegistry.
	DatasetRegistry resolve.DatasetRegistry
	// Fs is the file system every spec read goes through. Optional;
	// the zero value is afero.NewOsFs().
	Fs afero.Fs
	// ExecOpts is merged into the executor options for Plot/Scene
	// requests. Use this to wire optional OTel hooks (D086) via
	// internal/observability.Hooks().
	ExecOpts plan.ExecOpts
}

// fs returns the configured afero.Fs, defaulting to the OS file
// system.
func (s *PrismServer) fs() afero.Fs {
	if s.Fs != nil {
		return s.Fs
	}
	return afero.NewOsFs()
}

// registry returns the configured DatasetRegistry, defaulting to
// EmptyDatasetRegistry.
func (s *PrismServer) registry() resolve.DatasetRegistry {
	if s.DatasetRegistry != nil {
		return s.DatasetRegistry
	}
	return resolve.EmptyDatasetRegistry{}
}

// buildOpts is the per-call options bundle reused by every spec-
// bearing handler. The in-memory compile backend is allocated fresh
// per call so node caches do not leak across requests.
func (s *PrismServer) buildOpts() build.Options {
	return build.Options{
		FS:              s.fs(),
		Resolver:        resolve.New(nil),
		Backend:         inmem.New(),
		DatasetRegistry: s.registry(),
	}
}

// runPipeline executes the standard Plot / Scene pipeline: decode →
// build (composite-aware) → execute → encode. The SceneDoc returned
// is ready for any Renderer.
func (s *PrismServer) runPipeline(
	ctx context.Context,
	specJSON string,
	encOpts encode.EncodeOpts,
) (*scene.SceneDoc, error) {
	if specJSON == "" {
		return nil, prismerrors.New(
			"PRISM_SERVE_DECODE",
			"missing required field: spec",
			map[string]any{"Field": "spec"},
		)
	}
	sp, err := spec.DecodeBytes([]byte(specJSON))
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_SERVE_DECODE",
			"spec decode failed",
			map[string]any{"Reason": err.Error()},
			err,
		)
	}
	if build.IsComposite(sp) {
		composite, berr := build.BuildComposite(sp, s.buildOpts())
		if berr != nil {
			return nil, berr
		}
		per := make([]map[plan.NodeID]*table.Table, len(composite.Children))
		for i, child := range composite.Children {
			res, eerr := plan.Execute(ctx, child.DAG, s.ExecOpts)
			if eerr != nil {
				return nil, eerr
			}
			if len(res.Errors) > 0 {
				return nil, prismerrors.New(
					"PRISM_SERVE_EXECUTE",
					fmt.Sprintf("composite child %d: %d error(s)", i, len(res.Errors)),
					map[string]any{"Child": i, "Count": len(res.Errors), "First": res.Errors[0].Err.Error()},
				)
			}
			per[i] = res.Tables
		}
		doc, eerr := encode.EncodeComposite(sp, composite, per, encOpts)
		if eerr != nil {
			return nil, prismerrors.Wrap(
				"PRISM_SERVE_ENCODE",
				"composite encode failed",
				map[string]any{"Reason": eerr.Error()},
				eerr,
			)
		}
		return doc, nil
	}

	dag, tipID, berr := build.Build(sp, s.buildOpts())
	if berr != nil {
		return nil, berr
	}
	res, eerr := plan.Execute(ctx, dag, s.ExecOpts)
	if eerr != nil {
		return nil, eerr
	}
	if len(res.Errors) > 0 {
		return nil, prismerrors.New(
			"PRISM_SERVE_EXECUTE",
			fmt.Sprintf("%d execution error(s)", len(res.Errors)),
			map[string]any{"Count": len(res.Errors), "First": res.Errors[0].Err.Error()},
		)
	}
	doc, eerr := encode.Encode(sp, res.Tables, tipID, encOpts)
	if eerr != nil {
		return nil, prismerrors.Wrap(
			"PRISM_SERVE_ENCODE",
			"encode failed",
			map[string]any{"Reason": eerr.Error()},
			eerr,
		)
	}
	return doc, nil
}

// dimsOrDefault folds the width/height int32 fields into the encoder
// options struct, defaulting zero values to 800×600.
func dimsOrDefault(w, h int32) (float64, float64) {
	width := float64(w)
	height := float64(h)
	if width <= 0 {
		width = 800
	}
	if height <= 0 {
		height = 600
	}
	return width, height
}

// Plot implements the Plot RPC. SVG renders inline; PNG and PDF
// return PRISM_RENDER_FORMAT_UNAVAILABLE — the interceptor maps that
// code to twirp.Unimplemented per D085.
func (s *PrismServer) Plot(ctx context.Context, req *PlotRequest) (*PlotResponse, error) {
	if req == nil {
		req = &PlotRequest{}
	}
	format := req.Format
	if format == "" {
		format = "svg"
	}
	switch format {
	case "svg":
		// handled below
	case "png", "pdf", "canvas-json":
		// Match the CLI's landing-phase message so users see the
		// same fixup line regardless of surface.
		phase := "P15"
		if format == "canvas-json" {
			phase = "P12"
		}
		if format == "png" {
			phase = "P15"
		}
		return nil, prismerrors.New(
			"PRISM_RENDER_FORMAT_UNAVAILABLE",
			fmt.Sprintf("Render format %s is not available in the current Prism build (lands in %s).", format, phase),
			map[string]any{"Format": format, "Phase": phase},
		)
	default:
		return nil, prismerrors.New(
			"PRISM_RENDER_FORMAT_UNAVAILABLE",
			fmt.Sprintf("Unknown render format %q.", format),
			map[string]any{"Format": format},
		)
	}

	width, height := dimsOrDefault(req.Width, req.Height)
	encOpts := encode.EncodeOpts{
		Width:     width,
		Height:    height,
		ThemeName: req.Theme,
	}
	doc, err := s.runPipeline(ctx, req.Spec, encOpts)
	if err != nil {
		return nil, err
	}
	rend := svg.New()
	bytes, err := rend.Render(doc, render.RenderOpts{
		Format: "svg",
		Width:  width,
		Height: height,
	})
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_RENDER_FAILED",
			"SVG render failed",
			map[string]any{"Reason": err.Error()},
			err,
		)
	}
	warnings := make([]string, 0, len(doc.Warnings))
	for _, w := range doc.Warnings {
		warnings = append(warnings, w.Code+": "+w.Message)
	}
	return &PlotResponse{
		Bytes:    bytes,
		Mime:     rend.MimeType(),
		Warnings: warnings,
	}, nil
}

// Validate implements the Validate RPC. ok=true with no errors when
// the spec is valid; ok=false with the structured error list when
// not. Decode failures are surfaced as a single PRISM_SPEC_009
// entry, matching the CLI behaviour.
func (s *PrismServer) Validate(ctx context.Context, req *ValidateRequest) (*ValidateResponse, error) {
	_ = ctx
	if req == nil {
		req = &ValidateRequest{}
	}
	if req.Spec == "" {
		return nil, prismerrors.New(
			"PRISM_SERVE_DECODE",
			"missing required field: spec",
			map[string]any{"Field": "spec"},
		)
	}
	typedSpec, decodeErr := spec.DecodeBytes([]byte(req.Spec))
	if decodeErr != nil {
		ae := prismerrors.New(
			"PRISM_SPEC_009",
			fmt.Sprintf("Spec failed to decode: %v.", decodeErr),
			map[string]any{"Schema": "(decode failed)"},
		)
		return &ValidateResponse{Ok: false, Errors: []*ValidateError{appErrorToValidateError(ae)}}, nil
	}
	// Shape (JSON Schema) validation.
	var raw any
	if err := json.Unmarshal([]byte(req.Spec), &raw); err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_SERVE_DECODE",
			"re-parse for shape validation failed",
			map[string]any{"Reason": err.Error()},
			err,
		)
	}
	shape, err := validate.NewShapeValidator()
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_SCHEMA_INIT",
			"shape validator init failed",
			map[string]any{"Reason": err.Error()},
			err,
		)
	}
	out := &ValidateResponse{Ok: true}
	for _, se := range shape.Validate(raw) {
		ae := prismerrors.New(
			"PRISM_SPEC_009",
			fmt.Sprintf("Shape violation at %s: %s", se.InstanceLocation, se.Message),
			map[string]any{
				"Schema":           "shape",
				"Instance":         se.InstanceLocation,
				"KeywordViolation": se.KeywordLocation,
				"ViolationMessage": se.Message,
			},
		)
		out.Errors = append(out.Errors, appErrorToValidateError(ae))
		out.Ok = false
	}
	if !out.Ok {
		return out, nil
	}
	// Semantic validation.
	sem := validate.NewDefaultSemanticValidator()
	for _, ae := range sem.Validate(typedSpec, validatorutil.BuildLookup(typedSpec, s.fs())) {
		out.Errors = append(out.Errors, appErrorToValidateError(ae))
		out.Ok = false
	}
	return out, nil
}

func appErrorToValidateError(ae *prismerrors.AppError) *ValidateError {
	if ae == nil {
		return nil
	}
	return &ValidateError{
		Code:    ae.Code,
		Message: ae.Message,
		Fixups:  append([]string(nil), ae.Fixups...),
	}
}

// Scene implements the Scene RPC. The Scene IR is returned as JSON
// text in SceneResponse.scene_json — agents (and the /prism/scene
// compatibility wrapper) parse it back into their own scene type.
func (s *PrismServer) Scene(ctx context.Context, req *SceneRequest) (*SceneResponse, error) {
	if req == nil {
		req = &SceneRequest{}
	}
	width, height := dimsOrDefault(req.Width, req.Height)
	encOpts := encode.EncodeOpts{
		Width:     width,
		Height:    height,
		ThemeName: req.Theme,
	}
	doc, err := s.runPipeline(ctx, req.Spec, encOpts)
	if err != nil {
		return nil, err
	}
	body, mErr := json.Marshal(doc)
	if mErr != nil {
		return nil, prismerrors.Wrap(
			"PRISM_SERVE_ENCODE",
			"scene JSON marshal failed",
			map[string]any{"Reason": mErr.Error()},
			mErr,
		)
	}
	return &SceneResponse{SceneJson: string(body)}, nil
}

// Plan implements the Plan RPC. The DAG is rendered via the standard
// plan.RenderJSON emitter so the wire shape matches `prism plan
// --format json`.
func (s *PrismServer) Plan(ctx context.Context, req *PlanRequest) (*PlanResponse, error) {
	_ = ctx
	if req == nil {
		req = &PlanRequest{}
	}
	if req.Spec == "" {
		return nil, prismerrors.New(
			"PRISM_SERVE_DECODE",
			"missing required field: spec",
			map[string]any{"Field": "spec"},
		)
	}
	sp, err := spec.DecodeBytes([]byte(req.Spec))
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_SERVE_DECODE",
			"spec decode failed",
			map[string]any{"Reason": err.Error()},
			err,
		)
	}
	var buf jsonBuf
	if build.IsComposite(sp) {
		composite, berr := build.BuildComposite(sp, s.buildOpts())
		if berr != nil {
			return nil, berr
		}
		fmt.Fprintf(&buf, `{"kind":%q,"rows":%d,"cols":%d,"children":[`,
			composite.Kind, composite.Rows, composite.Cols)
		for i, child := range composite.Children {
			if i > 0 {
				buf.WriteByte(',')
			}
			if rerr := plan.RenderJSON(child.DAG, &buf); rerr != nil {
				return nil, rerr
			}
		}
		buf.WriteString("]}")
	} else {
		dag, _, berr := build.Build(sp, s.buildOpts())
		if berr != nil {
			return nil, berr
		}
		if rerr := plan.RenderJSON(dag, &buf); rerr != nil {
			return nil, rerr
		}
	}
	return &PlanResponse{PlanJson: buf.String()}, nil
}

// ListDatasets implements the ListDatasets RPC. Registries that
// implement resolve.DatasetLister return their full alias list;
// registries that don't return an empty slice (no error).
func (s *PrismServer) ListDatasets(ctx context.Context, req *DatasetsRequest) (*DatasetsResponse, error) {
	_ = ctx
	_ = req
	reg := s.registry()
	out := &DatasetsResponse{}
	lister, ok := reg.(resolve.DatasetLister)
	if !ok {
		return out, nil
	}
	for _, name := range lister.Names() {
		source, _ := reg.Resolve(name)
		out.Datasets = append(out.Datasets, &Dataset{
			Name:   name,
			Source: source,
		})
	}
	return out, nil
}

// jsonBuf is a tiny io.Writer that backs Plan's RenderJSON call
// without pulling bytes.Buffer's full API into scope here. The
// String() and Write() shapes are all that matters.
type jsonBuf struct{ b []byte }

func (j *jsonBuf) Write(p []byte) (int, error) { j.b = append(j.b, p...); return len(p), nil }
func (j *jsonBuf) WriteByte(c byte) error      { j.b = append(j.b, c); return nil }
func (j *jsonBuf) WriteString(s string) (int, error) {
	j.b = append(j.b, s...)
	return len(s), nil
}
func (j *jsonBuf) String() string { return string(j.b) }
