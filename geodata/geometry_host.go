//go:build !js && !wasm

package geodata

import (
	_ "embed"
	"fmt"
)

// Host-side embedded bundles. The build pipeline writes these files
// via internal/tools/build_geodata; they are committed to the repo so
// `make build` requires no network.
//
// WASM builds DO NOT embed these — see geometry_wasm.go.

//go:embed world-110m.geo.json
var bundleWorld110m []byte

//go:embed world-50m.geo.json
var bundleWorld50m []byte

//go:embed admin1-50m.geo.json
var bundleAdmin1_50m []byte

func platformTierLoader(tier Tier) ([]byte, error) {
	switch tier {
	case TierWorld110m:
		return bundleWorld110m, nil
	case TierWorld50m:
		return bundleWorld50m, nil
	case TierAdmin1_50m:
		return bundleAdmin1_50m, nil
	default:
		return nil, fmt.Errorf("geodata: unknown tier %q", tier)
	}
}

// EmbeddedTierBytes returns the on-disk bundle bytes for the given
// tier as embedded into the host binary. Host-only (the wasm build
// does not embed; it fetches). Returns an error for unknown tiers.
//
// Consumers (cmd/prism static-bundle) use this to copy the geodata
// artifacts to disk without duplicating //go:embed directives.
func EmbeddedTierBytes(tier Tier) ([]byte, error) {
	return platformTierLoader(tier)
}

// EmbeddedManifestBytes returns the manifest.json bytes as embedded
// at build time. Available on both host and WASM builds because
// manifest.go has no platform constraint.
func EmbeddedManifestBytes() []byte {
	return manifestBytes
}
