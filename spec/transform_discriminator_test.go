package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPrismTransformWindowCarriesSort pins the E5-S4 decoder fix: a
// window transform may carry a `sort` array, which is itself a
// transform discriminator. schema/v1/transform.schema.json has always
// advertised the shape; before the fix the decoder refused it.
func TestPrismTransformWindowCarriesSort(t *testing.T) {
	raw := `{"window": [{"op": "row_number", "as": "rn"}], "sort": [{"field": "v", "order": "descending"}]}`
	var tr Transform
	if err := json.Unmarshal([]byte(raw), &tr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tr.Window == nil {
		t.Fatal("window variant not selected")
	}
	if tr.Sort != nil {
		t.Fatal("sort variant must not also be selected")
	}
	if len(tr.Window.Sort) != 1 || tr.Window.Sort[0].Order != "descending" {
		t.Fatalf("window sort not carried: %+v", tr.Window.Sort)
	}
}

// TestPrismTransformStandaloneSort pins that the subordinate-key rule
// did not steal the plain sort transform.
func TestPrismTransformStandaloneSort(t *testing.T) {
	var tr Transform
	if err := json.Unmarshal([]byte(`{"sort": [{"field": "v"}]}`), &tr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tr.Sort == nil || tr.Window != nil {
		t.Fatalf("wrong variant: sort=%v window=%v", tr.Sort, tr.Window)
	}
}

// TestPrismTransformAmbiguousDiscriminator pins that a genuinely
// undecidable key pair still errors rather than silently picking one.
func TestPrismTransformAmbiguousDiscriminator(t *testing.T) {
	var tr Transform
	err := json.Unmarshal([]byte(`{"filter": {"field": "v", "gt": 1}, "limit": 5}`), &tr)
	if err == nil {
		t.Fatal("want an error for two unrelated discriminators")
	}
	if !strings.Contains(err.Error(), "multiple discriminator keys") {
		t.Fatalf("unexpected error: %v", err)
	}
}
