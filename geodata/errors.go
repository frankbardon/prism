package geodata

import (
	"errors"
	"fmt"
)

// ErrBundleDirUnset is returned by the host tier loader when no bundle
// directory has been configured (SetHostBundleDir was never called).
// The encode boundary maps it to PRISM_GEODATA_DIR_UNSET so a geo mark
// fails loudly instead of silently skipping its layer.
//
// Defined here (no build tag) so both the host and WASM builds carry the
// symbol; only the host loader returns it.
var ErrBundleDirUnset = errors.New("geodata: host bundle directory not configured")

// TierMissingError is returned by the host tier loader when the bundle
// directory is configured but the requested tier's "<tier>.geo.json"
// file is absent. The encode boundary maps it to
// PRISM_GEODATA_TIER_MISSING. Path is the absolute/relative path that
// was probed; Err is the underlying filesystem error.
type TierMissingError struct {
	Tier Tier
	Path string
	Err  error
}

// Error implements error. The "not found" phrasing is part of the
// loader contract relied upon by callers that inspect the message.
func (e *TierMissingError) Error() string {
	return fmt.Sprintf("geodata: tier %q bundle %s not found: %v", e.Tier, e.Path, e.Err)
}

// Unwrap exposes the underlying filesystem error for errors.Is/As.
func (e *TierMissingError) Unwrap() error { return e.Err }
