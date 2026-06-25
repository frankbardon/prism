package geodata

import (
	"errors"
	"testing"
)

// TestLoaderReturnsErrBundleDirUnset verifies the host loader returns the
// shared sentinel (not just a substring) so the encode boundary can map
// it to PRISM_GEODATA_DIR_UNSET via errors.Is.
func TestLoaderReturnsErrBundleDirUnset(t *testing.T) {
	withHostFs(t, "")
	_, err := platformTierLoader(TierWorld110m)
	if !errors.Is(err, ErrBundleDirUnset) {
		t.Fatalf("err = %v, want errors.Is(ErrBundleDirUnset)", err)
	}
}

// TestLoaderReturnsTierMissingError verifies the host loader returns a
// typed *TierMissingError carrying the probed tier + path so the encode
// boundary can map it to PRISM_GEODATA_TIER_MISSING via errors.As.
func TestLoaderReturnsTierMissingError(t *testing.T) {
	withHostFs(t, "/geo") // empty fs, no tier files written
	_, err := platformTierLoader(TierWorld110m)
	var missing *TierMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %v, want errors.As(*TierMissingError)", err)
	}
	if missing.Tier != TierWorld110m {
		t.Fatalf("missing.Tier = %q, want %q", missing.Tier, TierWorld110m)
	}
	if missing.Path == "" {
		t.Fatal("missing.Path is empty, want the probed <dir>/<tier>.geo.json path")
	}
}
