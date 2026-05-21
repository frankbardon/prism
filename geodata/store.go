package geodata

import (
	"fmt"
	"sync"
)

// Store resolves a feature ID to its raw lon/lat geometry. The host
// implementation reads from embedded TopoJSON; the WASM implementation
// fetches from `prism static-bundle` on first use. Both are safe for
// concurrent use after primer initialisation.
type Store interface {
	// Lookup returns the feature for id at the requested tier. When the
	// tier archive is not loaded yet the implementation loads it
	// (host: decode embed; wasm: fetch + decode) and caches the result.
	Lookup(tier Tier, id string) (*Feature, error)
	// Preload eagerly decodes the tier. Optional; Lookup pulls in
	// lazily. Useful in renderers that know they'll need many features
	// from the same tier.
	Preload(tier Tier) error
}

// memoryStore is the shared in-memory cache + decoder used by both
// host and WASM builds. The embed/fetch source is plugged via the
// tierLoader callback so the host build can hand it `tierEmbedBytes`
// and the WASM build can hand it the same-origin fetcher.
type memoryStore struct {
	loader     tierLoader
	mu         sync.RWMutex
	decoded    map[Tier]map[string]*Feature
	decodeErrs map[Tier]error
}

// tierLoader returns the raw bundle bytes for a tier. Implementations
// are platform-specific (embed vs fetch). The bytes follow the
// Prism geo-bundle format documented in decoder.go.
type tierLoader func(tier Tier) ([]byte, error)

func newMemoryStore(loader tierLoader) *memoryStore {
	return &memoryStore{
		loader:     loader,
		decoded:    map[Tier]map[string]*Feature{},
		decodeErrs: map[Tier]error{},
	}
}

// Lookup implements Store.
func (s *memoryStore) Lookup(tier Tier, id string) (*Feature, error) {
	s.mu.RLock()
	if features, ok := s.decoded[tier]; ok {
		s.mu.RUnlock()
		f, ok := features[id]
		if !ok {
			return nil, fmt.Errorf("geodata: feature %q not in tier %q", id, tier)
		}
		return f, nil
	}
	if err, ok := s.decodeErrs[tier]; ok {
		s.mu.RUnlock()
		return nil, err
	}
	s.mu.RUnlock()

	if err := s.loadTier(tier); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.decoded[tier][id]
	if !ok {
		return nil, fmt.Errorf("geodata: feature %q not in tier %q", id, tier)
	}
	return f, nil
}

// Preload implements Store.
func (s *memoryStore) Preload(tier Tier) error {
	s.mu.RLock()
	_, loaded := s.decoded[tier]
	cachedErr, hasErr := s.decodeErrs[tier]
	s.mu.RUnlock()
	if loaded {
		return nil
	}
	if hasErr {
		return cachedErr
	}
	return s.loadTier(tier)
}

func (s *memoryStore) loadTier(tier Tier) error {
	raw, err := s.loader(tier)
	if err != nil {
		s.mu.Lock()
		s.decodeErrs[tier] = err
		s.mu.Unlock()
		return err
	}
	features, err := decodeBundle(raw)
	if err != nil {
		err = fmt.Errorf("geodata: decode tier %q: %w", tier, err)
		s.mu.Lock()
		s.decodeErrs[tier] = err
		s.mu.Unlock()
		return err
	}
	s.mu.Lock()
	s.decoded[tier] = features
	s.mu.Unlock()
	return nil
}

// DefaultStore returns the platform-default Store. On host builds it
// reads from embedded TopoJSON; on WASM it fetches from the static
// bundle origin set via SetWasmBundleURL.
func DefaultStore() Store {
	return defaultStoreOnce()
}

var (
	defaultStore     Store
	defaultStoreInit sync.Once
)

func defaultStoreOnce() Store {
	defaultStoreInit.Do(func() {
		defaultStore = newMemoryStore(platformTierLoader)
	})
	return defaultStore
}
