package spec

import (
	"encoding/json"
	"testing"
)

// TestBulletMarkDefRoundTrip asserts a bullet mark def carrying all of its
// E3 inputs (target, bands, comparative, orientation) decodes into MarkDef
// without loss and re-encodes to an equivalent shape. target / comparative
// support both literal numbers and data-field names.
func TestBulletMarkDefRoundTrip(t *testing.T) {
	const in = `{
		"type": "bullet",
		"target": 80,
		"bands": [40, 60, 90],
		"comparative": "prior_period",
		"orientation": "vertical"
	}`

	var m Mark
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Def == nil {
		t.Fatalf("expected mark def, got shorthand %q", m.Shorthand)
	}
	def := m.Def

	if def.Type != "bullet" {
		t.Errorf("type = %q, want bullet", def.Type)
	}
	if got, ok := def.Target.(float64); !ok || got != 80 {
		t.Errorf("target = %#v, want literal 80", def.Target)
	}
	if got, ok := def.Comparative.(string); !ok || got != "prior_period" {
		t.Errorf("comparative = %#v, want field-ref \"prior_period\"", def.Comparative)
	}
	wantBands := []float64{40, 60, 90}
	if len(def.Bands) != len(wantBands) {
		t.Fatalf("bands len = %d, want %d", len(def.Bands), len(wantBands))
	}
	for i, b := range wantBands {
		if def.Bands[i] != b {
			t.Errorf("bands[%d] = %v, want %v", i, def.Bands[i], b)
		}
	}
	if def.Orientation != "vertical" {
		t.Errorf("orientation = %q, want vertical", def.Orientation)
	}

	// Re-encode and re-decode to confirm no field is dropped on the wire.
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var rt Mark
	if err := json.Unmarshal(out, &rt); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if rt.Def == nil {
		t.Fatalf("round-trip lost mark def")
	}
	if got, ok := rt.Def.Target.(float64); !ok || got != 80 {
		t.Errorf("round-trip target = %#v, want 80", rt.Def.Target)
	}
	if rt.Def.Comparative != "prior_period" {
		t.Errorf("round-trip comparative = %#v, want prior_period", rt.Def.Comparative)
	}
	if rt.Def.Orientation != "vertical" {
		t.Errorf("round-trip orientation = %q, want vertical", rt.Def.Orientation)
	}
}
