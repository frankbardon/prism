package geodata

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// TestMain points the host bundle loader at the package directory, where
// the committed *.geo.json tier files live. The host build no longer
// embeds them, so DefaultStore() needs an explicit directory.
func TestMain(m *testing.M) {
	SetHostBundleDir(".")
	os.Exit(m.Run())
}

func TestManifestLoad(t *testing.T) {
	m, err := LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Version != 1 {
		t.Fatalf("version = %d, want 1", m.Version)
	}
	if !m.Has("USA") {
		t.Fatal("manifest missing USA")
	}
	if !m.Has("US-CA") {
		t.Fatal("manifest missing US-CA")
	}
	if got := m.Features["USA"].Tier; got != TierWorld110m {
		t.Fatalf("USA tier = %q, want %q", got, TierWorld110m)
	}
	if got := m.Features["US-CA"].Parent; got != "USA" {
		t.Fatalf("US-CA parent = %q, want USA", got)
	}
}

func TestStoreLookup(t *testing.T) {
	store := DefaultStore()
	f, err := store.Lookup(TierWorld110m, "USA")
	if err != nil {
		t.Fatalf("Lookup USA: %v", err)
	}
	if f.ID != "USA" {
		t.Fatalf("ID = %q, want USA", f.ID)
	}
	if len(f.Polygons) == 0 {
		t.Fatal("USA has no polygons")
	}
	if len(f.Polygons[0].Outer) < 3 {
		t.Fatalf("USA outer ring has %d points, want >=3", len(f.Polygons[0].Outer))
	}
	// Dequantization sanity: first point should be in lon/lat degrees,
	// not the quantized integer form.
	first := f.Polygons[0].Outer[0]
	if first[0] < -180 || first[0] > 180 || first[1] < -90 || first[1] > 90 {
		t.Fatalf("first point out of [-180,180]/[-90,90]: %v", first)
	}
}

func TestStoreLookupMissing(t *testing.T) {
	store := DefaultStore()
	if _, err := store.Lookup(TierWorld110m, "ZZZ"); err == nil {
		t.Fatal("expected error for unknown feature")
	}
}

func TestStoreAdmin1(t *testing.T) {
	store := DefaultStore()
	f, err := store.Lookup(TierAdmin1_50m, "US-CA")
	if err != nil {
		t.Fatalf("Lookup US-CA: %v", err)
	}
	if len(f.Polygons) == 0 || len(f.Polygons[0].Outer) < 3 {
		t.Fatal("US-CA has insufficient geometry")
	}
}

// withHostFs swaps the host bundle filesystem + directory for the
// duration of a test, restoring the originals afterwards. Returns a
// fresh in-memory afero.Fs to seed.
func withHostFs(t *testing.T, dir string) afero.Fs {
	t.Helper()
	hostBundleMu.RLock()
	prevFs := hostBundleFs
	prevDir := hostBundleDir
	hostBundleMu.RUnlock()
	t.Cleanup(func() {
		hostBundleMu.Lock()
		hostBundleFs = prevFs
		hostBundleDir = prevDir
		hostBundleMu.Unlock()
	})
	fs := afero.NewMemMapFs()
	hostBundleMu.Lock()
	hostBundleFs = fs
	hostBundleDir = dir
	hostBundleMu.Unlock()
	return fs
}

// TestDirLoaderReadsViaAfero verifies the host loader reads tier bytes
// through the injected afero.Fs at "<dir>/<tier>.geo.json".
func TestDirLoaderReadsViaAfero(t *testing.T) {
	fs := withHostFs(t, "/geo")
	want := []byte("raw-bundle-bytes")
	if err := afero.WriteFile(fs, "/geo/world-110m.geo.json", want, 0o644); err != nil {
		t.Fatalf("seed tier file: %v", err)
	}
	got, err := platformTierLoader(TierWorld110m)
	if err != nil {
		t.Fatalf("platformTierLoader: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("bytes = %q, want %q", got, want)
	}
}

// TestDirLoaderUnsetDir distinguishes the unconfigured-directory case so
// a later story can map it to a dedicated error code.
func TestDirLoaderUnsetDir(t *testing.T) {
	withHostFs(t, "")
	_, err := platformTierLoader(TierWorld110m)
	if err == nil {
		t.Fatal("expected error when host bundle dir is unset")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("error = %v, want it to mention an unconfigured directory", err)
	}
}

// TestDirLoaderMissingTier distinguishes the missing-file case from the
// unset-directory case.
func TestDirLoaderMissingTier(t *testing.T) {
	withHostFs(t, "/geo") // empty fs, no tier files written
	_, err := platformTierLoader(TierWorld110m)
	if err == nil {
		t.Fatal("expected error when tier file is missing")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v, want it to mention a missing bundle", err)
	}
}

// TestDirLoaderUnknownTier rejects tiers outside the known set before
// touching the filesystem.
func TestDirLoaderUnknownTier(t *testing.T) {
	withHostFs(t, "/geo")
	_, err := platformTierLoader(Tier("not-a-tier"))
	if err == nil {
		t.Fatal("expected error for unknown tier")
	}
	if !strings.Contains(err.Error(), "unknown tier") {
		t.Fatalf("error = %v, want it to mention an unknown tier", err)
	}
}
