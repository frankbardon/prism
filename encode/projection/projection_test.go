package projection

import (
	"math"
	"testing"
)

func TestNewKnownProjections(t *testing.T) {
	for _, name := range Available() {
		p, err := New(name, Options{})
		if err != nil {
			t.Fatalf("New(%q): %v", name, err)
		}
		if p.Name() != name && !(name == "albers_usa" && p.Name() == "albers_usa") {
			t.Fatalf("Name = %q, want %q", p.Name(), name)
		}
	}
}

func TestNewUnknown(t *testing.T) {
	if _, err := New("orthographicXYZ", Options{}); err == nil {
		t.Fatal("expected error for unknown projection")
	}
}

func TestMercatorRoundsAtOrigin(t *testing.T) {
	p, _ := New("mercator", Options{Scale: 100, Translate: [2]float64{0, 0}})
	x, y, ok := p.Project(0, 0)
	if !ok {
		t.Fatal("origin should project ok")
	}
	if math.Abs(x) > 1e-9 || math.Abs(y) > 1e-9 {
		t.Fatalf("origin → (%v, %v), want ~0", x, y)
	}
}

func TestMercatorClipsPoles(t *testing.T) {
	p, _ := New("mercator", Options{Scale: 100})
	if _, _, ok := p.Project(0, 90); ok {
		t.Fatal("north pole should clip")
	}
	if _, _, ok := p.Project(0, -90); ok {
		t.Fatal("south pole should clip")
	}
}

func TestEquirectLinear(t *testing.T) {
	p, _ := New("equirectangular", Options{Scale: 100, Translate: [2]float64{0, 0}})
	x1, _, _ := p.Project(10, 0)
	x2, _, _ := p.Project(20, 0)
	if math.Abs((x2-x1)-(x1-0)) > 1e-9 {
		// (x2-x1) and (x1-0) should be equal for a linear projection.
		// x1 - 0 is just x1.
		t.Fatalf("equirect not linear in lon: x1=%v x2=%v", x1, x2)
	}
}

func TestOrthographicClipsBackside(t *testing.T) {
	p, _ := New("orthographic", Options{Scale: 100, Rotate: [3]float64{0, 0, 0}})
	if _, _, ok := p.Project(180, 0); ok {
		t.Fatal("antipode should clip")
	}
	if _, _, ok := p.Project(0, 0); !ok {
		t.Fatal("center should project")
	}
}

func TestAlbersUSAProjectsConus(t *testing.T) {
	p, _ := New("albers_usa", Options{Scale: 1000, Translate: [2]float64{500, 300}})
	if _, _, ok := p.Project(-98, 39); !ok {
		t.Fatal("CONUS centroid should project")
	}
}
