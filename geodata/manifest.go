package geodata

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

//go:embed manifest.json
var manifestBytes []byte

var (
	manifestOnce   sync.Once
	manifestLoaded *Manifest
	manifestErr    error
)

// LoadManifest returns the embedded manifest, parsing it on first call.
// Safe for concurrent use. Subsequent calls return the cached pointer.
func LoadManifest() (*Manifest, error) {
	manifestOnce.Do(func() {
		if len(manifestBytes) == 0 {
			manifestErr = fmt.Errorf("geodata: embedded manifest is empty (run `make geodata`)")
			return
		}
		m := &Manifest{}
		if err := json.Unmarshal(manifestBytes, m); err != nil {
			manifestErr = fmt.Errorf("geodata: decode manifest: %w", err)
			return
		}
		if m.Features == nil {
			m.Features = map[string]*FeatureMeta{}
		}
		manifestLoaded = m
	})
	return manifestLoaded, manifestErr
}

// MustLoadManifest panics on failure. Intended for package init paths
// that cannot recover (the embedded manifest is supposed to be
// well-formed; if it isn't, the build is broken).
func MustLoadManifest() *Manifest {
	m, err := LoadManifest()
	if err != nil {
		panic(err)
	}
	return m
}

// FeatureIDs returns every feature ID in the manifest, sorted. Useful
// for validation error messages ("did you mean ...").
func (m *Manifest) FeatureIDs() []string {
	out := make([]string, 0, len(m.Features))
	for id := range m.Features {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// FeatureIDsForTier returns the sorted IDs that belong to the given
// tier. Useful for choropleth fixtures and for trimming the search
// space in error suggestions.
func (m *Manifest) FeatureIDsForTier(t Tier) []string {
	out := make([]string, 0)
	for id, f := range m.Features {
		if f.Tier == t {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// Has reports whether id is present in the manifest.
func (m *Manifest) Has(id string) bool {
	_, ok := m.Features[id]
	return ok
}

// Lookup returns the metadata for id, or nil + false.
func (m *Manifest) Lookup(id string) (*FeatureMeta, bool) {
	f, ok := m.Features[id]
	return f, ok
}
