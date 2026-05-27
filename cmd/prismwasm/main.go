//go:build js && wasm

// Command prismwasm is the WebAssembly entry point for the Prism
// visualization library. It compiles to `bin/prism.wasm` and
// exposes the six-stage pipeline (spec → validate → plan →
// compile → encode → render) as `js.Func` methods on the
// `globalThis.prism` object.
//
// JS callers marshal everything across the bridge as JSON strings:
//
//	const sceneJSON = prism.execute(specJSON, datasetsJSON);
//	const svgString = prism.render(sceneJSON, "light");
//	const ok        = prism.validate(specJSON);
//
// Every exported function returns either a string (success) or an
// `{ok:false, error:{Code, Message, Fixups, SeeAlso, Context}}`
// envelope identical to the Twirp/MCP error shape. Boundary
// errors (missing argument, bad JSON) come back with code
// `PRISM_WASM_001`.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	prism "github.com/frankbardon/prism"
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
	"github.com/frankbardon/prism/schema"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/validate"
	_ "github.com/frankbardon/prism/validate/rules"
)

// versionString matches cmd/prism/main.go so JS callers can identify
// the wasm build against the host CLI.
const versionString = "prism v1.0.0"

func main() {
	api := js.Global().Get("Object").New()
	api.Set("version", js.FuncOf(versionFunc))
	api.Set("validate", js.FuncOf(validateFunc))
	api.Set("plan", js.FuncOf(planFunc))
	api.Set("execute", js.FuncOf(executeFunc))
	api.Set("compile", js.FuncOf(compileFunc))
	api.Set("setDataResolver", js.FuncOf(setDataResolverFunc))
	api.Set("applyPatch", js.FuncOf(applyPatchFunc))
	api.Set("diffSpecs", js.FuncOf(diffSpecsFunc))
	api.Set("render", js.FuncOf(renderFunc))
	api.Set("errorsLookup", js.FuncOf(errorsLookupFunc))
	api.Set("schemaBundle", js.FuncOf(schemaBundleFunc))
	api.Set("geo", buildGeoAPI())
	js.Global().Set("prism", api)

	// Signal readiness so the loader can resolve without polling.
	if cb := js.Global().Get("__prismWasmReady"); !cb.IsUndefined() && cb.Type() == js.TypeFunction {
		cb.Invoke()
	}

	// main returning unloads the WASM module; block forever so the
	// exported funcs remain callable for the page lifetime.
	select {}
}

// versionFunc returns the Prism version string verbatim.
func versionFunc(_ js.Value, _ []js.Value) any { return versionString }

// validateFunc shape: prism.validate(specJSON) → JSON string of
// {ok: true} on success, or an error envelope on failure. Shape
// errors and semantic errors both surface as PRISM_SPEC_* codes
// with full fixup context.
func validateFunc(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].IsUndefined() {
		return errEnvelope("PRISM_WASM_001", "validate(specJSON): missing specJSON argument")
	}
	specJSON := args[0].String()
	if err := doValidate([]byte(specJSON)); err != nil {
		return errFromError(err)
	}
	return `{"ok":true}`
}

func doValidate(body []byte) error {
	typed, decodeErr := spec.DecodeBytes(body)
	if decodeErr != nil {
		return prismerrors.New("PRISM_SPEC_009",
			fmt.Sprintf("Spec failed to decode: %v.", decodeErr),
			map[string]any{"Schema": "(decode failed)"})
	}

	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return prismerrors.New("PRISM_SPEC_009",
			fmt.Sprintf("Spec failed to re-parse for shape validation: %v.", err),
			map[string]any{"Schema": "(reparse failed)"})
	}
	shape, err := validate.NewShapeValidator()
	if err != nil {
		return err
	}
	shapeErrs := shape.Validate(raw)
	if len(shapeErrs) > 0 {
		first := shapeErrs[0]
		return prismerrors.New(
			"PRISM_SPEC_009",
			fmt.Sprintf("Shape violation at %s: %s", first.InstanceLocation, first.Message),
			map[string]any{
				"Schema":           "shape",
				"Instance":         first.InstanceLocation,
				"KeywordViolation": first.KeywordLocation,
				"ViolationMessage": first.Message,
			},
		)
	}

	sem := validate.NewDefaultSemanticValidator()
	semErrs := sem.Validate(typed, validatorutil.BuildLookup(typed, resolve.NewFetchFs()))
	if len(semErrs) > 0 {
		return semErrs[0]
	}
	return nil
}

// planFunc shape: prism.plan(specJSON, datasetsJSON?) → JSON of the
// built DAG. JS callers use this for diagnostics (the WASM render
// path internally calls execute, not plan). Composite specs return
// the per-child DAG list under `children`.
func planFunc(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].IsUndefined() {
		return errEnvelope("PRISM_WASM_001", "plan(specJSON, datasetsJSON?): missing specJSON argument")
	}
	specJSON := args[0].String()
	datasetsJSON := ""
	if len(args) >= 2 && args[1].Type() == js.TypeString {
		datasetsJSON = args[1].String()
	}
	out, err := doPlan(specJSON, datasetsJSON)
	if err != nil {
		return errFromError(err)
	}
	return out
}

func doPlan(specJSON, datasetsJSON string) (string, error) {
	s, err := spec.DecodeBytes([]byte(specJSON))
	if err != nil {
		return "", prismerrors.New("PRISM_SPEC_009",
			fmt.Sprintf("Spec failed to decode: %v.", err),
			map[string]any{"Schema": "(decode failed)"})
	}
	opts, err := newBuildOptions(datasetsJSON)
	if err != nil {
		return "", err
	}
	if build.IsComposite(s) {
		composite, err := build.BuildComposite(s, opts)
		if err != nil {
			return "", err
		}
		nodes := make([][]string, 0, len(composite.Children))
		for _, child := range composite.Children {
			ids := make([]string, 0, len(child.DAG.Nodes()))
			for _, id := range child.DAG.Nodes() {
				ids = append(ids, string(id))
			}
			nodes = append(nodes, ids)
		}
		out, mErr := json.Marshal(map[string]any{
			"composite": true,
			"children":  nodes,
		})
		return string(out), mErr
	}
	dag, tip, err := build.Build(s, opts)
	if err != nil {
		return "", err
	}
	ids := make([]string, 0, len(dag.Nodes()))
	for _, id := range dag.Nodes() {
		ids = append(ids, string(id))
	}
	out, mErr := json.Marshal(map[string]any{
		"composite": false,
		"nodes":     ids,
		"tip":       string(tip),
	})
	return string(out), mErr
}

// executeFunc shape: prism.execute(specJSON, datasetsJSON?, optsJSON?)
// → SceneDoc JSON. The browser passes a dataset alias map (alias →
// URL) so the fetch-backed Fs knows where to load each `.pulse`
// reference. optsJSON carries the optional encode knobs
// {width, height, theme}.
func executeFunc(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].IsUndefined() {
		return errEnvelope("PRISM_WASM_001", "execute(specJSON, datasetsJSON?, optsJSON?): missing specJSON argument")
	}
	specJSON := args[0].String()
	datasetsJSON := ""
	if len(args) >= 2 && args[1].Type() == js.TypeString {
		datasetsJSON = args[1].String()
	}
	optsJSON := ""
	if len(args) >= 3 && args[2].Type() == js.TypeString {
		optsJSON = args[2].String()
	}
	out, err := doExecute(specJSON, datasetsJSON, optsJSON)
	if err != nil {
		return errFromError(err)
	}
	return out
}

func doExecute(specJSON, datasetsJSON, optsJSON string) (string, error) {
	s, err := spec.DecodeBytes([]byte(specJSON))
	if err != nil {
		return "", prismerrors.New("PRISM_SPEC_009",
			fmt.Sprintf("Spec failed to decode: %v.", err),
			map[string]any{"Schema": "(decode failed)"})
	}
	buildOpts, err := newBuildOptions(datasetsJSON)
	if err != nil {
		return "", err
	}
	encOpts, err := decodeEncodeOpts(optsJSON)
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	doc, err := executePipeline(ctx, s, buildOpts, encOpts)
	if err != nil {
		return "", err
	}
	out, mErr := json.Marshal(doc)
	return string(out), mErr
}

// executePipeline mirrors plotPipeline in cmd/prism, minus the
// urfave/cli surface. Both flat and composite specs route through
// here; the returned SceneDoc is ready for json.Marshal.
func executePipeline(
	ctx context.Context,
	s *spec.Spec,
	buildOpts build.Options,
	encOpts encode.EncodeOpts,
) (*scene.SceneDoc, error) {
	execOpts := plan.ExecOpts{Workers: 1, AbortOnError: false}
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
	dag, tipID, err := build.Build(s, buildOpts)
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
	return encode.Encode(s, res.Tables, tipID, encOpts)
}

// compileFunc shape: prism.compile(specJSON, datasetsJSON?, optsJSON?)
// → CompiledPlan JSON. Same pipeline as execute, with the result
// flattened into the structured CompiledPlan view (marks / scales /
// data / layout / diagnostics + the canonical scene). The render
// stage is skipped — substantially cheaper than a full prism.execute
// + prism.render pair.
func compileFunc(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].IsUndefined() {
		return errEnvelope("PRISM_WASM_001", "compile(specJSON, datasetsJSON?, optsJSON?): missing specJSON argument")
	}
	specJSON := args[0].String()
	datasetsJSON := ""
	if len(args) >= 2 && args[1].Type() == js.TypeString {
		datasetsJSON = args[1].String()
	}
	optsJSON := ""
	if len(args) >= 3 && args[2].Type() == js.TypeString {
		optsJSON = args[2].String()
	}
	out, err := doCompile(specJSON, datasetsJSON, optsJSON)
	if err != nil {
		return errFromError(err)
	}
	return out
}

func doCompile(specJSON, datasetsJSON, optsJSON string) (string, error) {
	s, err := spec.DecodeBytes([]byte(specJSON))
	if err != nil {
		return "", prismerrors.New("PRISM_SPEC_009",
			fmt.Sprintf("Spec failed to decode: %v.", err),
			map[string]any{"Schema": "(decode failed)"})
	}
	buildOpts, err := newBuildOptions(datasetsJSON)
	if err != nil {
		return "", err
	}
	encOpts, err := decodeEncodeOpts(optsJSON)
	if err != nil {
		return "", err
	}
	cp, err := prism.Compile(context.Background(), s, prism.CompileOptions{
		Build:  buildOpts,
		Exec:   plan.ExecOpts{Workers: 1, AbortOnError: false},
		Encode: encOpts,
	})
	if err != nil {
		return "", err
	}
	body, mErr := json.Marshal(cp)
	return string(body), mErr
}

// applyPatchFunc shape: prism.applyPatch(specJSON, patchJSON) →
// patched-spec JSON, or an error envelope. The patch is RFC 6902 —
// an array of `{op, path, value?, from?}` objects.
//
// Atomic semantics: a failure on any op leaves the input spec
// untouched. The error envelope carries PRISM_SPEC_PATCH_001 with
// the failing op index in details.
func applyPatchFunc(_ js.Value, args []js.Value) any {
	if len(args) < 2 || args[0].IsUndefined() || args[1].IsUndefined() {
		return errEnvelope("PRISM_WASM_001", "applyPatch(specJSON, patchJSON): missing arguments")
	}
	specJSON := args[0].String()
	patchJSON := args[1].String()
	out, err := doApplyPatch(specJSON, patchJSON)
	if err != nil {
		return errFromError(err)
	}
	return out
}

func doApplyPatch(specJSON, patchJSON string) (string, error) {
	s, err := spec.DecodeBytes([]byte(specJSON))
	if err != nil {
		return "", prismerrors.New("PRISM_SPEC_009",
			fmt.Sprintf("Spec failed to decode: %v.", err),
			map[string]any{"Schema": "(decode failed)"})
	}
	var patch prism.Patch
	if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
		return "", prismerrors.New("PRISM_SPEC_PATCH_001",
			fmt.Sprintf("Patch JSON failed to decode: %v.", err),
			map[string]any{"OpIndex": -1})
	}
	next, err := prism.ApplyPatch(s, patch)
	if err != nil {
		return "", err
	}
	body, mErr := json.Marshal(next)
	return string(body), mErr
}

// diffSpecsFunc shape: prism.diffSpecs(beforeJSON, afterJSON) →
// patchJSON. Produces an RFC 6902 patch that transforms before
// into after.
func diffSpecsFunc(_ js.Value, args []js.Value) any {
	if len(args) < 2 || args[0].IsUndefined() || args[1].IsUndefined() {
		return errEnvelope("PRISM_WASM_001", "diffSpecs(beforeJSON, afterJSON): missing arguments")
	}
	beforeJSON := args[0].String()
	afterJSON := args[1].String()
	out, err := doDiffSpecs(beforeJSON, afterJSON)
	if err != nil {
		return errFromError(err)
	}
	return out
}

func doDiffSpecs(beforeJSON, afterJSON string) (string, error) {
	before, err := spec.DecodeBytes([]byte(beforeJSON))
	if err != nil {
		return "", prismerrors.New("PRISM_SPEC_009",
			fmt.Sprintf("before spec failed to decode: %v.", err),
			map[string]any{"Schema": "(decode failed)"})
	}
	after, err := spec.DecodeBytes([]byte(afterJSON))
	if err != nil {
		return "", prismerrors.New("PRISM_SPEC_009",
			fmt.Sprintf("after spec failed to decode: %v.", err),
			map[string]any{"Schema": "(decode failed)"})
	}
	patch, err := prism.DiffSpecs(before, after)
	if err != nil {
		return "", prismerrors.New("PRISM_SPEC_PATCH_001",
			fmt.Sprintf("diff: %v.", err),
			map[string]any{"OpIndex": -1})
	}
	body, mErr := json.Marshal(patch)
	return string(body), mErr
}

// jsDataResolver is the JS-side callback registered via
// prism.setDataResolver. Sync only: the JS function must return the
// Dataset object directly (no Promise). Promise returns are not
// awaitable from synchronous Go-WASM code and surface as an
// unresolved-ref error.
var jsDataResolver js.Value

// setDataResolverFunc shape: prism.setDataResolver(fn) — fn must be a
// synchronous (ref: string) → {values, fields?} | null function.
// Passing null/undefined clears the resolver. Returns the empty string
// on success or an error envelope on bad input.
func setDataResolverFunc(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].IsNull() || args[0].IsUndefined() {
		jsDataResolver = js.Undefined()
		return ""
	}
	fn := args[0]
	if fn.Type() != js.TypeFunction {
		return errEnvelope("PRISM_WASM_001", "setDataResolver(fn): argument must be a function or null")
	}
	jsDataResolver = fn
	return ""
}

// wasmDataResolver implements resolve.DataResolver by invoking the
// installed JS callback. Result-to-Go marshalling goes through
// JSON.stringify so the inline-data shape (values, fields) round-trips
// cleanly. If the JS side returns null/undefined or no callback is
// installed, the ref surfaces as resolve.ErrDataRefUnresolved and the
// build stage emits PRISM_RESOLVE_REF_UNRESOLVED.
type wasmDataResolver struct{}

func (wasmDataResolver) ResolveData(_ context.Context, ref string) (*resolve.Dataset, error) {
	if jsDataResolver.IsUndefined() || jsDataResolver.Type() != js.TypeFunction {
		return nil, resolve.ErrDataRefUnresolved{Ref: ref}
	}
	res := jsDataResolver.Invoke(ref)
	if res.IsNull() || res.IsUndefined() {
		return nil, resolve.ErrDataRefUnresolved{Ref: ref}
	}
	jsonStr := js.Global().Get("JSON").Call("stringify", res).String()
	var raw struct {
		Values []map[string]any `json:"values"`
		Fields []spec.FieldSpec `json:"fields"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, prismerrors.New("PRISM_RESOLVE_REF_UNRESOLVED",
			fmt.Sprintf("Data ref %q resolver callback returned undecodable value: %v.", ref, err),
			map[string]any{"Ref": ref})
	}
	return &resolve.Dataset{Values: raw.Values, Fields: raw.Fields}, nil
}

// renderFunc shape: prism.render(sceneJSON, themeName?) → SVG
// string. The themeName overrides the SceneDoc.Theme name; empty
// or omitted uses the SceneDoc's resolved theme.
func renderFunc(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].IsUndefined() {
		return errEnvelope("PRISM_WASM_001", "render(sceneJSON, themeName?): missing sceneJSON argument")
	}
	sceneJSON := args[0].String()
	themeName := ""
	if len(args) >= 2 && args[1].Type() == js.TypeString {
		themeName = args[1].String()
	}
	out, err := doRender(sceneJSON, themeName)
	if err != nil {
		return errFromError(err)
	}
	return out
}

func doRender(sceneJSON, themeName string) (string, error) {
	var doc scene.SceneDoc
	if err := json.Unmarshal([]byte(sceneJSON), &doc); err != nil {
		return "", prismerrors.New("PRISM_WASM_001",
			fmt.Sprintf("render: invalid SceneDoc JSON: %v.", err),
			map[string]any{"URL": "(inline)", "Status": 0, "Reason": err.Error()})
	}
	body, err := svg.New().Render(&doc, render.RenderOpts{Format: "svg"})
	if err != nil {
		return "", err
	}
	_ = themeName // theme name belongs in the encode stage; render
	// honours whatever the SceneDoc already carries.
	return string(body), nil
}

// errorsLookupFunc shape: prism.errorsLookup(code) → JSON object
// {Code, Message, Fixups, SeeAlso}. Mirrors `prism errors lookup`.
func errorsLookupFunc(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].IsUndefined() {
		return errEnvelope("PRISM_WASM_001", "errorsLookup(code): missing code argument")
	}
	code := args[0].String()
	meta, ok := prismerrors.Codes[code]
	if !ok {
		return errEnvelope("PRISM_WASM_001", fmt.Sprintf("unknown code: %s", code))
	}
	out, err := json.Marshal(meta)
	if err != nil {
		return errEnvelope("PRISM_WASM_001", fmt.Sprintf("marshal code metadata: %v", err))
	}
	return string(out)
}

// schemaBundleFunc shape: prism.schemaBundle() → JSON string of the
// single-file bundle (D087 wire shape).
func schemaBundleFunc(_ js.Value, _ []js.Value) any {
	all, err := schema.V1Schemas()
	if err != nil {
		return errEnvelope("PRISM_WASM_001", fmt.Sprintf("schema bundle: %v", err))
	}
	doc := struct {
		Schema  string                     `json:"$schema"`
		ID      string                     `json:"$id"`
		Version string                     `json:"version"`
		Files   map[string]json.RawMessage `json:"files"`
	}{
		Schema:  "https://json-schema.org/draft/2020-12/schema",
		ID:      "urn:prism:schema:v1:bundle",
		Version: schema.Version,
		Files:   make(map[string]json.RawMessage, len(all)),
	}
	for name, body := range all {
		doc.Files[name+".schema.json"] = json.RawMessage(body)
	}
	out, mErr := json.Marshal(doc)
	if mErr != nil {
		return errEnvelope("PRISM_WASM_001", fmt.Sprintf("marshal schema bundle: %v", mErr))
	}
	return string(out)
}

// newBuildOptions parses the dataset registry JSON (alias → URL) and
// constructs the build.Options struct routed through the fetch-
// backed Fs. Empty datasetsJSON yields an empty registry.
func newBuildOptions(datasetsJSON string) (build.Options, error) {
	reg := resolve.MapDatasetRegistry{}
	if datasetsJSON != "" {
		var raw map[string]string
		if err := json.Unmarshal([]byte(datasetsJSON), &raw); err != nil {
			return build.Options{}, prismerrors.New("PRISM_WASM_001",
				fmt.Sprintf("invalid datasets JSON: %v.", err),
				map[string]any{"URL": "(inline)", "Status": 0, "Reason": err.Error()})
		}
		for k, v := range raw {
			reg[k] = v
		}
	}
	return build.Options{
		FS:              resolve.NewFetchFs(),
		Resolver:        resolve.New(nil),
		Backend:         inmem.New(),
		DatasetRegistry: reg,
		DataResolver:    wasmDataResolver{},
	}, nil
}

// decodeEncodeOpts parses the optional optsJSON ({width, height,
// theme}) into an EncodeOpts. Empty input yields the zero value.
func decodeEncodeOpts(optsJSON string) (encode.EncodeOpts, error) {
	if optsJSON == "" {
		return encode.EncodeOpts{}, nil
	}
	var raw struct {
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
		Theme  string  `json:"theme"`
	}
	if err := json.Unmarshal([]byte(optsJSON), &raw); err != nil {
		return encode.EncodeOpts{}, prismerrors.New("PRISM_WASM_001",
			fmt.Sprintf("invalid encode opts JSON: %v.", err),
			map[string]any{"URL": "(inline)", "Status": 0, "Reason": err.Error()})
	}
	return encode.EncodeOpts{
		Width:     raw.Width,
		Height:    raw.Height,
		ThemeName: raw.Theme,
	}, nil
}

// errEnvelope marshals a synthetic AppError-shaped object as JSON.
// Used for bridge-boundary failures (missing argument, bad JSON).
func errEnvelope(code, message string) string {
	env := struct {
		OK      bool   `json:"ok"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}{OK: false, Code: code, Message: message}
	out, err := json.Marshal(env)
	if err != nil {
		return `{"ok":false,"code":"PRISM_WASM_001","message":"marshal envelope failed"}`
	}
	return string(out)
}

// errFromError wraps any error in a JSON envelope. *AppError
// instances are passed through verbatim so the JS side sees the
// full Code / Message / Fixups / SeeAlso payload.
func errFromError(err error) string {
	if ae, ok := err.(*prismerrors.AppError); ok {
		body, mErr := json.Marshal(struct {
			OK    bool                  `json:"ok"`
			Error *prismerrors.AppError `json:"error"`
		}{OK: false, Error: ae})
		if mErr == nil {
			return string(body)
		}
	}
	return errEnvelope("PRISM_WASM_001", err.Error())
}
