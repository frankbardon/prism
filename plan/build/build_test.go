package build_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/spec"
)

// repoRoot walks up from this test file until it finds go.mod. Used to
// locate testdata/specs/* regardless of where `go test` is invoked.
func repoRoot(t *testing.T) string {
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

func loadSpec(t *testing.T, path string) *spec.Spec {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return s
}

func TestPrismDAGBuildSingleSource(t *testing.T) {
	root := repoRoot(t)
	s := loadSpec(t, filepath.Join(root, "testdata", "specs", "bar_basic.json"))
	d, tip, err := build.Build(s, build.Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.Size() < 1 {
		t.Errorf("Size=%d, want >=1 (one source)", d.Size())
	}
	if len(d.Roots()) == 0 {
		t.Error("no roots")
	}
	if len(d.Sinks()) != 1 {
		t.Errorf("Sinks=%v, want 1", d.Sinks())
	}
	if tip == "" {
		t.Error("Build returned empty tip id")
	}
	if d.Sinks()[0] != tip {
		t.Errorf("Sinks[0]=%q, want tip=%q", d.Sinks()[0], tip)
	}
}

func TestPrismDAGBuildAllFixtures(t *testing.T) {
	// P08 unskipped layer + concat / hconcat / vconcat; P09 unskipped
	// facet / repeat. P13 unskipped selection (the builder pipes the
	// selection block to the encoder; no DAG nodes are required per
	// D004). No deferrals remain.
	skip := map[string]bool{}

	root := repoRoot(t)
	dir := filepath.Join(root, "testdata", "specs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("no spec fixtures discovered")
	}

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			if skip[name] {
				t.Skipf("composition/selection: deferred to P09/P13")
			}
			s := loadSpec(t, filepath.Join(dir, name))
			// Composite specs (layer/concat/hconcat/vconcat) build via
			// BuildComposite per D049/D050; each child must produce a
			// non-empty sub-DAG.
			if build.IsComposite(s) {
				c, err := build.BuildComposite(s, build.Options{})
				if err != nil {
					t.Fatalf("BuildComposite: %v", err)
				}
				if len(c.Children) == 0 {
					t.Fatal("composite has no children")
				}
				for i, child := range c.Children {
					if child.DAG == nil || child.DAG.Size() == 0 {
						t.Errorf("child %d: DAG empty", i)
					}
					if len(child.DAG.Roots()) == 0 {
						t.Errorf("child %d: no roots", i)
					}
					if len(child.DAG.Sinks()) == 0 {
						t.Errorf("child %d: no sinks", i)
					}
				}
				return
			}
			d, _, err := build.Build(s, build.Options{})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if d.Size() == 0 {
				t.Error("DAG is empty")
			}
			if len(d.Roots()) == 0 {
				t.Error("DAG has no roots")
			}
			if len(d.Sinks()) == 0 {
				t.Error("DAG has no sinks")
			}
		})
	}
}

// TestPrismDAGBuildSelectionFixturesPipe through P13 — the previously
// rejected selection_*.json fixtures now build through Build() because
// selections flow straight to the encoder per D004. This test pins
// the new positive contract.
func TestPrismDAGBuildSelectionFixturesPipe(t *testing.T) {
	cases := []string{
		"selection_point.json",
		"selection_interval.json",
	}
	root := repoRoot(t)
	for _, name := range cases {
		name := name
		t.Run(name, func(t *testing.T) {
			s := loadSpec(t, filepath.Join(root, "testdata", "specs", name))
			d, tip, err := build.Build(s, build.Options{})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			if d == nil || d.Size() == 0 {
				t.Fatal("DAG empty after Build")
			}
			if tip == "" {
				t.Fatal("tip id empty")
			}
		})
	}
}

func TestPrismDAGBuildMissingDataset(t *testing.T) {
	body := []byte(`{
		"$schema": "urn:prism:schema:v1:spec",
		"data": {"name": "missing_dataset"},
		"mark": "bar",
		"encoding": {"x": {"field": "v", "type": "quantitative"}}
	}`)
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	_, _, err = build.Build(s, build.Options{})
	if err == nil {
		t.Fatal("expected PRISM_PLAN_003, got nil")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AppError, got %T", err)
	}
	if ae.Code != "PRISM_PLAN_003" {
		t.Errorf("Code=%q, want PRISM_PLAN_003", ae.Code)
	}
}

// jsonProbe is a defensive sanity-check that the spec decoder is wired —
// catches go.mod/embed regressions before we trip on them upstream.
func TestPrismBuildFixtureDecodes(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "testdata", "specs", "bar_basic.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var probe map[string]any
	if err := json.Unmarshal(body, &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if probe["mark"] != "bar" {
		t.Fatalf("expected bar mark, got %v", probe["mark"])
	}
}
