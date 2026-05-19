package resolve

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/afero"
)

// DatasetRegistry resolves spec-level dataset aliases (`{"data":
// {"name": "current"}}`) to backing refs the resolver understands
// (paths, archive#shard anchors, gs:// urls, cohort:<id>). The
// registry is distinct from the cohort-id Registry above: cohort-ids
// are an internal indirection layer; dataset aliases are user-facing
// names declared in server config or the spec's `datasets` block.
//
// D008 documents the combined client + server registry strategy.
// P07's loader handles the server-side half (JSON file + env var);
// the browser-side `<prism-dataset>` mirror lands in P12.
type DatasetRegistry interface {
	// Resolve returns the backing ref for alias (path, anchor, gs://,
	// or cohort:<id>). The second return is false when the alias is
	// not registered.
	Resolve(alias string) (string, bool)
}

// MapDatasetRegistry is the trivial in-memory implementation. The zero
// value is an empty registry; construction is `MapDatasetRegistry{...}`
// or `LoadDatasetRegistryFile`.
type MapDatasetRegistry map[string]string

// Resolve implements DatasetRegistry.
func (m MapDatasetRegistry) Resolve(alias string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[alias]
	return v, ok
}

// EmptyDatasetRegistry rejects every Resolve. Useful when callers want
// to declare "no registry" without a nil check at every site.
type EmptyDatasetRegistry struct{}

// Resolve implements DatasetRegistry.
func (EmptyDatasetRegistry) Resolve(string) (string, bool) { return "", false }

// LoadDatasetRegistryFile parses a JSON file of shape
//
//	{"datasets": {"current": "cohorts/q1.pulse",
//	              "prior":   "cohorts/q4.pulse"}}
//
// into a MapDatasetRegistry. YAML support is a documented TODO (D048):
// Pulse's go.mod does not ship a YAML loader and the dep-parity rule
// (Rule 13) forbids adding one in P07.
//
// Returns an empty registry (not nil) when the file is absent so
// callers can chain it through ChainDatasetRegistries unconditionally.
func LoadDatasetRegistryFile(path string, fs afero.Fs) (DatasetRegistry, error) {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		if os.IsNotExist(err) {
			return MapDatasetRegistry{}, nil
		}
		return nil, fmt.Errorf("load dataset registry %s: %w", path, err)
	}
	var doc struct {
		Datasets map[string]string `json:"datasets"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse dataset registry %s: %w", path, err)
	}
	out := MapDatasetRegistry{}
	for k, v := range doc.Datasets {
		out[k] = v
	}
	return out, nil
}

// EnvDatasetVar is the env var consulted by LoadDatasetRegistryEnv.
const EnvDatasetVar = "PRISM_DATASETS"

// LoadDatasetRegistryEnv parses comma-separated `name=path` pairs from
// PRISM_DATASETS. Malformed entries (missing `=`, empty name, empty
// path) are silently dropped — callers can post-validate via
// `len(registry)` if they want to surface a config error.
func LoadDatasetRegistryEnv() DatasetRegistry {
	raw, ok := os.LookupEnv(EnvDatasetVar)
	if !ok || raw == "" {
		return MapDatasetRegistry{}
	}
	out := MapDatasetRegistry{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		eq := strings.IndexByte(entry, '=')
		if eq <= 0 || eq == len(entry)-1 {
			continue
		}
		name := strings.TrimSpace(entry[:eq])
		path := strings.TrimSpace(entry[eq+1:])
		if name == "" || path == "" {
			continue
		}
		out[name] = path
	}
	return out
}

// ChainDatasetRegistries walks the supplied registries in order and
// returns the first hit per alias. Layers are tried left-to-right, so
// callers pass the highest-priority registry first. Nil entries are
// skipped.
func ChainDatasetRegistries(layers ...DatasetRegistry) DatasetRegistry {
	cleaned := make([]DatasetRegistry, 0, len(layers))
	for _, r := range layers {
		if r != nil {
			cleaned = append(cleaned, r)
		}
	}
	if len(cleaned) == 0 {
		return EmptyDatasetRegistry{}
	}
	if len(cleaned) == 1 {
		return cleaned[0]
	}
	return chainedDatasetRegistry(cleaned)
}

type chainedDatasetRegistry []DatasetRegistry

func (c chainedDatasetRegistry) Resolve(alias string) (string, bool) {
	for _, r := range c {
		if v, ok := r.Resolve(alias); ok {
			return v, true
		}
	}
	return "", false
}
