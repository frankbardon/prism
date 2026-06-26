//go:build !js

package rpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/resolve"
)

// minimalSpec is the smallest valid Prism spec; mirrors
// examples/specs/bar_basic.json without dragging the fixture into
// this package's test surface.
const minimalSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "title": "tiny",
  "data": {
    "name": "v",
    "values": [
      {"x": "a", "y": 1},
      {"x": "b", "y": 2}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "x", "type": "nominal"},
    "y": {"field": "y", "type": "quantitative"}
  }
}`

// invalidSpec exercises Validate's failure path (unknown mark name).
const invalidSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{"x": 1, "y": 2}]},
  "mark": "not_a_real_mark",
  "encoding": {
    "x": {"field": "x", "type": "quantitative"},
    "y": {"field": "y", "type": "quantitative"}
  }
}`

func newServer() *PrismServer {
	return &PrismServer{
		DatasetRegistry: resolve.MapDatasetRegistry{},
		Fs:              afero.NewMemMapFs(),
	}
}

func TestPrismTwirpServerPlotSVG(t *testing.T) {
	srv := newServer()
	resp, err := srv.Plot(context.Background(), &PlotRequest{
		Spec:   minimalSpec,
		Format: "svg",
		Width:  400,
		Height: 300,
	})
	if err != nil {
		t.Fatalf("Plot: %v", err)
	}
	if resp.Mime != "image/svg+xml" {
		t.Fatalf("Plot mime = %q, want image/svg+xml", resp.Mime)
	}
	if len(resp.Bytes) == 0 {
		t.Fatalf("Plot returned zero bytes")
	}
	if !strings.HasPrefix(strings.TrimSpace(string(resp.Bytes)), "<svg") {
		t.Fatalf("Plot bytes do not start with <svg: %q...", resp.Bytes[:min(len(resp.Bytes), 32)])
	}
}

func TestPrismTwirpServerPlotPNGUnimplemented(t *testing.T) {
	srv := newServer()
	_, err := srv.Plot(context.Background(), &PlotRequest{
		Spec: minimalSpec, Format: "png",
	})
	if err == nil {
		t.Fatalf("Plot(format=png) returned nil error; want PRISM_RENDER_FORMAT_UNAVAILABLE")
	}
	if !strings.Contains(err.Error(), "PRISM_RENDER_FORMAT_UNAVAILABLE") {
		t.Fatalf("Plot(format=png) error = %v; want PRISM_RENDER_FORMAT_UNAVAILABLE", err)
	}
}

// TestPrismTwirpServerPlotPDF — PDF lands in P15. The handler should
// return application/pdf bytes starting with the %PDF- magic.
func TestPrismTwirpServerPlotPDF(t *testing.T) {
	srv := newServer()
	resp, err := srv.Plot(context.Background(), &PlotRequest{
		Spec: minimalSpec, Format: "pdf",
	})
	if err != nil {
		t.Fatalf("Plot(format=pdf): %v", err)
	}
	if resp.Mime != "application/pdf" {
		t.Fatalf("Plot mime = %q, want application/pdf", resp.Mime)
	}
	if len(resp.Bytes) < 1000 {
		t.Fatalf("Plot(format=pdf) returned %d bytes; want a non-trivial PDF", len(resp.Bytes))
	}
	if !strings.HasPrefix(string(resp.Bytes), "%PDF-") {
		t.Fatalf("Plot(format=pdf) bytes do not start with %%PDF-: %q...", resp.Bytes[:min(len(resp.Bytes), 16)])
	}
}

func TestPrismTwirpServerValidateOK(t *testing.T) {
	srv := newServer()
	resp, err := srv.Validate(context.Background(), &ValidateRequest{Spec: minimalSpec})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !resp.Ok {
		t.Fatalf("Validate ok=false unexpectedly: errors=%v", resp.Errors)
	}
	if len(resp.Errors) != 0 {
		t.Fatalf("Validate returned %d errors; want 0", len(resp.Errors))
	}
}

func TestPrismTwirpServerValidateBad(t *testing.T) {
	srv := newServer()
	resp, err := srv.Validate(context.Background(), &ValidateRequest{Spec: invalidSpec})
	if err != nil {
		t.Fatalf("Validate(invalid): %v", err)
	}
	if resp.Ok {
		t.Fatalf("Validate(invalid) ok=true; want false")
	}
	if len(resp.Errors) == 0 {
		t.Fatalf("Validate(invalid) returned no errors")
	}
}

func TestPrismTwirpServerScene(t *testing.T) {
	srv := newServer()
	resp, err := srv.Scene(context.Background(), &SceneRequest{
		Spec: minimalSpec, Width: 400, Height: 300,
	})
	if err != nil {
		t.Fatalf("Scene: %v", err)
	}
	if resp.SceneJson == "" {
		t.Fatalf("Scene returned empty scene_json")
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(resp.SceneJson), &doc); err != nil {
		t.Fatalf("Scene JSON unparseable: %v", err)
	}
	if _, ok := doc["grid"]; !ok {
		t.Fatalf("Scene JSON missing grid key: %v", doc)
	}
	if v, _ := doc["version"].(string); v == "" {
		t.Fatalf("Scene JSON missing version: %v", doc)
	}
}

func TestPrismTwirpServerPlan(t *testing.T) {
	srv := newServer()
	resp, err := srv.Plan(context.Background(), &PlanRequest{Spec: minimalSpec})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if resp.PlanJson == "" {
		t.Fatalf("Plan returned empty plan_json")
	}
	var dag map[string]any
	if err := json.Unmarshal([]byte(resp.PlanJson), &dag); err != nil {
		t.Fatalf("Plan JSON unparseable: %v\n%s", err, resp.PlanJson)
	}
	nodes, ok := dag["nodes"].([]any)
	if !ok || len(nodes) == 0 {
		t.Fatalf("Plan JSON missing or empty nodes: %v", dag)
	}
}

func TestPrismTwirpServerListDatasetsEmpty(t *testing.T) {
	srv := newServer()
	resp, err := srv.ListDatasets(context.Background(), &DatasetsRequest{})
	if err != nil {
		t.Fatalf("ListDatasets: %v", err)
	}
	if len(resp.Datasets) != 0 {
		t.Fatalf("ListDatasets returned %d entries; want 0", len(resp.Datasets))
	}
}

func TestPrismTwirpServerListDatasetsPopulated(t *testing.T) {
	srv := &PrismServer{
		DatasetRegistry: resolve.MapDatasetRegistry{
			"current": "cohorts/q1.pulse",
			"prior":   "cohorts/q4.pulse",
		},
	}
	resp, err := srv.ListDatasets(context.Background(), &DatasetsRequest{})
	if err != nil {
		t.Fatalf("ListDatasets: %v", err)
	}
	if len(resp.Datasets) != 2 {
		t.Fatalf("ListDatasets returned %d entries; want 2", len(resp.Datasets))
	}
	names := []string{resp.Datasets[0].Name, resp.Datasets[1].Name}
	if names[0] != "current" || names[1] != "prior" {
		t.Fatalf("ListDatasets names = %v; want [current prior] (sorted)", names)
	}
}

func TestPrismTwirpServerMissingSpec(t *testing.T) {
	srv := newServer()
	if _, err := srv.Plan(context.Background(), &PlanRequest{}); err == nil {
		t.Fatalf("Plan(empty spec) returned nil error")
	}
	if _, err := srv.Scene(context.Background(), &SceneRequest{}); err == nil {
		t.Fatalf("Scene(empty spec) returned nil error")
	}
	if _, err := srv.Validate(context.Background(), &ValidateRequest{}); err == nil {
		t.Fatalf("Validate(empty spec) returned nil error")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
