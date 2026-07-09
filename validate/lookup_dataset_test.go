package validate_test

import (
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/validate"
)

// TestDatasetLookupSourceBoundIsBestEffortMiss pins the post-E4 behavior:
// DatasetLookup no longer reads a `.pulse` header (the loader was removed),
// so a source-bound dataset carries no inline schema. Schema is a
// best-effort miss — semantic rules skip field-existence checks for that
// dataset rather than firing false positives against a schema Prism can
// no longer read. Register still records the name so the dataset-ref
// rule treats the externally-bound dataset as declared.
func TestDatasetLookupSourceBoundIsBestEffortMiss(t *testing.T) {
	pl := validate.NewDatasetLookup(resolve.New(nil), afero.NewMemMapFs())
	pl.Register("tiny", "tiny")

	if _, ok := pl.Schema("tiny"); ok {
		t.Fatal("Schema(tiny) hit; want best-effort miss (Pulse loader removed in E4)")
	}

	found := false
	for _, n := range pl.Names() {
		if n == "tiny" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Names()=%v, want to include the registered name \"tiny\"", pl.Names())
	}
}

func TestDatasetLookupUnregisteredMisses(t *testing.T) {
	pl := validate.NewDatasetLookup(resolve.New(nil), afero.NewOsFs())
	if _, ok := pl.Schema("nothing"); ok {
		t.Fatal("Schema(nothing) hit; want miss")
	}
}

func TestDatasetLookupCachesMisses(t *testing.T) {
	pl := validate.NewDatasetLookup(resolve.New(nil), afero.NewOsFs())
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
	a.Register("only_in_a", &validate.SchemaShim{
		Name:   "only_in_a",
		Fields: []validate.FieldShim{{Name: "x", Type: "nominal"}},
	})
	b := validate.NewStaticLookup()
	b.Register("only_in_b", &validate.SchemaShim{
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
