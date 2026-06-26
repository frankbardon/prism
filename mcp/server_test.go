//go:build !js

package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	gosdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/afero"

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

// newTestSession builds an MCP server via the public New() entrypoint
// (all four tools registered) and connects an in-memory go-sdk client
// to it. Returns the client session; both sessions are torn down on
// cleanup. Leaves ExamplesRoot empty so prism_examples_search exercises
// the embedded examples corpus — the default a real `prism mcp` uses.
func newTestSession(t *testing.T) *gosdk.ClientSession {
	t.Helper()
	return connectSession(t, Options{
		PrismServer: &rpc.PrismServer{Fs: afero.NewMemMapFs()},
	})
}

// connectSession wires an in-memory client to a server built from opts and
// registers teardown for both sessions.
func connectSession(t *testing.T, opts Options) *gosdk.ClientSession {
	t.Helper()
	srv := New(opts)

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

// TestPrismMCPToolsRegistered sends tools/list to the running server
// and asserts all four tool names are present.
func TestPrismMCPToolsRegistered(t *testing.T) {
	cs := newTestSession(t)

	listResult, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := map[string]bool{}
	for _, tool := range listResult.Tools {
		got[tool.Name] = true
	}
	want := []string{"prism_plot", "prism_validate", "prism_describe", "prism_examples_search"}
	for _, name := range want {
		if !got[name] {
			t.Errorf("tool %q not registered (got: %v)", name, got)
		}
	}
}

// TestPrismMCPPlotTool exercises the prism_plot round trip end-to-end
// through the in-memory client + transport.
func TestPrismMCPPlotTool(t *testing.T) {
	cs := newTestSession(t)
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
	var payload plotResult
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

// TestPrismMCPPlotToolPDF round-trips the prism_plot tool with
// format=pdf; verifies the response carries application/pdf mime +
// base64-decoded bytes start with %PDF-.
func TestPrismMCPPlotToolPDF(t *testing.T) {
	cs := newTestSession(t)
	res, err := cs.CallTool(context.Background(), &gosdk.CallToolParams{
		Name: "prism_plot",
		Arguments: map[string]any{
			"spec":   fixtureSpec,
			"format": "pdf",
		},
	})
	if err != nil {
		t.Fatalf("CallTool prism_plot pdf: %v", err)
	}
	if res.IsError {
		t.Fatalf("prism_plot pdf returned error: %s", textOf(res))
	}
	var payload plotResult
	if err := json.Unmarshal([]byte(textOf(res)), &payload); err != nil {
		t.Fatalf("plot result parse: %v\n%s", err, textOf(res))
	}
	if payload.Mime != "application/pdf" {
		t.Errorf("mime = %q; want application/pdf", payload.Mime)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload.Bytes)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if !strings.HasPrefix(string(decoded), "%PDF-") {
		t.Errorf("decoded bytes do not start with %%PDF-")
	}
	if len(decoded) < 1000 {
		t.Errorf("decoded PDF unexpectedly small: %d bytes", len(decoded))
	}
}

// TestPrismMCPValidateTool round-trips the prism_validate tool on a
// valid spec.
func TestPrismMCPValidateTool(t *testing.T) {
	cs := newTestSession(t)

	res, err := cs.CallTool(context.Background(), &gosdk.CallToolParams{
		Name:      "prism_validate",
		Arguments: map[string]any{"spec": fixtureSpec},
	})
	if err != nil {
		t.Fatalf("CallTool prism_validate (valid): %v", err)
	}
	if res.IsError {
		t.Fatalf("prism_validate (valid) returned error: %s", textOf(res))
	}
	var v struct {
		Ok     bool             `json:"ok"`
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal([]byte(textOf(res)), &v); err != nil {
		t.Fatalf("validate body parse: %v", err)
	}
	if !v.Ok {
		t.Errorf("Validate(valid) ok=false; errors=%v", v.Errors)
	}
}

// TestPrismMCPDescribeTool exercises prism_describe.
func TestPrismMCPDescribeTool(t *testing.T) {
	cs := newTestSession(t)
	res, err := cs.CallTool(context.Background(), &gosdk.CallToolParams{
		Name:      "prism_describe",
		Arguments: map[string]any{"spec": fixtureSpec},
	})
	if err != nil {
		t.Fatalf("CallTool prism_describe: %v", err)
	}
	if res.IsError {
		t.Fatalf("prism_describe returned error: %s", textOf(res))
	}
	var d struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(textOf(res)), &d); err != nil {
		t.Fatalf("describe body parse: %v", err)
	}
	if !strings.Contains(d.Summary, "bar chart") {
		t.Errorf("summary missing 'bar chart': %q", d.Summary)
	}
	if !strings.Contains(d.Summary, "hello") {
		t.Errorf("summary missing title 'hello': %q", d.Summary)
	}
}

// TestPrismMCPExamplesSearchTool exercises prism_examples_search against
// the EMBEDDED examples corpus (the default — no ExamplesRoot set), which
// is what a real `prism mcp` with no --examples-root flag serves.
func TestPrismMCPExamplesSearchTool(t *testing.T) {
	cs := newTestSession(t)
	res, err := cs.CallTool(context.Background(), &gosdk.CallToolParams{
		Name:      "prism_examples_search",
		Arguments: map[string]any{"query": "bar"},
	})
	if err != nil {
		t.Fatalf("CallTool prism_examples_search: %v", err)
	}
	if res.IsError {
		t.Fatalf("prism_examples_search returned error: %s", textOf(res))
	}
	var s struct {
		Examples []exampleResult `json:"examples"`
	}
	if err := json.Unmarshal([]byte(textOf(res)), &s); err != nil {
		t.Fatalf("search body parse: %v", err)
	}
	if len(s.Examples) == 0 {
		t.Fatalf("search returned no examples")
	}
	if len(s.Examples) > 5 {
		t.Errorf("expected at most 5 results (cap); got %d", len(s.Examples))
	}
	if s.Examples[0].Name != "bar_basic" {
		t.Errorf("expected first match bar_basic; got %q", s.Examples[0].Name)
	}
	if s.Examples[0].Summary == "" || s.Examples[0].Spec == "" {
		t.Errorf("embedded result missing summary/spec: %+v", s.Examples[0])
	}
}

// TestPrismMCPExamplesSearchOverride confirms a non-empty ExamplesRoot
// still drives the on-disk afero walk instead of the embedded corpus.
func TestPrismMCPExamplesSearchOverride(t *testing.T) {
	exFS := afero.NewMemMapFs()
	_ = afero.WriteFile(exFS, "fixtures/only_override.json",
		[]byte(`{"$schema":"urn:prism:schema:v1:spec","title":"override only","mark":"bar","encoding":{}}`), 0o644)

	cs := connectSession(t, Options{
		PrismServer:  &rpc.PrismServer{Fs: afero.NewMemMapFs()},
		ExamplesRoot: "fixtures/",
		ExamplesFS:   exFS,
	})

	res, err := cs.CallTool(context.Background(), &gosdk.CallToolParams{
		Name:      "prism_examples_search",
		Arguments: map[string]any{"query": "override"},
	})
	if err != nil {
		t.Fatalf("CallTool prism_examples_search (override): %v", err)
	}
	if res.IsError {
		t.Fatalf("prism_examples_search (override) returned error: %s", textOf(res))
	}
	var s struct {
		Examples []exampleResult `json:"examples"`
	}
	if err := json.Unmarshal([]byte(textOf(res)), &s); err != nil {
		t.Fatalf("search body parse: %v", err)
	}
	if len(s.Examples) != 1 || s.Examples[0].Name != "only_override" {
		t.Fatalf("expected the single on-disk override fixture; got %+v", s.Examples)
	}
}

// TestPrismMCPNewSmoke ensures the public New() entrypoint registers
// exactly the four tools (no missing wiring), driven over the
// in-memory transport.
func TestPrismMCPNewSmoke(t *testing.T) {
	cs := newTestSession(t)
	listResult, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(listResult.Tools) != 4 {
		t.Fatalf("ListTools returned %d tools; want 4 (got %v)", len(listResult.Tools), listResult.Tools)
	}
}

// TestPrismMCPPlotMissingSpec confirms a missing required argument
// surfaces as a tool-result error (IsError=true), not a protocol-level
// Go error — so the agent can see the message and self-correct.
func TestPrismMCPPlotMissingSpec(t *testing.T) {
	cs := newTestSession(t)
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
