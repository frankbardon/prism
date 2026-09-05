//go:build !js

package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/custommark"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/rpc"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// E3-S2: an MCP tool-call harness for a consumer binary that has
// registered custom marks and wired mcpserve.ServeStdio/ServeTransport
// itself. These tests exercise the prism_plot tool (mcp/catalog.go's
// only mark-rendering tool) with specs containing a `table` mark and a
// `custom` mark, asserting the tool-call responses carry correctly
// rendered output rather than an error or garbled/truncated content.
// This mirrors rpc/table_custom_marks_test.go (E3-S1) — a TEST-LOCAL
// harness within mcp/, not touching cmd/prism/cmd_mcp.go, which
// registers zero custom marks.

// mcpTableMarkSpecJSON is the same table-mark fixture E3-S1 used for
// the Twirp path: a plain column plus a sparkline sub-mark column, so
// the assertions catch the Scene IR's scene.Table node (and its
// embedded sub-mark SVG) getting dropped or mangled anywhere along the
// MCP tool-call path.
const mcpTableMarkSpecJSON = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "values": [
      {"name": "Acme", "revenue": 120, "trend": [10, 12, 9, 14, 20]},
      {"name": "Globex", "revenue": 80, "trend": [5, 6, 4, 7, 9]}
    ]
  },
  "mark": {"type": "table"},
  "encoding": {
    "columns": [
      {"field": "name", "type": "nominal", "title": "Account"},
      {"field": "revenue", "type": "quantitative"},
      {"field": "trend", "type": "quantitative", "mark": "sparkline", "title": "Trend"}
    ]
  }
}`

// mcpCustomMarkSpecJSON builds a minimal custom-mark spec referencing
// a renderer name registered via custommark.Register, mirroring
// rpc/table_custom_marks_test.go's helper of the same shape.
func mcpCustomMarkSpecJSON(rendererName string) string {
	return fmt.Sprintf(`{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"x": 1}, {"x": 2}]},
  "mark": {"type": "custom", "renderer": %q},
  "encoding": {}
}`, rendererName)
}

// mcpTestCustomRenderer implements both SVGCustomRenderer and
// HTMLCustomRenderer, mimicking a downstream consumer binary that
// registers one renderer usable under either render backend. The
// distinctive markers below let the assertions confirm the fragment
// survived the MCP tool-call round trip unmangled.
type mcpTestCustomRenderer struct{}

func (mcpTestCustomRenderer) RenderSVG(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return fmt.Sprintf(`<circle cx="%.3f" cy="%.3f" r="5" fill="tomato" data-marker="mcp-custom-svg"/>`, box.W/2, box.H/2), nil
}

func (mcpTestCustomRenderer) RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return `<strong data-marker="mcp-custom-html">mcp custom mark</strong>`, nil
}

// mcpTestServer builds a bare *rpc.PrismServer suitable for driving
// the prism_plot tool directly, mirroring catalog_test.go's existing
// construction.
func mcpTestServer() *rpc.PrismServer {
	return &rpc.PrismServer{Fs: afero.NewMemMapFs()}
}

// decodePlotOutput unwraps the base64-encoded PlotOutput.Bytes so
// assertions can inspect the rendered text directly.
func decodePlotOutput(t *testing.T, out PlotOutput) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(out.Bytes)
	if err != nil {
		t.Fatalf("decode PlotOutput.Bytes: %v", err)
	}
	return string(raw)
}

// TestMCPPlotToolTableMarkHTML is E3-S2 acceptance criterion 1 (table
// mark, format=html): invoking prism_plot's typed handler with a
// table-mark spec must render well-formed HTML containing the table's
// header/body rows and the sparkline sub-mark's embedded inline SVG,
// not an error.
func TestMCPPlotToolTableMarkHTML(t *testing.T) {
	f := mcpTestServer()
	out, err := PlotTool(context.Background(), f, PlotInput{Spec: mcpTableMarkSpecJSON, Format: "html"})
	if err != nil {
		t.Fatalf("PlotTool(table mark, html): %v", err)
	}
	if out.Mime != "text/html" {
		t.Fatalf("PlotTool mime = %q, want text/html", out.Mime)
	}
	body := decodePlotOutput(t, out)
	if !strings.Contains(body, "<table") {
		t.Fatalf("expected a <table> element in table-mark HTML output, got:\n%s", body)
	}
	if !strings.Contains(body, "Acme") || !strings.Contains(body, "Globex") {
		t.Fatalf("expected both data rows present in table-mark HTML output, got:\n%s", body)
	}
	if !strings.Contains(body, "<svg") {
		t.Fatalf("expected the sparkline sub-mark's embedded inline <svg> in table-mark HTML output, got:\n%s", body)
	}
}

// TestMCPPlotToolTableMarkSVGFailsLoudly confirms the MCP tool-call
// path preserves render/svg's documented, intentional behaviour for
// the table mark (docs/src/concepts/marks.md: "table" has no SVG
// geometry equivalent — requesting it via the svg backend fails
// loudly with PRISM_RENDER_MARK_UNSUPPORTED rather than silently
// dropping the mark or emitting garbled output). This is not a gap in
// mcp/server.go; it's the shared render/render.go dispatch surfacing
// the same backend limitation cmd/prism and rpc/ already see.
func TestMCPPlotToolTableMarkSVGFailsLoudly(t *testing.T) {
	f := mcpTestServer()
	_, err := PlotTool(context.Background(), f, PlotInput{Spec: mcpTableMarkSpecJSON, Format: "svg"})
	if err == nil {
		t.Fatalf("PlotTool(table mark, svg) returned nil error; want PRISM_RENDER_MARK_UNSUPPORTED")
	}
	if !strings.Contains(err.Error(), "PRISM_RENDER_MARK_UNSUPPORTED") {
		t.Fatalf("PlotTool(table mark, svg) error = %v; want PRISM_RENDER_MARK_UNSUPPORTED", err)
	}
}

// TestMCPPlotToolCustomMarkHTML is E3-S2 acceptance criterion 2
// (custom mark, format=html): a consumer that has registered a
// custom-mark renderer via custommark.Register before invoking
// prism_plot must see that renderer's HTML fragment appear verbatim in
// the tool's output.
func TestMCPPlotToolCustomMarkHTML(t *testing.T) {
	const name = "mcp-e3s2-custom-html"
	custommark.ResetForTest(t)
	if err := custommark.Register(name, mcpTestCustomRenderer{}); err != nil {
		t.Fatalf("custommark.Register: %v", err)
	}

	f := mcpTestServer()
	out, err := PlotTool(context.Background(), f, PlotInput{Spec: mcpCustomMarkSpecJSON(name), Format: "html"})
	if err != nil {
		t.Fatalf("PlotTool(custom mark, html): %v", err)
	}
	if out.Mime != "text/html" {
		t.Fatalf("PlotTool mime = %q, want text/html", out.Mime)
	}
	body := decodePlotOutput(t, out)
	if !strings.Contains(body, `data-marker="mcp-custom-html"`) {
		t.Fatalf("expected the registered HTMLCustomRenderer's fragment verbatim in PlotTool output, got:\n%s", body)
	}
	if !strings.Contains(body, "mcp custom mark") {
		t.Fatalf("expected the custom mark's fragment text present, got:\n%s", body)
	}
}

// TestMCPPlotToolCustomMarkSVG is E3-S2 acceptance criterion 2's SVG
// counterpart: the same registered renderer's SVG fragment must
// appear verbatim in a format=svg prism_plot invocation.
func TestMCPPlotToolCustomMarkSVG(t *testing.T) {
	const name = "mcp-e3s2-custom-svg"
	custommark.ResetForTest(t)
	if err := custommark.Register(name, mcpTestCustomRenderer{}); err != nil {
		t.Fatalf("custommark.Register: %v", err)
	}

	f := mcpTestServer()
	out, err := PlotTool(context.Background(), f, PlotInput{Spec: mcpCustomMarkSpecJSON(name), Format: "svg"})
	if err != nil {
		t.Fatalf("PlotTool(custom mark, svg): %v", err)
	}
	if out.Mime != "image/svg+xml" {
		t.Fatalf("PlotTool mime = %q, want image/svg+xml", out.Mime)
	}
	body := decodePlotOutput(t, out)
	if !strings.Contains(body, `data-marker="mcp-custom-svg"`) {
		t.Fatalf("expected the registered SVGCustomRenderer's fragment verbatim in PlotTool output, got:\n%s", body)
	}
	if !strings.Contains(body, `fill="tomato"`) {
		t.Fatalf("expected the custom mark's spliced <circle> present, got:\n%s", body)
	}
}

// TestMCPPlotToolCustomMarkMissingRendererErrors confirms an
// unregistered custom-mark renderer name still surfaces the same
// PRISM_RENDER_CUSTOM_MARK_NOT_FOUND *errors.AppError through the MCP
// tool-call path that render/svg and render/html raise directly, and
// that rpc/'s Twirp path (E3-S1) already confirmed, rather than a bare
// Go error or a silently-dropped mark.
func TestMCPPlotToolCustomMarkMissingRendererErrors(t *testing.T) {
	custommark.ResetForTest(t)

	f := mcpTestServer()
	_, err := PlotTool(context.Background(), f, PlotInput{Spec: mcpCustomMarkSpecJSON("mcp-e3s2-does-not-exist"), Format: "html"})
	if err == nil {
		t.Fatalf("PlotTool(custom mark, unregistered renderer) returned nil error")
	}
	if !strings.Contains(err.Error(), "PRISM_RENDER_CUSTOM_MARK_NOT_FOUND") {
		t.Fatalf("PlotTool(custom mark, unregistered renderer) error = %v; want PRISM_RENDER_CUSTOM_MARK_NOT_FOUND", err)
	}
}

// TestMCPPlotToolTableMarkViaCatalogInvoke drives the same table-mark
// HTML case through Tools(cfg)'s type-erased Invoke closure (rather
// than calling PlotTool directly) so the catalog wiring itself —
// json.RawMessage decode -> typed handler -> any — is proven for a
// table mark, matching how a real MCP transport would call in.
func TestMCPPlotToolTableMarkViaCatalogInvoke(t *testing.T) {
	tools := Tools(Config{})
	var plot ToolDescriptor
	for _, d := range tools {
		if d.Name == "prism_plot" {
			plot = d
		}
	}
	if plot.Invoke == nil {
		t.Fatalf("prism_plot descriptor missing from catalog")
	}

	raw, err := json.Marshal(PlotInput{Spec: mcpTableMarkSpecJSON, Format: "html"})
	if err != nil {
		t.Fatalf("marshal PlotInput: %v", err)
	}
	out, err := plot.Invoke(context.Background(), mcpTestServer(), raw)
	if err != nil {
		t.Fatalf("prism_plot Invoke(table mark, html): %v", err)
	}
	pout, ok := out.(PlotOutput)
	if !ok {
		t.Fatalf("prism_plot Invoke returned %T; want PlotOutput", out)
	}
	body := decodePlotOutput(t, pout)
	if !strings.Contains(body, "<table") {
		t.Fatalf("expected a <table> element in table-mark HTML output via catalog Invoke, got:\n%s", body)
	}
}
