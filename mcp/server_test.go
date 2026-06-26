//go:build !js

package mcp

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

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

// newFacade returns a hermetic rpc facade backed by an in-memory filesystem —
// the seam the typed tool handlers call into.
func newFacade() *rpc.PrismServer {
	return &rpc.PrismServer{Fs: afero.NewMemMapFs()}
}

// TestPlotTool exercises the typed prism_plot handler directly against the
// facade (no MCP SDK): base64 SVG bytes + mime + caption.
func TestPlotTool(t *testing.T) {
	out, err := PlotTool(context.Background(), newFacade(), PlotInput{Spec: fixtureSpec, Format: "svg"})
	if err != nil {
		t.Fatalf("PlotTool: %v", err)
	}
	if out.Mime != "image/svg+xml" {
		t.Errorf("mime = %q; want image/svg+xml", out.Mime)
	}
	decoded, _ := base64.StdEncoding.DecodeString(out.Bytes)
	if !strings.HasPrefix(strings.TrimSpace(string(decoded)), "<svg") {
		t.Errorf("decoded bytes do not start with <svg")
	}
	if out.Caption == "" {
		t.Errorf("caption empty")
	}
}

// TestPlotToolPDF confirms format=pdf returns application/pdf bytes.
func TestPlotToolPDF(t *testing.T) {
	out, err := PlotTool(context.Background(), newFacade(), PlotInput{Spec: fixtureSpec, Format: "pdf"})
	if err != nil {
		t.Fatalf("PlotTool pdf: %v", err)
	}
	if out.Mime != "application/pdf" {
		t.Errorf("mime = %q; want application/pdf", out.Mime)
	}
	decoded, err := base64.StdEncoding.DecodeString(out.Bytes)
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

// TestPlotToolMissingSpec confirms the missing-argument guard returns a Go
// error (the SDK adapter is what maps it to a tool-result error).
func TestPlotToolMissingSpec(t *testing.T) {
	_, err := PlotTool(context.Background(), newFacade(), PlotInput{})
	if err == nil {
		t.Fatal("expected error for missing spec; got nil")
	}
	if !strings.Contains(err.Error(), "missing required argument: spec") {
		t.Errorf("error = %q; want 'missing required argument: spec'", err.Error())
	}
}

// TestValidateTool round-trips the typed prism_validate handler on a valid
// spec.
func TestValidateTool(t *testing.T) {
	out, err := ValidateTool(context.Background(), newFacade(), ValidateInput{Spec: fixtureSpec})
	if err != nil {
		t.Fatalf("ValidateTool: %v", err)
	}
	if !out.Ok {
		t.Errorf("Validate(valid) ok=false; errors=%v", out.Errors)
	}
}

// TestDescribeTool exercises the typed prism_describe handler.
func TestDescribeTool(t *testing.T) {
	out, err := DescribeTool(context.Background(), newFacade(), DescribeInput{Spec: fixtureSpec})
	if err != nil {
		t.Fatalf("DescribeTool: %v", err)
	}
	if !strings.Contains(out.Summary, "bar chart") {
		t.Errorf("summary missing 'bar chart': %q", out.Summary)
	}
	if !strings.Contains(out.Summary, "hello") {
		t.Errorf("summary missing title 'hello': %q", out.Summary)
	}
}

// TestExamplesSearchToolEmbedded searches the embedded corpus (empty Root),
// the default a real `prism mcp` with no --examples-root serves.
func TestExamplesSearchToolEmbedded(t *testing.T) {
	out, err := ExamplesSearchTool(context.Background(), newFacade(), ExamplesSearchInput{Query: "bar"})
	if err != nil {
		t.Fatalf("ExamplesSearchTool: %v", err)
	}
	if len(out.Examples) == 0 {
		t.Fatalf("search returned no examples")
	}
	if len(out.Examples) > 5 {
		t.Errorf("expected at most 5 results (cap); got %d", len(out.Examples))
	}
	if out.Examples[0].Name != "bar_basic" {
		t.Errorf("expected first match bar_basic; got %q", out.Examples[0].Name)
	}
	if out.Examples[0].Summary == "" || out.Examples[0].Spec == "" {
		t.Errorf("embedded result missing summary/spec: %+v", out.Examples[0])
	}
}

// TestExamplesSearchToolOverride confirms a non-empty Root drives the on-disk
// afero walk instead of the embedded corpus.
func TestExamplesSearchToolOverride(t *testing.T) {
	exFS := afero.NewMemMapFs()
	_ = afero.WriteFile(exFS, "fixtures/only_override.json",
		[]byte(`{"$schema":"urn:prism:schema:v1:spec","title":"override only","mark":"bar","encoding":{}}`), 0o644)

	out, err := ExamplesSearchTool(context.Background(), newFacade(), ExamplesSearchInput{
		Query: "override",
		Root:  "fixtures/",
		FS:    exFS,
	})
	if err != nil {
		t.Fatalf("ExamplesSearchTool (override): %v", err)
	}
	if len(out.Examples) != 1 || out.Examples[0].Name != "only_override" {
		t.Fatalf("expected the single on-disk override fixture; got %+v", out.Examples)
	}
}

// TestExamplesSearchToolMissingQuery confirms the missing-argument guard.
func TestExamplesSearchToolMissingQuery(t *testing.T) {
	_, err := ExamplesSearchTool(context.Background(), newFacade(), ExamplesSearchInput{})
	if err == nil {
		t.Fatal("expected error for missing query; got nil")
	}
	if !strings.Contains(err.Error(), "missing required argument: query") {
		t.Errorf("error = %q; want 'missing required argument: query'", err.Error())
	}
}

// TestSummariseSpec covers the pure summariser used by prism_describe.
func TestSummariseSpec(t *testing.T) {
	summary := summariseSpec(fixtureSpec)
	if !strings.Contains(summary, "bar chart") {
		t.Errorf("summary missing 'bar chart': %q", summary)
	}
	if summariseSpec("{not json") != "" {
		t.Errorf("expected empty summary for undecodable spec")
	}
}

// TestSearchExamples covers the on-disk afero walk helper directly.
func TestSearchExamples(t *testing.T) {
	fsys := afero.NewMemMapFs()
	_ = afero.WriteFile(fsys, "specs/bar_demo.json",
		[]byte(`{"$schema":"urn:prism:schema:v1:spec","title":"Bar demo","mark":"bar","encoding":{}}`), 0o644)
	_ = afero.WriteFile(fsys, "specs/invalid/broken.json", []byte(`{bad`), 0o644)

	hits := searchExamples(fsys, "specs/", "bar", 5)
	if len(hits) != 1 || hits[0].Name != "bar_demo" {
		t.Fatalf("expected the single bar_demo hit (invalid/ skipped); got %+v", hits)
	}
	if hits[0].Summary != "Bar demo" {
		t.Errorf("summary = %q; want title 'Bar demo'", hits[0].Summary)
	}
}
