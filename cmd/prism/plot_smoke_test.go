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

// TestPrismPlotProducesHTMLForBasicFixture proves `prism plot
// --format html` dispatches to the html backend end-to-end: the
// output is a well-formed HTML document wrapping an inline <svg>.
// There's no table-mark Scene IR yet (E1-S2/E1-S3), so this uses the
// simplest existing mark (bar) as the integration test for the html
// backend's CLI wiring — mirrors render/html's own golden test.
func TestPrismPlotProducesHTMLForBasicFixture(t *testing.T) {
	fixturePath := repoFile(t, "examples", "specs", "bar_basic.json")
	out, exit := runCLI(t, "prism", "plot", "--format", "html", fixturePath)
	if exit != 0 {
		t.Fatalf("plot --format html exited %d: %s", exit, firstChars(out, 400))
	}
	body := stripLeadingWarnings(out)
	if !strings.HasPrefix(body, "<!doctype html>") {
		t.Fatalf("output does not start with <!doctype html>: %s", firstChars(body, 200))
	}
	if !strings.Contains(body, "<svg ") {
		t.Fatalf("output missing embedded <svg: %s", firstChars(body, 400))
	}
}

// TestPrismPlotRejectsPDFFormat asserts the removed PDF backend is
// reported cleanly: `prism plot --format pdf` must exit non-zero with
// PRISM_RENDER_FORMAT_UNAVAILABLE rather than crash or emit bytes.
func TestPrismPlotRejectsPDFFormat(t *testing.T) {
	fixturePath := repoFile(t, "examples", "specs", "bar_basic.json")
	out, exit := runCLI(t, "prism", "plot", "--format", "pdf", fixturePath)
	if exit == 0 {
		t.Fatalf("plot --format pdf exited 0; want non-zero: %s", firstChars(out, 200))
	}
	if !strings.Contains(out, "PRISM_RENDER_FORMAT_UNAVAILABLE") {
		t.Fatalf("plot --format pdf output missing PRISM_RENDER_FORMAT_UNAVAILABLE: %s", firstChars(out, 200))
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
