package validate_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/validate"
)

func fixturePath(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Join(filepath.Dir(here), "..")
	return filepath.Join(root, "testdata", "cohorts", "tiny.pulse")
}

func TestPulseLookupSchemaResolvesRealCohort(t *testing.T) {
	path := fixturePath(t)
	pl := validate.NewPulseLookup(resolve.New(nil), afero.NewOsFs())
	pl.Register("tiny", path)

	shim, ok := pl.Schema("tiny")
	if !ok {
		t.Fatal("Schema(tiny) miss; want hit")
	}
	if shim.Name != "tiny" {
		t.Fatalf("shim.Name = %q, want tiny", shim.Name)
	}

	wantTypes := map[string]string{
		"brand_id": "nominal",
		"score":    "quantitative",
		"age":      "quantitative",
	}
	if len(shim.Fields) != len(wantTypes) {
		t.Fatalf("Fields len = %d, want %d (%+v)", len(shim.Fields), len(wantTypes), shim.Fields)
	}
	for _, f := range shim.Fields {
		want, ok := wantTypes[f.Name]
		if !ok {
			t.Fatalf("unexpected field %q", f.Name)
		}
		if f.Type != want {
			t.Fatalf("field %q type = %q, want %q", f.Name, f.Type, want)
		}
	}
}

func TestPulseLookupUnregisteredMisses(t *testing.T) {
	pl := validate.NewPulseLookup(resolve.New(nil), afero.NewOsFs())
	if _, ok := pl.Schema("nothing"); ok {
		t.Fatal("Schema(nothing) hit; want miss")
	}
}

func TestPulseLookupCachesMisses(t *testing.T) {
	pl := validate.NewPulseLookup(resolve.New(nil), afero.NewOsFs())
	pl.Register("ghost", "definitely_not_a_real_path.pulse")
	if _, ok := pl.Schema("ghost"); ok {
		t.Fatal("first Schema(ghost) unexpectedly hit")
	}
	// Second call should hit the negative cache and not retry the resolve.
	if _, ok := pl.Schema("ghost"); ok {
		t.Fatal("second Schema(ghost) unexpectedly hit")
	}
}

func TestCompositeLookupOrder(t *testing.T) {
	a := validate.NewStaticLookup()
	a.Register("only_in_a", &validate.PulseSchemaShim{
		Name:   "only_in_a",
		Fields: []validate.FieldShim{{Name: "x", Type: "nominal"}},
	})
	b := validate.NewStaticLookup()
	b.Register("only_in_b", &validate.PulseSchemaShim{
		Name:   "only_in_b",
		Fields: []validate.FieldShim{{Name: "y", Type: "quantitative"}},
	})
	composite := validate.NewCompositeLookup(a, b)

	if shim, ok := composite.Schema("only_in_a"); !ok || shim.Fields[0].Name != "x" {
		t.Fatalf("only_in_a not resolved: %v %v", shim, ok)
	}
	if shim, ok := composite.Schema("only_in_b"); !ok || shim.Fields[0].Name != "y" {
		t.Fatalf("only_in_b not resolved: %v %v", shim, ok)
	}
	if _, ok := composite.Schema("missing"); ok {
		t.Fatal("missing unexpectedly resolved")
	}
}
