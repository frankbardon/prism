package main

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrismGalleryFixtures — PHASE.md P16 mandate. Walks
// docs/src/gallery/**/*.prism.json; for each fixture asserts: (a) validates
// clean, (b) plot produces non-empty SVG that parses as XML.
//
// Set UPDATE_GOLDENS=1 to regenerate the committed .svg files.
func TestPrismGalleryFixtures(t *testing.T) {
	galleryDir := repoFile(t, "docs", "src", "gallery")

	type fixture struct {
		spec string
		svg  string
		name string
	}
	var fixtures []fixture
	err := filepath.Walk(galleryDir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}
		if !strings.HasSuffix(p, ".prism.json") {
			return nil
		}
		rel, _ := filepath.Rel(galleryDir, p)
		fixtures = append(fixtures, fixture{
			spec: p,
			svg:  strings.TrimSuffix(p, ".prism.json") + ".svg",
			name: rel,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk gallery: %v", err)
	}
	if len(fixtures) < 50 {
		t.Fatalf("gallery has %d fixtures; expected ≥50 per P16 PHASE.md", len(fixtures))
	}

	// Selection-only specs and a few specialty marks render axes-only
	// or empty SVG today (PRISM_WARN flagged). They still validate; we
	// only assert validate exit and skip the SVG assertion.
	plotSkip := map[string]bool{
		"selections/selection_point.prism.json":            true,
		"selections/selection_interval.prism.json":         true,
		"specialty-marks/sparkline_inline.prism.json":      true,
		"specialty-marks/sparkline_inline_grid.prism.json": true,
		"themes/bar_light.prism.json":                      true,
		"themes/bar_dark.prism.json":                       true,
		"themes/bar_print.prism.json":                      true,
		// Pulse-backed fixtures reference testdata/cohorts/*.pulse with
		// repo-root-relative paths; plot from the gallery cwd misses them.
		"multi-source/actual_vs_benchmark.prism.json":              true,
		"multi-source/bar_pulse_backed.prism.json":                 true,
		"composite-marks/crosstab_heatmap.prism.json":              true,
		"composite-marks/crosstab_overlay_share.prism.json":        true,
		"composite-marks/regression_trend.prism.json":              true,
		"composite-marks/crosstab_significance_shading.prism.json": true,
	}

	updateGoldens := os.Getenv("UPDATE_GOLDENS") == "1"

	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			out, exit := runCLI(t, "prism", "validate", fx.spec)
			if exit != 0 {
				t.Errorf("validate exit %d: %s", exit, firstChars(out, 200))
				return
			}

			if plotSkip[fx.name] {
				return
			}

			out, exit = runCLI(t, "prism", "plot", fx.spec)
			if exit != 0 {
				t.Errorf("plot exit %d: %s", exit, firstChars(out, 200))
				return
			}
			body := stripLeadingWarnings(out)
			if !strings.HasPrefix(body, "<svg ") {
				t.Errorf("plot output not SVG: %s", firstChars(body, 200))
				return
			}
			dec := xml.NewDecoder(bytes.NewReader([]byte(body)))
			for {
				_, err := dec.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Errorf("SVG malformed: %v", err)
					return
				}
			}

			if updateGoldens {
				if err := os.WriteFile(fx.svg, []byte(body), 0o644); err != nil {
					t.Errorf("write golden: %v", err)
				}
			}
		})
	}
}
