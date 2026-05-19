package errors

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPrismCodesHaveFixups enforces that every catalog entry either has
// at least one fixup template or sets FixupNotApplicable: true.
func TestPrismCodesHaveFixups(t *testing.T) {
	for code, meta := range Codes {
		if meta.FixupNotApplicable {
			if len(meta.Fixups) != 0 {
				t.Errorf("%s: FixupNotApplicable=true but Fixups is non-empty", code)
			}
			continue
		}
		if len(meta.Fixups) == 0 {
			t.Errorf("%s: no fixups and not marked FixupNotApplicable", code)
		}
		if meta.Code != code {
			t.Errorf("%s: meta.Code=%q does not match map key", code, meta.Code)
		}
		if meta.Message == "" {
			t.Errorf("%s: empty Message", code)
		}
	}
}

func TestAppErrorJSONEnvelopeShape(t *testing.T) {
	e := New("PRISM_SPEC_001", "Field xfield not in source schema for dataset cohort.", map[string]any{
		"Field":     "xfield",
		"Dataset":   "cohort",
		"Available": "x, y, brand_id",
	})
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, want := range []string{"code", "message", "context", "fixups"} {
		if _, ok := probe[want]; !ok {
			t.Errorf("envelope missing key %q (have %v)", want, keys(probe))
		}
	}
	if !strings.Contains(string(raw), "PRISM_SPEC_001") {
		t.Errorf("envelope missing code value: %s", raw)
	}
	if !strings.Contains(string(raw), "xfield") {
		t.Errorf("envelope missing fixup expansion of {{.Field}}: %s", raw)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
