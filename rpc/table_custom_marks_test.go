//go:build !js

package rpc

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/frankbardon/prism/custommark"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// E3-S1: a Twirp round-trip harness for a consumer binary that has
// registered custom marks and constructed its own rpc.NewServer.
// These tests exercise the Plot RPC with specs containing a `table`
// mark and a `custom` mark end to end (decode -> build -> execute ->
// encode -> render), asserting the responses carry correctly
// rendered output rather than an error or garbled/truncated content.
// This is a TEST-LOCAL harness within rpc/ — it does not touch the
// shipped cmd/prism binary, which registers zero custom marks.

// tableMarkSpecJSON is a table-mark spec with a plain column and a
// sparkline sub-mark column, mirroring render/html/table_test.go's
// tableSparklineSpecJSON fixture — enough surface to catch the Scene
// IR's scene.Table node (and its embedded sub-mark SVG) getting
// dropped or mangled somewhere along the RPC path.
const tableMarkSpecJSON = `{
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

// customMarkSpecJSON builds a minimal custom-mark spec referencing a
// renderer name registered via custommark.Register, mirroring render/
// svg/custom_test.go and render/html/custom_test.go's helper of the
// same shape.
func customMarkSpecJSON(rendererName string) string {
	return fmt.Sprintf(`{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"x": 1}, {"x": 2}]},
  "mark": {"type": "custom", "renderer": %q},
  "encoding": {}
}`, rendererName)
}

// rpcTestCustomRenderer implements both SVGCustomRenderer and
// HTMLCustomRenderer, mimicking a downstream consumer binary that
// registers one renderer usable under either render backend. The
// distinctive markers below let the assertions confirm the fragment
// survived the RPC round trip unmangled.
type rpcTestCustomRenderer struct{}

func (rpcTestCustomRenderer) RenderSVG(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return fmt.Sprintf(`<circle cx="%.3f" cy="%.3f" r="5" fill="tomato" data-marker="rpc-custom-svg"/>`, box.W/2, box.H/2), nil
}

func (rpcTestCustomRenderer) RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return `<strong data-marker="rpc-custom-html">rpc custom mark</strong>`, nil
}

// TestPrismTwirpServerPlotTableMarkHTML is E3-S1 acceptance criterion
// 1 (table mark, format=html): the Plot RPC must render a table-mark
// spec to well-formed HTML containing the table's header/body rows
// and the sparkline sub-mark's embedded inline SVG, not an error.
func TestPrismTwirpServerPlotTableMarkHTML(t *testing.T) {
	srv := newServer()
	resp, err := srv.Plot(context.Background(), &PlotRequest{
		Spec:   tableMarkSpecJSON,
		Format: "html",
		Width:  400,
		Height: 300,
	})
	if err != nil {
		t.Fatalf("Plot(table mark, html): %v", err)
	}
	if resp.Mime != "text/html" {
		t.Fatalf("Plot mime = %q, want text/html", resp.Mime)
	}
	body := string(resp.Bytes)
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

// TestPrismTwirpServerPlotTableMarkSVGFailsLoudly confirms the RPC
// path preserves render/svg's documented, intentional behaviour for
// the table mark (docs/src/concepts/marks.md: "table" has no SVG
// geometry equivalent — requesting it via the svg backend fails
// loudly with PRISM_RENDER_MARK_UNSUPPORTED rather than silently
// dropping the mark or emitting garbled output). This is not a gap in
// rpc/server.go; it's the shared render/render.go dispatch surfacing
// the same backend limitation cmd/prism and mcp/ already see.
func TestPrismTwirpServerPlotTableMarkSVGFailsLoudly(t *testing.T) {
	srv := newServer()
	_, err := srv.Plot(context.Background(), &PlotRequest{
		Spec:   tableMarkSpecJSON,
		Format: "svg",
		Width:  400,
		Height: 300,
	})
	if err == nil {
		t.Fatalf("Plot(table mark, svg) returned nil error; want PRISM_RENDER_MARK_UNSUPPORTED")
	}
	if !strings.Contains(err.Error(), "PRISM_RENDER_MARK_UNSUPPORTED") {
		t.Fatalf("Plot(table mark, svg) error = %v; want PRISM_RENDER_MARK_UNSUPPORTED", err)
	}
}

// TestPrismTwirpServerPlotCustomMarkHTML is E3-S1 acceptance criterion
// 2 (custom mark, format=html): a consumer that has registered a
// custom-mark renderer via custommark.Register before constructing
// its rpc.PrismServer must see that renderer's HTML fragment appear
// verbatim in the Plot response.
func TestPrismTwirpServerPlotCustomMarkHTML(t *testing.T) {
	const name = "rpc-e3s1-custom-html"
	custommark.ResetForTest(t)
	if err := custommark.Register(name, rpcTestCustomRenderer{}); err != nil {
		t.Fatalf("custommark.Register: %v", err)
	}

	srv := newServer()
	resp, err := srv.Plot(context.Background(), &PlotRequest{
		Spec:   customMarkSpecJSON(name),
		Format: "html",
		Width:  400,
		Height: 300,
	})
	if err != nil {
		t.Fatalf("Plot(custom mark, html): %v", err)
	}
	if resp.Mime != "text/html" {
		t.Fatalf("Plot mime = %q, want text/html", resp.Mime)
	}
	body := string(resp.Bytes)
	if !strings.Contains(body, `data-marker="rpc-custom-html"`) {
		t.Fatalf("expected the registered HTMLCustomRenderer's fragment verbatim in Plot output, got:\n%s", body)
	}
	if !strings.Contains(body, "rpc custom mark") {
		t.Fatalf("expected the custom mark's fragment text present, got:\n%s", body)
	}
}

// TestPrismTwirpServerPlotCustomMarkSVG is E3-S1 acceptance criterion
// 2's SVG counterpart: the same registered renderer's SVG fragment
// must appear verbatim in a format=svg Plot response.
func TestPrismTwirpServerPlotCustomMarkSVG(t *testing.T) {
	const name = "rpc-e3s1-custom-svg"
	custommark.ResetForTest(t)
	if err := custommark.Register(name, rpcTestCustomRenderer{}); err != nil {
		t.Fatalf("custommark.Register: %v", err)
	}

	srv := newServer()
	resp, err := srv.Plot(context.Background(), &PlotRequest{
		Spec:   customMarkSpecJSON(name),
		Format: "svg",
		Width:  400,
		Height: 300,
	})
	if err != nil {
		t.Fatalf("Plot(custom mark, svg): %v", err)
	}
	if resp.Mime != "image/svg+xml" {
		t.Fatalf("Plot mime = %q, want image/svg+xml", resp.Mime)
	}
	body := string(resp.Bytes)
	if !strings.Contains(body, `data-marker="rpc-custom-svg"`) {
		t.Fatalf("expected the registered SVGCustomRenderer's fragment verbatim in Plot output, got:\n%s", body)
	}
	if !strings.Contains(body, `fill="tomato"`) {
		t.Fatalf("expected the custom mark's spliced <circle> present, got:\n%s", body)
	}
}

// TestPrismTwirpServerPlotCustomMarkMissingRendererErrors confirms an
// unregistered custom-mark renderer name still surfaces the same
// PRISM_RENDER_CUSTOM_MARK_NOT_FOUND *errors.AppError through the RPC
// path that render/svg and render/html raise directly (E2-S2/E2-S3),
// rather than a bare Go error or a silently-dropped mark.
func TestPrismTwirpServerPlotCustomMarkMissingRendererErrors(t *testing.T) {
	custommark.ResetForTest(t)

	srv := newServer()
	_, err := srv.Plot(context.Background(), &PlotRequest{
		Spec:   customMarkSpecJSON("rpc-e3s1-does-not-exist"),
		Format: "html",
		Width:  400,
		Height: 300,
	})
	if err == nil {
		t.Fatalf("Plot(custom mark, unregistered renderer) returned nil error")
	}
	if !strings.Contains(err.Error(), "PRISM_RENDER_CUSTOM_MARK_NOT_FOUND") {
		t.Fatalf("Plot(custom mark, unregistered renderer) error = %v; want PRISM_RENDER_CUSTOM_MARK_NOT_FOUND", err)
	}
}
