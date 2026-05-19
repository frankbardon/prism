package resolve

import (
	"testing"

	"github.com/spf13/afero"
)

func TestPrismDatasetRegistryFileLoad(t *testing.T) {
	fs := afero.NewMemMapFs()
	_ = afero.WriteFile(fs, "/cfg/datasets.json", []byte(`{
		"datasets": {
			"current": "cohorts/q1.pulse",
			"prior":   "cohorts/q4.pulse"
		}
	}`), 0o644)
	r, err := LoadDatasetRegistryFile("/cfg/datasets.json", fs)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v, ok := r.Resolve("current"); !ok || v != "cohorts/q1.pulse" {
		t.Errorf("Resolve(current)=(%q,%v); want (cohorts/q1.pulse,true)", v, ok)
	}
	if v, ok := r.Resolve("prior"); !ok || v != "cohorts/q4.pulse" {
		t.Errorf("Resolve(prior)=(%q,%v)", v, ok)
	}
	if _, ok := r.Resolve("missing"); ok {
		t.Errorf("Resolve(missing) should be false")
	}
}

func TestPrismDatasetRegistryFileMissingIsEmpty(t *testing.T) {
	fs := afero.NewMemMapFs()
	r, err := LoadDatasetRegistryFile("/nonexistent.json", fs)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := r.Resolve("anything"); ok {
		t.Errorf("empty registry should not resolve")
	}
}

func TestPrismDatasetRegistryEnvLoad(t *testing.T) {
	t.Setenv("PRISM_DATASETS", "a=cohorts/a.pulse,b=cohorts/b.pulse,broken,=onlyrhs,c=,d=cohorts/d.pulse")
	r := LoadDatasetRegistryEnv()
	for k, want := range map[string]string{
		"a": "cohorts/a.pulse",
		"b": "cohorts/b.pulse",
		"d": "cohorts/d.pulse",
	} {
		v, ok := r.Resolve(k)
		if !ok || v != want {
			t.Errorf("Resolve(%q)=(%q,%v); want (%q,true)", k, v, ok, want)
		}
	}
	for _, k := range []string{"broken", "", "c", "onlyrhs"} {
		if _, ok := r.Resolve(k); ok {
			t.Errorf("Resolve(%q) should be false", k)
		}
	}
}

func TestPrismDatasetRegistryChainPrecedence(t *testing.T) {
	high := MapDatasetRegistry{"x": "first", "y": "first-y"}
	low := MapDatasetRegistry{"x": "second", "z": "second-z"}
	r := ChainDatasetRegistries(high, low)
	if v, _ := r.Resolve("x"); v != "first" {
		t.Errorf("x=%q; want first (highest priority wins)", v)
	}
	if v, _ := r.Resolve("y"); v != "first-y" {
		t.Errorf("y=%q; want first-y", v)
	}
	if v, _ := r.Resolve("z"); v != "second-z" {
		t.Errorf("z=%q; want second-z (low layer reachable)", v)
	}
	if _, ok := r.Resolve("missing"); ok {
		t.Errorf("missing should be false")
	}
}

func TestPrismDatasetRegistryChainSkipsNil(t *testing.T) {
	r := ChainDatasetRegistries(nil, MapDatasetRegistry{"a": "1"}, nil)
	if v, _ := r.Resolve("a"); v != "1" {
		t.Errorf("nil skip failed: a=%q", v)
	}
}
