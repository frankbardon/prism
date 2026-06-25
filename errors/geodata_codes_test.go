package errors

import (
	"strings"
	"testing"
)

// TestGeodataCodesRegistered asserts the two descriptor-style geodata
// codes exist in the catalog with a usable fixup, and that fixup template
// expansion threads the tier/path context (reachable via
// `prism errors lookup`).
func TestGeodataCodesRegistered(t *testing.T) {
	for _, code := range []string{"PRISM_GEODATA_DIR_UNSET", "PRISM_GEODATA_TIER_MISSING"} {
		meta, ok := Codes[code]
		if !ok {
			t.Fatalf("%s not registered in Codes", code)
		}
		if meta.Code != code {
			t.Errorf("%s: meta.Code = %q", code, meta.Code)
		}
		if meta.Message == "" {
			t.Errorf("%s: empty Message", code)
		}
		if len(meta.Fixups) == 0 {
			t.Errorf("%s: no fixups", code)
		}
	}

	e := New("PRISM_GEODATA_TIER_MISSING", "Geodata tier file for world-110m not found at /geo/world-110m.geo.json.", map[string]any{
		"Tier": "world-110m",
		"Path": "/geo/world-110m.geo.json",
	})
	joined := strings.Join(e.Fixups, "\n")
	if !strings.Contains(joined, "world-110m.geo.json") {
		t.Errorf("fixups did not expand {{.Tier}}: %q", joined)
	}
}
