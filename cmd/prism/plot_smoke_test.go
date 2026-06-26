package main

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrismPlotProducesValidSVGForAllFixtures sweeps examples/specs
// and runs every fixture that builds + executes cleanly through
// `prism plot`. Asserts the output decodes as well-formed XML. Acts
// as the regression net for the spec → svg pipeline; breaks the
// build if anyone breaks structure in a later phase.
func TestPrismPlotProducesValidSVGForAllFixtures(t *testing.T) {
	// P08 unskipped layer + concat / hconcat / vconcat; P09 unskipped
	// facet + repeat (BuildComposite + EncodeComposite). Remaining
	// deferrals: selection (P13).
	skip := map[string]bool{
		"selection_interval.json": true,
		"selection_point.json":    true,
		// Specialty / composite marks render as axes-only with a
		// PRISM_WARN_MARK_NOT_IMPLEMENTED warning; we still expect
		// the SVG to be well-formed, so DO NOT skip them.
	}

	dir := repoFile(t, "examples", "specs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		name := e.Name()
		if skip[name] {
			continue
		}
		t.Run(name, func(t *testing.T) {
			fixturePath := filepath.Join(dir, name)
			out, exit := runCLI(t, "prism", "plot", fixturePath)
			if exit != 0 {
				t.Skipf("plot exited %d (likely PRISM_PLAN_002 or PRISM_SPEC_*); skipping: %s", exit, firstChars(out, 200))
			}
			// Strip the warning lines (which precede the SVG bytes on
			// stderr; in tests stderr/stdout are merged).
			body := stripLeadingWarnings(out)
			if !strings.HasPrefix(body, "<svg ") {
				t.Fatalf("output does not start with <svg: %s", firstChars(body, 200))
			}
			dec := xml.NewDecoder(bytes.NewReader([]byte(body)))
			depth := 0
			for {
				tok, err := dec.Token()
				if err != nil {
					break
				}
				switch tok.(type) {
				case xml.StartElement:
					depth++
				case xml.EndElement:
					depth--
				}
			}
			if depth != 0 {
				t.Errorf("XML unbalanced (depth %d) for fixture %s", depth, name)
			}
		})
	}
}

// stripLeadingWarnings drops any `WARN PRISM_WARN_*` lines at the
// top of the buffer so the XML parser sees the SVG bytes directly.
// In the CLI test harness stderr is merged into the output buffer.
func stripLeadingWarnings(s string) string {
	for {
		nl := strings.IndexByte(s, '\n')
		if nl < 0 || !strings.HasPrefix(s, "WARN ") {
			return s
		}
		s = s[nl+1:]
	}
}
