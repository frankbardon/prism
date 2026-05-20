package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/twitchtv/twirp"

	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/rpc"
)

// TestPrismTwirpRoundTrip is one of the four PHASE.md-mandated P14
// test gates. For each RPC, stand up an in-process httptest server
// hosting the Twirp handler + ErrorInterceptor, invoke through the
// generated client, and assert the response shape. No actual network
// stack is required (httptest binds 127.0.0.1:0); no out-of-process
// binary is required.
func TestPrismTwirpRoundTrip(t *testing.T) {
	const fixture = `{
      "$schema":"urn:prism:schema:v1:spec",
      "data":{"name":"v","values":[
        {"x":"a","y":1.0},{"x":"b","y":2.0},{"x":"c","y":3.0}
      ]},
      "mark":"bar",
      "encoding":{"x":{"field":"x","type":"nominal"},"y":{"field":"y","type":"quantitative"}}
    }`

	impl := &rpc.PrismServer{
		DatasetRegistry: resolve.MapDatasetRegistry{
			"current": "cohorts/q1.pulse",
		},
		Fs: afero.NewMemMapFs(),
	}
	mux := http.NewServeMux()
	twirpHandler := rpc.NewPrismServer(impl, twirp.WithServerInterceptors(rpc.ErrorInterceptor))
	mux.Handle(rpc.PrismPathPrefix, twirpHandler)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Run("ProtobufClient", func(t *testing.T) {
		runRoundTripSuite(t, rpc.NewPrismProtobufClient(srv.URL, http.DefaultClient), fixture)
	})
	t.Run("JSONClient", func(t *testing.T) {
		runRoundTripSuite(t, rpc.NewPrismJSONClient(srv.URL, http.DefaultClient), fixture)
	})
}

// runRoundTripSuite invokes all five RPCs through the supplied
// generated client. Same assertion set for the protobuf and JSON
// transports — both shapes must round-trip identically.
func runRoundTripSuite(t *testing.T, client rpc.Prism, fixture string) {
	t.Helper()
	ctx := context.Background()

	// 1. Plot
	plot, err := client.Plot(ctx, &rpc.PlotRequest{
		Spec: fixture, Format: "svg", Width: 400, Height: 300,
	})
	if err != nil {
		t.Fatalf("Plot: %v", err)
	}
	if plot.Mime != "image/svg+xml" {
		t.Errorf("Plot mime = %q; want image/svg+xml", plot.Mime)
	}
	if len(plot.Bytes) == 0 {
		t.Errorf("Plot bytes empty")
	}
	if !strings.HasPrefix(strings.TrimSpace(string(plot.Bytes)), "<svg") {
		t.Errorf("Plot bytes do not start with <svg")
	}

	// 2. Validate (ok=true on a valid spec)
	val, err := client.Validate(ctx, &rpc.ValidateRequest{Spec: fixture})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !val.Ok || len(val.Errors) != 0 {
		t.Errorf("Validate ok=%v errors=%v; want ok=true with 0 errors", val.Ok, val.Errors)
	}

	// 3. Scene (returns SceneDoc JSON)
	scene, err := client.Scene(ctx, &rpc.SceneRequest{
		Spec: fixture, Width: 400, Height: 300,
	})
	if err != nil {
		t.Fatalf("Scene: %v", err)
	}
	if scene.SceneJson == "" {
		t.Errorf("Scene scene_json empty")
	}
	var sceneDoc map[string]any
	if err := json.Unmarshal([]byte(scene.SceneJson), &sceneDoc); err != nil {
		t.Errorf("Scene JSON unparseable: %v", err)
	}
	if _, ok := sceneDoc["grid"]; !ok {
		t.Errorf("Scene JSON missing grid key")
	}

	// 4. Plan
	plan, err := client.Plan(ctx, &rpc.PlanRequest{Spec: fixture})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.PlanJson == "" {
		t.Errorf("Plan plan_json empty")
	}
	var planDoc map[string]any
	if err := json.Unmarshal([]byte(plan.PlanJson), &planDoc); err != nil {
		t.Errorf("Plan JSON unparseable: %v", err)
	}
	nodes, _ := planDoc["nodes"].([]any)
	if len(nodes) == 0 {
		t.Errorf("Plan nodes empty")
	}

	// 5. ListDatasets
	ds, err := client.ListDatasets(ctx, &rpc.DatasetsRequest{})
	if err != nil {
		t.Fatalf("ListDatasets: %v", err)
	}
	if len(ds.Datasets) != 1 {
		t.Fatalf("ListDatasets returned %d entries; want 1", len(ds.Datasets))
	}
	if ds.Datasets[0].Name != "current" {
		t.Errorf("ListDatasets[0].Name = %q; want 'current'", ds.Datasets[0].Name)
	}
}

// TestPrismTwirpRoundTripErrorMapping confirms the interceptor's
// status codes surface through the generated client. PNG format
// → twirp.Unimplemented per D085.
func TestPrismTwirpRoundTripErrorMapping(t *testing.T) {
	impl := &rpc.PrismServer{Fs: afero.NewMemMapFs()}
	mux := http.NewServeMux()
	twirpHandler := rpc.NewPrismServer(impl, twirp.WithServerInterceptors(rpc.ErrorInterceptor))
	mux.Handle(rpc.PrismPathPrefix, twirpHandler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := rpc.NewPrismJSONClient(srv.URL, http.DefaultClient)
	_, err := client.Plot(context.Background(), &rpc.PlotRequest{
		Spec: `{"data":{"values":[{"x":1}]},"mark":"bar","encoding":{"x":{"field":"x","type":"quantitative"},"y":{"field":"x","type":"quantitative"}}}`,
		Format: "png",
	})
	if err == nil {
		t.Fatalf("Plot(png) returned nil error; want twirp Unimplemented")
	}
	twerr, ok := err.(twirp.Error)
	if !ok {
		t.Fatalf("Plot(png) returned non-twirp error: %v", err)
	}
	if twerr.Code() != twirp.Unimplemented {
		t.Fatalf("Plot(png) code = %v; want Unimplemented", twerr.Code())
	}
	if twerr.Meta("code") != "PRISM_RENDER_FORMAT_UNAVAILABLE" {
		t.Errorf("meta.code = %q; want PRISM_RENDER_FORMAT_UNAVAILABLE", twerr.Meta("code"))
	}
}
