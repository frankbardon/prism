package geodata

import "testing"

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
