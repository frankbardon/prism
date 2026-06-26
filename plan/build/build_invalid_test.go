package build_test

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/build"
)

// TestPrismDAGBuildInvalidGallery walks every fixture under
// examples/specs/invalid/. Most are validator-gate failures
// (PRISM_SPEC_*) that the builder does NOT re-check — they should
// still produce a non-empty DAG. The one builder-gate failure is
// dataset_undefined.json which raises PRISM_PLAN_003.
//
// The split is intentional: the validator owns shape and semantic
// correctness; the builder owns plan-time structural correctness
// (cycles, missing dataset refs, unsupported features). A future
// regression that conflates the two would surface here.
func TestPrismDAGBuildInvalidGallery(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "examples", "specs", "invalid")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	// Builder-gate failures (the spec is structurally invalid in a
	// way the planner cares about; validator may or may not also
	// catch it).
	builderGate := map[string]string{
		"dataset_undefined.json": "PRISM_PLAN_003",
	}

	// Composite-spec negatives belong to BuildComposite + the validate
	// layer; flat Build cannot exercise them. List them so the
	// validator-gate sweep does not flake on the composite-shape error
	// that flat Build returns.
	skip := map[string]bool{
		"layer_shared_y_incompatible.json": true,
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			if skip[name] {
				t.Skip("skipped")
			}
			s := loadSpec(t, filepath.Join(dir, name))
			d, _, err := build.Build(s, build.Options{})
			if wantCode, isBuilderGate := builderGate[name]; isBuilderGate {
				if err == nil {
					t.Fatalf("expected %s, got nil error", wantCode)
				}
				var ae *prismerrors.AppError
				if !errors.As(err, &ae) || ae.Code != wantCode {
					t.Errorf("expected %s, got %v", wantCode, err)
				}
				return
			}
			// Validator-gate failure: builder should succeed.
			if err != nil {
				t.Fatalf("validator-gate fixture %s: builder returned %v (expected build to succeed; semantic checks belong to validate)", name, err)
			}
			if d == nil || d.Size() == 0 {
				t.Error("DAG empty")
			}
		})
	}
}
