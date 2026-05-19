package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// buildIncompatibleLayerSpec is a tiny constructor used by the
// happy-path / negative-path tests; channel arg picks which axis
// gets the mismatched type.
func buildIncompatibleLayerSpec(resolveMode, layer0Type, layer1Type string) *spec.Spec {
	return &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Resolve: &spec.Resolve{
			Scale: &spec.ResolveChannelMap{Y: resolveMode},
		},
		Layer: []*spec.Spec{
			{
				Schema: "urn:prism:schema:v1:spec",
				Data: &spec.Data{Values: []map[string]any{
					{"x": "a", "y": "alpha"},
				}},
				Mark: &spec.Mark{Shorthand: "point"},
				Encoding: &spec.Encoding{
					X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "x", Type: "nominal"}},
					Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "y", Type: layer0Type}},
				},
			},
			{
				Schema: "urn:prism:schema:v1:spec",
				Data: &spec.Data{Values: []map[string]any{
					{"x": "a", "y": 1.0},
				}},
				Mark: &spec.Mark{Shorthand: "line"},
				Encoding: &spec.Encoding{
					X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "x", Type: "nominal"}},
					Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "y", Type: layer1Type}},
				},
			},
		},
	}
}

func TestPrismResolveScaleCompatRaisesOnSharedMixed(t *testing.T) {
	s := buildIncompatibleLayerSpec("shared", "nominal", "quantitative")
	errs := ResolveScaleCompat{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_PLAN_005" {
		t.Fatalf("expected one PRISM_PLAN_005, got: %+v", errs)
	}
}

func TestPrismResolveScaleCompatPassesOnSharedConsistent(t *testing.T) {
	s := buildIncompatibleLayerSpec("shared", "quantitative", "quantitative")
	errs := ResolveScaleCompat{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors on consistent shared types, got: %+v", errs)
	}
}

func TestPrismResolveScaleCompatPassesOnIndependentMixed(t *testing.T) {
	// Same incompatibility, but resolve mode is independent → OK.
	s := buildIncompatibleLayerSpec("independent", "nominal", "quantitative")
	errs := ResolveScaleCompat{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors when independent, got: %+v", errs)
	}
}

func TestPrismResolveScaleCompatSkipsImplicitTypes(t *testing.T) {
	// One layer omits encoding.y.type → rule defers to runtime encoder
	// inference, so no static error fires here.
	s := buildIncompatibleLayerSpec("shared", "", "quantitative")
	errs := ResolveScaleCompat{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 0 {
		t.Fatalf("expected no static error when type implicit, got: %+v", errs)
	}
}

// TestPrismResolveScaleCompatFixtureFires loads the new invalid
// fixture and walks it through the full SemanticValidator surface
// (mirroring how the CLI calls validate).
func TestPrismResolveScaleCompatFixtureFires(t *testing.T) {
	root := repoRootRules(t)
	path := filepath.Join(root, "testdata", "specs", "invalid", "layer_shared_y_incompatible.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	errs := ResolveScaleCompat{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_PLAN_005" {
		t.Fatalf("expected one PRISM_PLAN_005, got: %+v", errs)
	}
	if got := errs[0].Context["Channel"]; got != "y" {
		t.Errorf("Channel=%v, want y", got)
	}
}

func repoRootRules(t *testing.T) string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	dir := filepath.Dir(here)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", here)
		}
		dir = parent
	}
}
