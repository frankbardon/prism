//go:build !js

package gosdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	gosdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/afero"

	mcpcore "github.com/frankbardon/prism/mcp"
	"github.com/frankbardon/prism/rpc"
)

const fixtureSpec = `{
  "$schema":"urn:prism:schema:v1:spec",
  "title":"hello",
  "data":{"name":"v","values":[
    {"x":"a","y":1.0},{"x":"b","y":2.0},{"x":"c","y":3.0}
  ]},
  "mark":"bar",
  "encoding":{
    "x":{"field":"x","type":"nominal"},
    "y":{"field":"y","type":"quantitative"}
  }
}`

// connect mounts Prism's surface onto a bare go-sdk server via Register and
// wires an in-memory client to it. Both sessions are torn down on cleanup.
func connect(t *testing.T) *gosdk.ClientSession {
	t.Helper()
	srv := gosdk.NewServer(&gosdk.Implementation{Name: "prism", Version: "0.0.0"}, nil)
	if err := Register(srv, &rpc.PrismServer{Fs: afero.NewMemMapFs()}, mcpcore.Config{}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	ctx := context.Background()
	serverT, clientT := gosdk.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := gosdk.NewClient(&gosdk.Implementation{Name: "test", Version: "0.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	return clientSession
}

// TestRegisterMountsAllTools asserts Register grafts the full descriptor
// catalog (all four tools) onto the caller-supplied server.
func TestRegisterMountsAllTools(t *testing.T) {
	cs := connect(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	for _, name := range []string{"prism_plot", "prism_validate", "prism_describe", "prism_examples_search"} {
		if !got[name] {
			t.Errorf("tool %q not registered (got: %v)", name, got)
		}
	}
}

// TestRegisterPlotEndToEnd drives prism_plot through the in-memory client +
// transport and asserts the result shape matches the descriptor handler
// contract (base64 SVG bytes + mime + caption, returned as JSON text content).
func TestRegisterPlotEndToEnd(t *testing.T) {
	cs := connect(t)
	res, err := cs.CallTool(context.Background(), &gosdk.CallToolParams{
		Name: "prism_plot",
		Arguments: map[string]any{
			"spec":   fixtureSpec,
			"format": "svg",
		},
	})
	if err != nil {
		t.Fatalf("CallTool prism_plot: %v", err)
	}
	if res.IsError {
		t.Fatalf("prism_plot returned error: %s", textOf(res))
	}
	var payload mcpcore.PlotOutput
	if err := json.Unmarshal([]byte(textOf(res)), &payload); err != nil {
		t.Fatalf("plot result parse: %v\n%s", err, textOf(res))
	}
	if payload.Mime != "image/svg+xml" {
		t.Errorf("mime = %q; want image/svg+xml", payload.Mime)
	}
	decoded, _ := base64.StdEncoding.DecodeString(payload.Bytes)
	if !strings.HasPrefix(strings.TrimSpace(string(decoded)), "<svg") {
		t.Errorf("decoded bytes do not start with <svg")
	}
	if payload.Caption == "" {
		t.Errorf("caption empty")
	}
}

// TestRegisterPlotMissingSpec confirms a missing required argument surfaces as
// a tool-result error (IsError=true), mirroring the descriptor handler contract.
func TestRegisterPlotMissingSpec(t *testing.T) {
	cs := connect(t)
	res, err := cs.CallTool(context.Background(), &gosdk.CallToolParams{
		Name:      "prism_plot",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool prism_plot (missing spec): %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected IsError=true for missing spec; got success: %s", textOf(res))
	}
	if !strings.Contains(textOf(res), "missing required argument: spec") {
		t.Errorf("error text = %q; want 'missing required argument: spec'", textOf(res))
	}
}

// TestRegisterExampleResources asserts Register mounts the examples corpus as
// resources and that one can be read back end-to-end.
func TestRegisterExampleResources(t *testing.T) {
	cs := connect(t)
	res, err := cs.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(res.Resources) == 0 {
		t.Fatalf("no example resources registered")
	}
	var probe string
	for _, r := range res.Resources {
		if strings.HasPrefix(r.URI, exampleURIScheme) {
			probe = r.URI
			break
		}
	}
	if probe == "" {
		t.Fatalf("no resource with %q prefix (got %d resources)", exampleURIScheme, len(res.Resources))
	}
	read, err := cs.ReadResource(context.Background(), &gosdk.ReadResourceParams{URI: probe})
	if err != nil {
		t.Fatalf("ReadResource %q: %v", probe, err)
	}
	if len(read.Contents) == 0 || read.Contents[0].Text == "" {
		t.Fatalf("resource %q returned empty contents", probe)
	}
	if read.Contents[0].MIMEType != "application/json" {
		t.Errorf("resource mime = %q; want application/json", read.Contents[0].MIMEType)
	}
}

// textOf concatenates every TextContent entry in a CallToolResult.
func textOf(res *gosdk.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*gosdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
