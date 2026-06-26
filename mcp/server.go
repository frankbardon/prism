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

// plotResult is the structured payload returned by prism_plot.
type plotResult struct {
	Bytes    string   `json:"bytes"` // base64-encoded
	Mime     string   `json:"mime"`
	Caption  string   `json:"caption"`
	Warnings []string `json:"warnings,omitempty"`
}

func plotHandler(impl *rpc.PrismServer) gosdk.ToolHandler {
	return func(ctx context.Context, req *gosdk.CallToolRequest) (*gosdk.CallToolResult, error) {
		args := toolArgs(req)
		spec, _ := args["spec"].(string)
		format, _ := args["format"].(string)
		if spec == "" {
			return errorResult("missing required argument: spec"), nil
		}
		if format == "" {
			format = "svg"
		}

		// The Twirp Plot handler enforces the format switch + runs
		// the full pipeline; we reuse it to keep one source of
		// truth for "what 'svg' means".
		resp, err := impl.Plot(ctx, &rpc.PlotRequest{
			Spec: spec, Format: format,
		})
		if err != nil {
			return errorResult(err.Error()), nil
		}

		// Caption from the parsed spec (mark + encoding fields).
		caption := summariseSpec(spec)

		result := plotResult{
			Bytes:    base64.StdEncoding.EncodeToString(resp.Bytes),
			Mime:     resp.Mime,
			Caption:  caption,
			Warnings: append([]string(nil), resp.Warnings...),
		}
		body, _ := json.Marshal(result)
		return textResult(string(body)), nil
	}
}

func validateHandler(impl *rpc.PrismServer) gosdk.ToolHandler {
	return func(ctx context.Context, req *gosdk.CallToolRequest) (*gosdk.CallToolResult, error) {
		args := toolArgs(req)
		spec, _ := args["spec"].(string)
		if spec == "" {
			return errorResult("missing required argument: spec"), nil
		}
		resp, err := impl.Validate(ctx, &rpc.ValidateRequest{Spec: spec})
		if err != nil {
			return errorResult(err.Error()), nil
		}
		// Hand-marshal a tidy shape: {ok, errors:[{code,message,fixups}]}.
		errs := make([]map[string]any, 0, len(resp.Errors))
		for _, e := range resp.Errors {
			errs = append(errs, map[string]any{
				"code":    e.Code,
				"message": e.Message,
				"fixups":  e.Fixups,
			})
		}
		body, _ := json.Marshal(map[string]any{
			"ok":     resp.Ok,
			"errors": errs,
		})
		return textResult(string(body)), nil
	}
}

func describeHandler() gosdk.ToolHandler {
	return func(ctx context.Context, req *gosdk.CallToolRequest) (*gosdk.CallToolResult, error) {
		args := toolArgs(req)
		spec, _ := args["spec"].(string)
		if spec == "" {
			return errorResult("missing required argument: spec"), nil
		}
		summary := summariseSpec(spec)
		body, _ := json.Marshal(map[string]any{"summary": summary})
		return textResult(string(body)), nil
	}
}

// exampleResult is one entry returned by prism_examples_search.
type exampleResult struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Spec    string `json:"spec"`
}

func examplesSearchHandler(opts Options) gosdk.ToolHandler {
	return func(ctx context.Context, req *gosdk.CallToolRequest) (*gosdk.CallToolResult, error) {
		args := toolArgs(req)
		query, _ := args["query"].(string)
		if query == "" {
			return errorResult("missing required argument: query"), nil
		}
		var hits []exampleResult
		if opts.ExamplesRoot == "" {
			hits = searchEmbedded(query, 5)
		} else {
			hits = searchExamples(opts.ExamplesFS, opts.ExamplesRoot, query, 5)
		}
		body, _ := json.Marshal(map[string]any{"examples": hits})
		return textResult(string(body)), nil
	}
}

// searchEmbedded serves prism_examples_search from the embedded examples
// corpus. It mirrors examples.Search but layers the richer summariseSpec
// fallback for specs that carry no title: examples.Search falls back to the
// bare stem to stay stdlib-pure, whereas the MCP tool historically produced a
// spec-aware summary, so we preserve that user-visible output here.
func searchEmbedded(query string, limit int) []exampleResult {
	results := examples.Search(query, limit)
	out := make([]exampleResult, 0, len(results))
	for _, r := range results {
		summary := r.Summary
		if extractTitle([]byte(r.Spec)) == "" {
			if richer := summariseSpec(r.Spec); richer != "" {
				summary = richer
			}
		}
		out = append(out, exampleResult{
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
func searchExamples(fsys afero.Fs, root, query string, limit int) []exampleResult {
	if fsys == nil {
		fsys = afero.NewOsFs()
	}
	q := strings.ToLower(query)
	var out []exampleResult
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
		out = append(out, exampleResult{
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
