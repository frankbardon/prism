//go:build !js

// Package mcp wires Prism into the Model Context Protocol via
// modelcontextprotocol/go-sdk (pinned at v1.6.1).
//
// New(opts) returns a configured *gosdk.Server with four tools
// registered:
//
//   - prism_plot(spec, format?)         → bytes + mime + caption
//   - prism_validate(spec)              → ok + structured errors
//   - prism_describe(spec)              → natural-language summary
//   - prism_examples_search(query)      → list of fixture specs
//
// The server is transport-agnostic; the CLI's `prism mcp`
// subcommand drives it over a stdio transport for agent-host use.
package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	gosdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/afero"

	"github.com/frankbardon/prism/examples"
	"github.com/frankbardon/prism/rpc"
	prismspec "github.com/frankbardon/prism/spec"
)

// Options configures a new MCP server instance. ExamplesRoot selects the
// data source for the prism_examples_search tool: empty (the default)
// serves results from the embedded examples corpus; a non-empty value
// opts into an on-disk afero walk of that directory.
type Options struct {
	PrismServer  *rpc.PrismServer
	ExamplesRoot string
	// ExamplesFS is the file system the on-disk examples walk uses when
	// ExamplesRoot is set. Defaults to afero.NewOsFs(). Tests inject an
	// afero.MemMapFs. Ignored when ExamplesRoot is empty (embedded corpus).
	ExamplesFS afero.Fs
}

// New constructs an MCP server with all four Prism tools registered.
// The returned *gosdk.Server is ready to drive via a stdio transport
// (for the `prism mcp` CLI subcommand) or any other transport the
// go-sdk supports.
func New(opts Options) *gosdk.Server {
	if opts.PrismServer == nil {
		opts.PrismServer = &rpc.PrismServer{Fs: afero.NewOsFs()}
	}
	// An empty ExamplesRoot serves the embedded corpus; a non-empty root
	// opts into an on-disk walk, defaulting the filesystem to the OS.
	if opts.ExamplesRoot != "" && opts.ExamplesFS == nil {
		opts.ExamplesFS = afero.NewOsFs()
	}

	s := gosdk.NewServer(&gosdk.Implementation{Name: "prism", Version: "0.1.0"}, nil)
	registerTools(s, opts)
	return s
}

// registerTools attaches the four Prism tools to the supplied server.
// Public so tests can build a bare server and register selectively.
//
// We use the low-level gosdk.Server.AddTool with hand-written input
// schemas so the tool result is the raw JSON text content the agent
// host expects (the typed gosdk.AddTool[In,Out] form would wrap the
// payload in structured content instead).
func registerTools(s *gosdk.Server, opts Options) {
	s.AddTool(
		&gosdk.Tool{
			Name:        "prism_plot",
			Description: "Compile a Prism spec and render to image bytes. SVG (default) and PDF are supported; PNG returns PRISM_RENDER_FORMAT_UNAVAILABLE (V2).",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"spec":{"type":"string","description":"Prism spec as a JSON string (matches the schemas under schema/v1/)."},"format":{"type":"string","description":"Output format: svg (default) | pdf. PNG returns PRISM_RENDER_FORMAT_UNAVAILABLE."}},"required":["spec"]}`),
		},
		plotHandler(opts.PrismServer),
	)
	s.AddTool(
		&gosdk.Tool{
			Name:        "prism_validate",
			Description: "Validate a Prism spec against the embedded JSON Schema + semantic rules.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"spec":{"type":"string","description":"Prism spec as a JSON string."}},"required":["spec"]}`),
		},
		validateHandler(opts.PrismServer),
	)
	s.AddTool(
		&gosdk.Tool{
			Name:        "prism_describe",
			Description: "Return a natural-language summary of what a Prism spec renders.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"spec":{"type":"string","description":"Prism spec as a JSON string."}},"required":["spec"]}`),
		},
		describeHandler(),
	)
	s.AddTool(
		&gosdk.Tool{
			Name:        "prism_examples_search",
			Description: "Search the curated example spec library by substring match on name + title. Returns up to 5 results.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Case-insensitive substring to match against fixture names + titles."}},"required":["query"]}`),
		},
		examplesSearchHandler(opts),
	)
}

// toolArgs decodes the raw tool-call arguments into a string-keyed map,
// mirroring the mark3labs CallToolRequest.GetArguments() shape the
// handler bodies were written against.
func toolArgs(req *gosdk.CallToolRequest) map[string]any {
	out := map[string]any{}
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return out
	}
	_ = json.Unmarshal(req.Params.Arguments, &out)
	return out
}

// textResult wraps a JSON payload string as a successful tool result.
func textResult(body string) *gosdk.CallToolResult {
	return &gosdk.CallToolResult{
		Content: []gosdk.Content{&gosdk.TextContent{Text: body}},
	}
}

// errorResult surfaces a facade/argument error as a tool-result error
// (IsError=true) rather than an MCP protocol-level Go error, so the
// agent can see the message and self-correct.
func errorResult(msg string) *gosdk.CallToolResult {
	return &gosdk.CallToolResult{
		IsError: true,
		Content: []gosdk.Content{&gosdk.TextContent{Text: msg}},
	}
}

// PlotInput is the typed input for the prism_plot tool.
type PlotInput struct {
	Spec   string `json:"spec"`
	Format string `json:"format,omitempty"`
}

// PlotOutput is the structured payload returned by prism_plot.
type PlotOutput struct {
	Bytes    string   `json:"bytes"` // base64-encoded
	Mime     string   `json:"mime"`
	Caption  string   `json:"caption"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateInput is the typed input for the prism_validate tool.
type ValidateInput struct {
	Spec string `json:"spec"`
}

// ValidateError is one validation diagnostic in a ValidateOutput. Field order
// mirrors the historical map-marshalled shape (code, fixups, message).
type ValidateError struct {
	Code    string   `json:"code"`
	Fixups  []string `json:"fixups"`
	Message string   `json:"message"`
}

// ValidateOutput is the structured payload returned by prism_validate. Field
// order mirrors the historical map-marshalled shape (errors, ok).
type ValidateOutput struct {
	Errors []ValidateError `json:"errors"`
	Ok     bool            `json:"ok"`
}

// DescribeInput is the typed input for the prism_describe tool.
type DescribeInput struct {
	Spec string `json:"spec"`
}

// DescribeOutput is the structured payload returned by prism_describe.
type DescribeOutput struct {
	Summary string `json:"summary"`
}

// ExampleResult is one entry returned by prism_examples_search.
type ExampleResult struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Spec    string `json:"spec"`
}

// ExamplesSearchInput is the typed input for the prism_examples_search tool.
// Query is the wire-facing field; Root + FS carry the server's examples-source
// configuration (empty Root → embedded corpus, non-empty → on-disk walk) and
// are not part of the tool's JSON schema.
type ExamplesSearchInput struct {
	Query string   `json:"query"`
	Root  string   `json:"-"`
	FS    afero.Fs `json:"-"`
}

// ExamplesSearchOutput is the structured payload returned by
// prism_examples_search.
type ExamplesSearchOutput struct {
	Examples []ExampleResult `json:"examples"`
}

// missingArg builds the argument-error returned when a required tool argument
// is absent. The SDK wiring surfaces it as a tool-result error so the agent
// can self-correct.
func missingArg(name string) error {
	return errors.New("missing required argument: " + name)
}

// PlotTool compiles a Prism spec and renders it to image bytes via the rpc
// facade. It returns the facade's coded errors verbatim.
func PlotTool(ctx context.Context, f *rpc.PrismServer, in PlotInput) (PlotOutput, error) {
	if in.Spec == "" {
		return PlotOutput{}, missingArg("spec")
	}
	format := in.Format
	if format == "" {
		format = "svg"
	}

	// The Twirp Plot handler enforces the format switch + runs the full
	// pipeline; we reuse it to keep one source of truth for "what 'svg' means".
	resp, err := f.Plot(ctx, &rpc.PlotRequest{Spec: in.Spec, Format: format})
	if err != nil {
		return PlotOutput{}, err
	}
	return PlotOutput{
		Bytes:    base64.StdEncoding.EncodeToString(resp.Bytes),
		Mime:     resp.Mime,
		Caption:  summariseSpec(in.Spec), // caption from the parsed spec
		Warnings: append([]string(nil), resp.Warnings...),
	}, nil
}

// ValidateTool validates a Prism spec via the rpc facade, returning the
// facade's coded errors verbatim.
func ValidateTool(ctx context.Context, f *rpc.PrismServer, in ValidateInput) (ValidateOutput, error) {
	if in.Spec == "" {
		return ValidateOutput{}, missingArg("spec")
	}
	resp, err := f.Validate(ctx, &rpc.ValidateRequest{Spec: in.Spec})
	if err != nil {
		return ValidateOutput{}, err
	}
	errs := make([]ValidateError, 0, len(resp.Errors))
	for _, e := range resp.Errors {
		errs = append(errs, ValidateError{
			Code:    e.Code,
			Fixups:  e.Fixups,
			Message: e.Message,
		})
	}
	return ValidateOutput{Errors: errs, Ok: resp.Ok}, nil
}

// DescribeTool returns a natural-language summary of a Prism spec. The facade
// argument is accepted for signature uniformity and is unused.
func DescribeTool(_ context.Context, _ *rpc.PrismServer, in DescribeInput) (DescribeOutput, error) {
	if in.Spec == "" {
		return DescribeOutput{}, missingArg("spec")
	}
	return DescribeOutput{Summary: summariseSpec(in.Spec)}, nil
}

// ExamplesSearchTool searches the example spec library. An empty in.Root serves
// the embedded corpus; a non-empty Root opts into an on-disk afero walk of that
// directory. The facade argument is accepted for signature uniformity.
func ExamplesSearchTool(_ context.Context, _ *rpc.PrismServer, in ExamplesSearchInput) (ExamplesSearchOutput, error) {
	if in.Query == "" {
		return ExamplesSearchOutput{}, missingArg("query")
	}
	var hits []ExampleResult
	if in.Root == "" {
		hits = searchEmbedded(in.Query, 5)
	} else {
		hits = searchExamples(in.FS, in.Root, in.Query, 5)
	}
	return ExamplesSearchOutput{Examples: hits}, nil
}

func plotHandler(impl *rpc.PrismServer) gosdk.ToolHandler {
	return func(ctx context.Context, req *gosdk.CallToolRequest) (*gosdk.CallToolResult, error) {
		args := toolArgs(req)
		spec, _ := args["spec"].(string)
		format, _ := args["format"].(string)
		out, err := PlotTool(ctx, impl, PlotInput{Spec: spec, Format: format})
		if err != nil {
			return errorResult(err.Error()), nil
		}
		body, _ := json.Marshal(out)
		return textResult(string(body)), nil
	}
}

func validateHandler(impl *rpc.PrismServer) gosdk.ToolHandler {
	return func(ctx context.Context, req *gosdk.CallToolRequest) (*gosdk.CallToolResult, error) {
		args := toolArgs(req)
		spec, _ := args["spec"].(string)
		out, err := ValidateTool(ctx, impl, ValidateInput{Spec: spec})
		if err != nil {
			return errorResult(err.Error()), nil
		}
		body, _ := json.Marshal(out)
		return textResult(string(body)), nil
	}
}

func describeHandler() gosdk.ToolHandler {
	return func(ctx context.Context, req *gosdk.CallToolRequest) (*gosdk.CallToolResult, error) {
		args := toolArgs(req)
		spec, _ := args["spec"].(string)
		out, err := DescribeTool(ctx, nil, DescribeInput{Spec: spec})
		if err != nil {
			return errorResult(err.Error()), nil
		}
		body, _ := json.Marshal(out)
		return textResult(string(body)), nil
	}
}

func examplesSearchHandler(opts Options) gosdk.ToolHandler {
	return func(ctx context.Context, req *gosdk.CallToolRequest) (*gosdk.CallToolResult, error) {
		args := toolArgs(req)
		query, _ := args["query"].(string)
		out, err := ExamplesSearchTool(ctx, opts.PrismServer, ExamplesSearchInput{
			Query: query,
			Root:  opts.ExamplesRoot,
			FS:    opts.ExamplesFS,
		})
		if err != nil {
			return errorResult(err.Error()), nil
		}
		body, _ := json.Marshal(out)
		return textResult(string(body)), nil
	}
}

// searchEmbedded serves prism_examples_search from the embedded examples
// corpus. It mirrors examples.Search but layers the richer summariseSpec
// fallback for specs that carry no title: examples.Search falls back to the
// bare stem to stay stdlib-pure, whereas the MCP tool historically produced a
// spec-aware summary, so we preserve that user-visible output here.
func searchEmbedded(query string, limit int) []ExampleResult {
	results := examples.Search(query, limit)
	out := make([]ExampleResult, 0, len(results))
	for _, r := range results {
		summary := r.Summary
		if extractTitle([]byte(r.Spec)) == "" {
			if richer := summariseSpec(r.Spec); richer != "" {
				summary = richer
			}
		}
		out = append(out, ExampleResult{
			Name:    r.Name,
			Summary: summary,
			Spec:    r.Spec,
		})
	}
	return out
}

// searchExamples walks the examples root and returns up to limit
// results whose filename stem (or title) contains query case-
// insensitively. Each result carries the raw spec JSON for direct
// invocation.
func searchExamples(fsys afero.Fs, root, query string, limit int) []ExampleResult {
	if fsys == nil {
		fsys = afero.NewOsFs()
	}
	q := strings.ToLower(query)
	var out []ExampleResult
	_ = afero.Walk(fsys, root, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".json") {
			return nil
		}
		// Skip the "invalid/" subdirectory — those are designed to fail.
		if strings.Contains(path, string(filepath.Separator)+"invalid"+string(filepath.Separator)) {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(path), ".json")
		body, err := afero.ReadFile(fsys, path)
		if err != nil {
			return nil
		}
		title := extractTitle(body)
		hay := strings.ToLower(base + " " + title)
		if !strings.Contains(hay, q) {
			return nil
		}
		summary := title
		if summary == "" {
			summary = summariseSpec(string(body))
		}
		out = append(out, ExampleResult{
			Name:    base,
			Summary: summary,
			Spec:    string(body),
		})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// extractTitle picks the spec.title field without forcing a full
// decode (the spec may not pass strict decoding even for examples).
func extractTitle(body []byte) string {
	var doc struct {
		Title string `json:"title"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return ""
	}
	return doc.Title
}

// summariseSpec returns a one-line English description of the
// rendered chart. Deterministic and side-effect-free; safe for the
// describe tool. Returns "" when the spec fails decoding.
func summariseSpec(specJSON string) string {
	s, err := prismspec.DecodeBytes([]byte(specJSON))
	if err != nil {
		return ""
	}
	mark := strings.ToLower(s.Mark.TypeName())
	if mark == "" {
		mark = "chart"
	}
	var parts []string
	parts = append(parts, mark+" chart")
	if s.Encoding != nil {
		var enc []string
		if s.Encoding.X != nil && s.Encoding.X.Field != "" {
			enc = append(enc, "x="+s.Encoding.X.Field)
		}
		if s.Encoding.Y != nil && s.Encoding.Y.Field != "" {
			enc = append(enc, "y="+s.Encoding.Y.Field)
		}
		if s.Encoding.Color != nil && s.Encoding.Color.Field != "" {
			enc = append(enc, "color="+s.Encoding.Color.Field)
		}
		if len(enc) > 0 {
			parts = append(parts, "with "+strings.Join(enc, ", "))
		}
	}
	if len(s.Transform) > 0 {
		parts = append(parts, fmt.Sprintf("(%d transform%s)", len(s.Transform), pluralS(len(s.Transform))))
	}
	if title := titleString(s.Title); title != "" {
		parts = append([]string{title + ":"}, parts...)
	}
	return strings.Join(parts, " ")
}

// titleString extracts the text portion of a spec title regardless of
// whether it was provided as a bare string or rich object.
func titleString(t *prismspec.TextOrTextObj) string {
	if t == nil {
		return ""
	}
	if t.Text != nil {
		return *t.Text
	}
	if t.Obj != nil {
		return t.Obj.Text
	}
	return ""
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
