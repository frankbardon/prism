package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Epic E2's CLI reachability gate (E2-S4).
//
// errors/codes.go is only half the contract: the repo's error-handling
// rule is that every PRISM_* code is REACHABLE — `prism errors lookup
// CODE` resolves it and prints a fixup an author can act on. A code
// that exists in the map but cannot be looked up is as useless as no
// code at all.
//
// These run the wired-up CLI end to end, exactly as TestValidateCLISmoke
// does for PRISM_SPEC_001.

// offsetCodes is every code epic E2 registered: the four rejections
// plus the one warning.
var offsetCodes = []string{
	"PRISM_SPEC_063",
	"PRISM_SPEC_064",
	"PRISM_SPEC_065",
	"PRISM_SPEC_066",
	"PRISM_WARN_OFFSET_COLLISION",
}

func TestPrismOffsetErrorsLookupSmoke(t *testing.T) {
	for _, code := range offsetCodes {
		t.Run(code, func(t *testing.T) {
			out, exit := runCLI(t, "prism", "errors", "lookup", code)
			if exit != 0 {
				t.Fatalf("expected exit 0, got %d (stdout=%q)", exit, out)
			}
			if !strings.Contains(out, code) {
				t.Errorf("lookup output does not name %s: %q", code, out)
			}
			if !strings.Contains(out, "Fixups:") {
				t.Errorf("%s resolves but prints no fixup: %q", code, out)
			}
			// A fixup list that renders empty is the same failure as
			// none at all — the header alone tells an author nothing.
			if idx := strings.Index(out, "Fixups:"); idx >= 0 && !strings.Contains(out[idx:], "  - ") {
				t.Errorf("%s prints a Fixups header with no entries: %q", code, out)
			}
		})
	}
}

// TestPrismOffsetValidateCLISmoke drives the four rejections through
// `prism validate` itself: a spec an author could paste exits 1 and
// names its own code, rather than a generic shape error.
func TestPrismOffsetValidateCLISmoke(t *testing.T) {
	cases := []struct {
		name string
		code string
		body string
	}{
		{
			name: "unsupported-mark",
			code: "PRISM_SPEC_063",
			body: `{"$schema":"urn:prism:schema:v1:spec",
			 "data":{"values":[{"q":"Q1","s":"a","v":10}]},
			 "mark":{"type":"tick"},
			 "encoding":{
			   "x":{"field":"q","type":"nominal"},
			   "y":{"field":"v","type":"quantitative"},
			   "x_offset":{"field":"s","type":"nominal"}}}`,
		},
		{
			name: "no-band-on-axis",
			code: "PRISM_SPEC_064",
			body: `{"$schema":"urn:prism:schema:v1:spec",
			 "data":{"values":[{"q":1,"s":"a","v":10}]},
			 "mark":{"type":"bar"},
			 "encoding":{
			   "x":{"field":"q","type":"quantitative"},
			   "y":{"field":"v","type":"quantitative"},
			   "x_offset":{"field":"s","type":"nominal"}}}`,
		},
		{
			name: "both-offsets-bound",
			code: "PRISM_SPEC_064",
			body: `{"$schema":"urn:prism:schema:v1:spec",
			 "data":{"values":[{"q":"Q1","s":"a","v":10}]},
			 "mark":{"type":"bar"},
			 "encoding":{
			   "x":{"field":"q","type":"nominal"},
			   "y":{"field":"v","type":"quantitative"},
			   "x_offset":{"field":"s","type":"nominal"},
			   "y_offset":{"field":"s","type":"nominal"}}}`,
		},
		{
			name: "explicit-stack",
			code: "PRISM_SPEC_065",
			body: `{"$schema":"urn:prism:schema:v1:spec",
			 "data":{"values":[{"q":"Q1","s":"a","v":10}]},
			 "mark":{"type":"bar"},
			 "encoding":{
			   "x":{"field":"q","type":"nominal"},
			   "y":{"field":"v","type":"quantitative","stack":"zero"},
			   "x_offset":{"field":"s","type":"nominal"}}}`,
		},
		{
			name: "same-axis-span",
			code: "PRISM_SPEC_066",
			body: `{"$schema":"urn:prism:schema:v1:spec",
			 "data":{"values":[{"q":"Q1","q2":"Q2","s":"a","v":10}]},
			 "mark":{"type":"bar"},
			 "encoding":{
			   "x":{"field":"q","type":"nominal"},
			   "x2":{"field":"q2","type":"nominal"},
			   "y":{"field":"v","type":"quantitative"},
			   "x_offset":{"field":"s","type":"nominal"}}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeSpecFile(t, tc.name, tc.body)
			out, exit := runCLI(t, "prism", "validate", path)
			if exit != 1 {
				t.Fatalf("expected exit 1, got %d (stdout=%q)", exit, out)
			}
			if !strings.Contains(out, tc.code) {
				t.Errorf("expected %s in output, got: %q", tc.code, out)
			}
			if strings.Contains(out, "PRISM_SPEC_009") {
				t.Errorf("rejected as a generic shape error rather than %s: %q", tc.code, out)
			}
		})
	}
}

// TestPrismOffsetValidateCLIAcceptsGroupedBar is the negative: the
// canonical grouped bar exits 0.
func TestPrismOffsetValidateCLIAcceptsGroupedBar(t *testing.T) {
	path := writeSpecFile(t, "grouped_bar", `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6},
	   {"q":"Q2","s":"a","v":14},{"q":"Q2","s":"b","v":9}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"aggregate":"sum","field":"v","type":"quantitative"},
	   "color":{"field":"s","type":"nominal"},
	   "x_offset":{"field":"s","type":"nominal"}}}`)

	out, exit := runCLI(t, "prism", "validate", path)
	if exit != 0 {
		t.Fatalf("expected exit 0, got %d (stdout=%q)", exit, out)
	}
	if !strings.Contains(out, "valid") {
		t.Errorf("expected stdout to contain \"valid\", got: %q", out)
	}
}

// writeSpecFile drops a spec into the test's temp dir and returns the
// path. The CLI reads a real file, so the smoke test must supply one.
func writeSpecFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
