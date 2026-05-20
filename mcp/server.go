// Package mcp wires Prism into the Model Context Protocol via
// mark3labs/mcp-go (D008 parity with Pulse, pinned at v0.54.0).
//
// New(opts) returns a configured *server.MCPServer with four tools
// registered:
//
//   - prism_plot(spec, format?)         → bytes + mime + caption
//   - prism_validate(spec)              → ok + structured errors
//   - prism_describe(spec)              → natural-language summary
//   - prism_examples_search(query)      → list of fixture specs
//
// The server is transport-agnostic; the CLI's `prism mcp`
// subcommand wraps it in server.ServeStdio for agent-host use.
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

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/afero"

	"github.com/frankbardon/prism/rpc"
	prismspec "github.com/frankbardon/prism/spec"
)

// Options configures a new MCP server instance. ExamplesRoot is the
// directory the prism_examples_search tool searches; defaults to
// "testdata/specs/".
type Options struct {
	PrismServer  *rpc.PrismServer
	ExamplesRoot string
	// ExamplesFS is the file system the examples search walks.
	// Defaults to afero.NewOsFs(). Tests inject an afero.MemMapFs.
	ExamplesFS afero.Fs
}

// New constructs an MCP server with all four Prism tools registered.
// The returned *server.MCPServer is ready to drive via either
// server.ServeStdio (for the `prism mcp` CLI subcommand) or any
// other transport mcp-go supports.
func New(opts Options) *server.MCPServer {
	if opts.PrismServer == nil {
		opts.PrismServer = &rpc.PrismServer{Fs: afero.NewOsFs()}
	}
	if opts.ExamplesRoot == "" {
		opts.ExamplesRoot = "testdata/specs/"
	}
	if opts.ExamplesFS == nil {
		opts.ExamplesFS = afero.NewOsFs()
	}

	s := server.NewMCPServer("prism", "0.1.0")
	registerTools(s, opts)
	return s
}

// registerTools attaches the four Prism tools to the supplied server.
// Public so tests can build a bare server and register selectively.
func registerTools(s *server.MCPServer, opts Options) {
	s.AddTool(
		mcpgo.NewTool("prism_plot",
			mcpgo.WithDescription("Compile a Prism spec and render to image bytes. SVG (default) and PDF are supported; PNG returns PRISM_RENDER_FORMAT_UNAVAILABLE (V2)."),
			mcpgo.WithString("spec",
				mcpgo.Required(),
				mcpgo.Description("Prism spec as a JSON string (matches the schemas under schema/v1/).")),
			mcpgo.WithString("format",
				mcpgo.Description("Output format: svg (default) | pdf. PNG returns PRISM_RENDER_FORMAT_UNAVAILABLE.")),
		),
		plotHandler(opts.PrismServer),
	)
	s.AddTool(
		mcpgo.NewTool("prism_validate",
			mcpgo.WithDescription("Validate a Prism spec against the embedded JSON Schema + semantic rules."),
			mcpgo.WithString("spec",
				mcpgo.Required(),
				mcpgo.Description("Prism spec as a JSON string.")),
		),
		validateHandler(opts.PrismServer),
	)
	s.AddTool(
		mcpgo.NewTool("prism_describe",
			mcpgo.WithDescription("Return a natural-language summary of what a Prism spec renders."),
			mcpgo.WithString("spec",
				mcpgo.Required(),
				mcpgo.Description("Prism spec as a JSON string.")),
		),
		describeHandler(),
	)
	s.AddTool(
		mcpgo.NewTool("prism_examples_search",
			mcpgo.WithDescription("Search the curated example spec library by substring match on name + title. Returns up to 5 results."),
			mcpgo.WithString("query",
				mcpgo.Required(),
				mcpgo.Description("Case-insensitive substring to match against fixture names + titles.")),
		),
		examplesSearchHandler(opts),
	)
}

// plotResult is the structured payload returned by prism_plot.
type plotResult struct {
	Bytes    string   `json:"bytes"` // base64-encoded
	Mime     string   `json:"mime"`
	Caption  string   `json:"caption"`
	Warnings []string `json:"warnings,omitempty"`
}

func plotHandler(impl *rpc.PrismServer) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		spec, _ := args["spec"].(string)
		format, _ := args["format"].(string)
		if spec == "" {
			return mcpgo.NewToolResultError("missing required argument: spec"), nil
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
			return mcpgo.NewToolResultError(err.Error()), nil
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
		return mcpgo.NewToolResultText(string(body)), nil
	}
}

func validateHandler(impl *rpc.PrismServer) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		spec, _ := args["spec"].(string)
		if spec == "" {
			return mcpgo.NewToolResultError("missing required argument: spec"), nil
		}
		resp, err := impl.Validate(ctx, &rpc.ValidateRequest{Spec: spec})
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
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
		return mcpgo.NewToolResultText(string(body)), nil
	}
}

func describeHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		spec, _ := args["spec"].(string)
		if spec == "" {
			return mcpgo.NewToolResultError("missing required argument: spec"), nil
		}
		summary := summariseSpec(spec)
		body, _ := json.Marshal(map[string]any{"summary": summary})
		return mcpgo.NewToolResultText(string(body)), nil
	}
}

// exampleResult is one entry returned by prism_examples_search.
type exampleResult struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Spec    string `json:"spec"`
}

func examplesSearchHandler(opts Options) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		query, _ := args["query"].(string)
		if query == "" {
			return mcpgo.NewToolResultError("missing required argument: query"), nil
		}
		hits := searchExamples(opts.ExamplesFS, opts.ExamplesRoot, query, 5)
		body, _ := json.Marshal(map[string]any{"examples": hits})
		return mcpgo.NewToolResultText(string(body)), nil
	}
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

