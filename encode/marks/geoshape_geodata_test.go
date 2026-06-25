package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/projection"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/geodata"
)

// fakeGeoStore returns a fixed error from both Store methods, letting the
// encode-boundary mapping be exercised without touching the host loader
// or any global geodata state.
type fakeGeoStore struct{ err error }

func (f fakeGeoStore) Lookup(tier geodata.Tier, id string) (*geodata.Feature, error) {
	return nil, f.err
}
func (f fakeGeoStore) Preload(tier geodata.Tier) error { return f.err }

func geoInputs(t *testing.T, store geodata.Store) Inputs {
	t.Helper()
	proj, err := projection.New("mercator", projection.Options{})
	if err != nil {
		t.Fatalf("projection.New: %v", err)
	}
	return Inputs{
		Table:      buildTable(t, map[string]any{"region": []string{"USA"}}),
		Feature:    Channel{Field: "region"},
		Projection: proj,
		GeoStore:   store,
		Layout:     plotRect(),
	}
}

func assertGeoCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", want)
	}
	ae, ok := err.(*prismerrors.AppError)
	if !ok {
		t.Fatalf("err type = %T, want *prismerrors.AppError", err)
	}
	if ae.Code != want {
		t.Fatalf("code = %q, want %q", ae.Code, want)
	}
}

// TestGeoshapeSurfacesDirUnset asserts the geo layer fails loudly with a
// coded error (never a silent skip) when no bundle directory is set.
func TestGeoshapeSurfacesDirUnset(t *testing.T) {
	_, err := encodeGeoshape(geoInputs(t, fakeGeoStore{err: geodata.ErrBundleDirUnset}))
	assertGeoCode(t, err, "PRISM_GEODATA_DIR_UNSET")
}

// TestGeoshapeSurfacesTierMissing asserts a configured-but-missing tier
// file maps to its dedicated code, carrying the probed path in context.
func TestGeoshapeSurfacesTierMissing(t *testing.T) {
	store := fakeGeoStore{err: &geodata.TierMissingError{
		Tier: geodata.TierWorld110m,
		Path: "/geo/world-110m.geo.json",
	}}
	_, err := encodeGeoshape(geoInputs(t, store))
	assertGeoCode(t, err, "PRISM_GEODATA_TIER_MISSING")
}
