package html_test

import (
	"strings"
	"testing"
)

// tableFormatSpecJSON binds a d3-format specifier to a plain text
// column. Two committed table fixtures asked for one and published
// the raw value until E7-S4.
const tableFormatSpecJSON = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "values": [
      {"name": "Acme", "revenue": 120000, "share": 0.4213},
      {"name": "Globex", "revenue": 80500, "share": 0.1187}
    ]
  },
  "mark": {"type": "table"},
  "encoding": {
    "columns": [
      {"field": "name", "type": "nominal", "title": "Account"},
      {"field": "revenue", "type": "quantitative", "format": ",.0f"},
      {"field": "share", "type": "quantitative", "format": ".1%"}
    ]
  }
}`

// TestPrismHTMLTableColumnFormatApplied asserts encoding.columns[].format
// reaches the rendered cell through the encode/format d3 subset, and
// that the raw scalar still rides in data-prism-sort-value so the
// client-side sort stays numeric rather than lexical.
func TestPrismHTMLTableColumnFormatApplied(t *testing.T) {
	got, err := renderTableSpec(t, tableFormatSpecJSON)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(got)

	for _, want := range []string{">120,000<", ">80,500<", ">42.1%<", ">11.9%<"} {
		if !strings.Contains(s, want) {
			t.Errorf("formatted cell %q missing:\n%s", want, truncate(got, 2000))
		}
	}
	// The unformatted raw values must be gone from the cell text.
	if strings.Contains(s, ">120000<") {
		t.Errorf("raw unformatted revenue still rendered:\n%s", truncate(got, 2000))
	}
	// Sort values stay raw — a formatted string sorts lexically.
	if !strings.Contains(s, `data-prism-sort-value="120000"`) {
		t.Errorf("sort value should stay the raw scalar:\n%s", truncate(got, 2000))
	}
	// A column with no format is untouched.
	if !strings.Contains(s, ">Acme<") {
		t.Errorf("unformatted column lost its value:\n%s", truncate(got, 2000))
	}
}

// TestPrismHTMLTableUnparseableColumnFormatDegrades pins the
// fallback: validate (PRISM_SPEC_011) rejects a bad specifier, but a
// caller that skipped validation must still get the raw value rather
// than broken text — the same degradation formatTick makes on an
// axis.
func TestPrismHTMLTableUnparseableColumnFormatDegrades(t *testing.T) {
	got, err := renderTableSpec(t, strings.Replace(tableFormatSpecJSON, `",.0f"`, `"$,.0f"`, 1))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if s := string(got); !strings.Contains(s, ">120000<") {
		t.Errorf("unparseable format should fall back to the raw value:\n%s", truncate(got, 2000))
	}
}
