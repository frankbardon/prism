//go:build !js

// Package gosdk is the thin adapter that mounts Prism's SDK-agnostic MCP
// surface onto a modelcontextprotocol/go-sdk server. It is the ONLY package
// (besides the soon-to-be-removed mcp/server.go wiring) that imports the
// go-sdk: everything else depends on the transport-neutral descriptor catalog
// in package mcp and the embedded examples corpus.
//
// Register iterates the descriptor catalog and the examples corpus and grafts
// them onto a caller-supplied *gosdk.Server, so a downstream host can mount
// Prism's tools alongside its own. It never constructs or returns a server.
package gosdk

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	gosdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/frankbardon/prism/examples"
	mcpcore "github.com/frankbardon/prism/mcp"
	"github.com/frankbardon/prism/rpc"
)

// exampleURIScheme prefixes every example-corpus resource URI. A resource URI
// is exampleURIScheme + stem, e.g. "prism://examples/bar_basic".
const exampleURIScheme = "prism://examples/"

// Register mounts Prism's MCP surface onto the caller-supplied server: every
// tool from the descriptor catalog (Tools(cfg)) and every spec in the embedded
// examples corpus (as a read-only resource). It does NOT construct or return a
// server, so a downstream MCP host can graft Prism's tools alongside its own.
//
// The tools are registered via the low-level gosdk.Server.AddTool with the
// reflected input/output schemas, and each handler returns the raw JSON text
// content the agent host expects (mirroring mcp/server.go's result shape); the
// typed AddTool[In,Out] form would wrap the payload in structured content
// instead.
func Register(server *gosdk.Server, facade *rpc.PrismServer, cfg mcpcore.Config) error {
	if server == nil {
		return fmt.Errorf("gosdk.Register: nil server")
	}
	for _, d := range mcpcore.Tools(cfg) {
		input, err := schemaOf(d.InputSchema)
		if err != nil {
			return fmt.Errorf("gosdk.Register: tool %q input schema: %w", d.Name, err)
		}
		output, err := schemaOf(d.OutputSchema)
		if err != nil {
			return fmt.Errorf("gosdk.Register: tool %q output schema: %w", d.Name, err)
		}
		server.AddTool(
			&gosdk.Tool{
				Name:         d.Name,
				Description:  d.Description,
				InputSchema:  input,
				OutputSchema: output,
			},
			toolHandler(d, facade),
		)
	}
	registerExampleResources(server)
	return nil
}

// schemaOf decodes a reflected JSON Schema (json.RawMessage) into the
// *jsonschema.Schema value go-sdk's AddTool expects. A nil/empty raw schema
// returns a nil schema (the descriptor always supplies one for input; output
// may legitimately be absent).
func schemaOf(raw json.RawMessage) (*jsonschema.Schema, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// toolHandler adapts a type-erased ToolDescriptor into a go-sdk tool handler:
// it forwards the raw call arguments to Invoke, marshals the typed output as
// JSON text content, and surfaces handler errors as tool-result errors
// (IsError=true) so the agent can self-correct. This mirrors mcp/server.go's
// descriptorHandler result shape exactly.
func toolHandler(d mcpcore.ToolDescriptor, facade *rpc.PrismServer) gosdk.ToolHandler {
	return func(ctx context.Context, req *gosdk.CallToolRequest) (*gosdk.CallToolResult, error) {
		var raw json.RawMessage
		if req != nil && req.Params != nil {
			raw = req.Params.Arguments
		}
		out, err := d.Invoke(ctx, facade, raw)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		body, _ := json.Marshal(out)
		return textResult(string(body)), nil
	}
}

// registerExampleResources mounts one read-only resource per valid spec in the
// embedded examples corpus. Each resource is JSON, addressed by
// exampleURIScheme + stem, and serves the spec bytes from examples.Get.
func registerExampleResources(server *gosdk.Server) {
	for _, name := range examples.List() {
		stem := name
		uri := exampleURIScheme + stem
		server.AddResource(
			&gosdk.Resource{
				Name:        stem,
				URI:         uri,
				MIMEType:    "application/json",
				Description: "Prism example spec: " + stem,
			},
			exampleResourceHandler(stem, uri),
		)
	}
}

// exampleResourceHandler serves a single example spec's bytes. It re-reads from
// examples.Get so the corpus stays the single source of truth; a missing stem
// (the corpus changed under us) surfaces as a protocol ResourceNotFoundError.
func exampleResourceHandler(stem, uri string) gosdk.ResourceHandler {
	return func(_ context.Context, _ *gosdk.ReadResourceRequest) (*gosdk.ReadResourceResult, error) {
		body, ok := examples.Get(stem)
		if !ok {
			return nil, gosdk.ResourceNotFoundError(uri)
		}
		return &gosdk.ReadResourceResult{
			Contents: []*gosdk.ResourceContents{{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(body),
			}},
		}, nil
	}
}

// textResult wraps a JSON payload string as a successful tool result.
func textResult(body string) *gosdk.CallToolResult {
	return &gosdk.CallToolResult{
		Content: []gosdk.Content{&gosdk.TextContent{Text: body}},
	}
}

// errorResult surfaces a facade/argument error as a tool-result error
// (IsError=true) rather than an MCP protocol-level Go error, so the agent can
// see the message and self-correct.
func errorResult(msg string) *gosdk.CallToolResult {
	return &gosdk.CallToolResult{
		IsError: true,
		Content: []gosdk.Content{&gosdk.TextContent{Text: msg}},
	}
}
