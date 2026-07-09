package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDataSourceVariantRejected pins the E4-S3 removal of the wire-level
// `data.source` variant: a spec `data` block that carries a `source`
// key must fail to decode (Prism no longer opens .pulse files), with a
// message pointing at the surviving inline `values` / runtime `ref`
// paths and the PRISM_SPEC_039 code.
func TestDataSourceVariantRejected(t *testing.T) {
	cases := []string{
		`{"source": "cohort.pulse"}`,
		`{"name": "tiny", "source": "testdata/cohorts/tiny.pulse"}`,
	}
	for _, in := range cases {
		var d Data
		err := json.Unmarshal([]byte(in), &d)
		if err == nil {
			t.Fatalf("decode %s: expected error, got nil (Source=%q)", in, d.Source)
		}
		if !strings.Contains(err.Error(), "PRISM_SPEC_039") {
			t.Errorf("decode %s: error %q does not reference PRISM_SPEC_039", in, err)
		}
		if !strings.Contains(err.Error(), "source") {
			t.Errorf("decode %s: error %q should mention the removed source variant", in, err)
		}
	}
}

// TestDataSurvivingVariantsDecode confirms the four surviving data
// variants still decode after the source variant was removed.
func TestDataSurvivingVariantsDecode(t *testing.T) {
	cases := map[string]string{
		"values":             `{"values": [{"a": 1}]}`,
		"ref":                `{"ref": "cohort"}`,
		"name":               `{"name": "current"}`,
		"feature_collection": `{"feature_collection": {"tier": "world-110m"}}`,
	}
	for variant, in := range cases {
		var d Data
		if err := json.Unmarshal([]byte(in), &d); err != nil {
			t.Errorf("%s: decode %s failed: %v", variant, in, err)
		}
	}
}
