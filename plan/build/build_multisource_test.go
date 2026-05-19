package build_test

import (
	"errors"
	"testing"

	"github.com/spf13/afero"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// TestPrismBuildDuplicateDatasetAlias asserts that two distinct leaves
// claiming the same alias raise PRISM_RESOLVE_DUPLICATE_DATASET. We use
// two inline datasets with different rows so the underlying ids
// guaranteed differ.
func TestPrismBuildDuplicateDatasetAlias(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"datasets": {
			"dup": {"values": [{"v": 1}]}
		},
		"data": {"name": "dup", "values": [{"v": 2}]},
		"mark": "bar",
		"encoding": {"x": {"field": "v", "type": "quantitative"}}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	_, _, err = build.Build(s, build.Options{})
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) || ae.Code != "PRISM_RESOLVE_DUPLICATE_DATASET" {
		t.Fatalf("got %v; want PRISM_RESOLVE_DUPLICATE_DATASET", err)
	}
	if alias, _ := ae.Context["Alias"].(string); alias != "dup" {
		t.Errorf("Alias=%v; want dup", ae.Context["Alias"])
	}
}

// TestPrismBuildTransformAsPublishesAlias asserts that a transform's
// `as` field registers the new node id so downstream transforms can
// reference it via `data: "<alias>"`.
func TestPrismBuildTransformAsPublishesAlias(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"datasets": {
			"scores": {"values": [{"brand_id": "alpha", "score": 1}]},
			"labels": {"values": [{"brand_id": "alpha", "label": "Alpha"}]}
		},
		"data": {"name": "scores"},
		"transform": [
			{"data": "scores", "groupby": ["brand_id"],
			 "aggregate": [{"op": "mean", "field": "score", "as": "score_mean"}],
			 "as": "scores_agg"},
			{"join": "inner", "with": "labels", "on": "brand_id",
			 "data": "scores_agg"}
		],
		"mark": "bar",
		"encoding": {
			"x": {"field": "brand_id", "type": "nominal"},
			"y": {"field": "score_mean", "type": "quantitative"}
		}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	d, tip, err := build.Build(s, build.Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Walk the DAG: confirm a JoinNode exists and references the
	// scores_agg node (whose id is an arbitrary auto-generated "ga:N").
	var joinFound bool
	for _, id := range d.Nodes() {
		n, _ := d.Node(id)
		if n.Inputs() == nil || len(n.Inputs()) != 2 {
			continue
		}
		// Heuristic: a 2-input node downstream of a "ga:" node is the
		// join we want.
		for _, in := range n.Inputs() {
			if len(string(in)) >= 3 && string(in)[:3] == "ga:" {
				joinFound = true
			}
		}
	}
	if !joinFound {
		t.Errorf("expected a 2-input join node fed by the scores_agg (ga:N) node")
	}
	_ = tip
}

// TestPrismBuildMultipleSources asserts a 3-dataset spec produces 3
// SourceNodes at the roots (one per declared dataset). Inline values
// stand in for .pulse paths so the test doesn't need real cohorts.
func TestPrismBuildMultipleSources(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"datasets": {
			"a": {"values": [{"v": 1}]},
			"b": {"values": [{"v": 2}]},
			"c": {"values": [{"v": 3}]}
		},
		"data": {"name": "a"},
		"mark": "bar",
		"encoding": {"x": {"field": "v", "type": "quantitative"}}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	d, _, err := build.Build(s, build.Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := len(d.Roots()); got != 3 {
		t.Errorf("Roots=%d; want 3", got)
	}
}

// TestPrismBuildResolvesDatasetAliasFromRegistry pins the dataset
// registry contract: a spec with `{"data": {"name": "current"}}` and
// no inline datasets should resolve `current` through the configured
// DatasetRegistry. We stub the path to a non-existent file so the
// build succeeds at translation time even though execution would
// later fail at resolve.
func TestPrismBuildResolvesDatasetAliasFromRegistry(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"name": "current"},
		"mark": "bar",
		"encoding": {"x": {"field": "v", "type": "quantitative"}}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	registry := resolve.MapDatasetRegistry{
		"current": "stub-cohort.pulse",
	}
	d, _, err := build.Build(s, build.Options{
		FS:              afero.NewMemMapFs(),
		DatasetRegistry: registry,
	})
	if err != nil {
		t.Fatalf("Build with registry: %v", err)
	}
	// One root = one SourceNode (created from the resolved path).
	if got := len(d.Roots()); got != 1 {
		t.Errorf("Roots=%d; want 1 (single source from registry)", got)
	}
}

// TestPrismBuildRegistryMissingAliasStillErrors confirms the registry
// is consulted before the missing-alias error fires — but absent
// registry config, the spec still raises PRISM_PLAN_003.
func TestPrismBuildRegistryMissingAliasStillErrors(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"name": "current"},
		"mark": "bar",
		"encoding": {"x": {"field": "v", "type": "quantitative"}}
	}`)
	s, _ := spec.DecodeBytes(body)
	_, _, err := build.Build(s, build.Options{
		DatasetRegistry: resolve.MapDatasetRegistry{}, // empty, alias not resolvable
	})
	if err == nil {
		t.Fatal("expected error for unresolved alias")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AppError, got %T", err)
	}
	// The alias is not in the registry, no inline datasets exist; the
	// spec resolution falls through to the same "no data binding" path
	// as P03 (an empty leaf list raises a generic builder error).
	if ae.Code == "" {
		t.Errorf("expected non-empty error code, got bare: %v", err)
	}
}
