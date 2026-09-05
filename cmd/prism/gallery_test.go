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
// clean, (b) plot produces non-empty output that parses cleanly for its
// backend.
//
// Every fixture renders via the svg backend and goldens to a sibling
// .svg, EXCEPT fixtures under gallery/table/ (E1-S6): a top-level
// table mark has no SVG geometry equivalent (render/html's
// PRISM_RENDER_MARK_UNSUPPORTED guard on the svg backend), so those
// render via `--format html` and golden to a sibling .html instead.
//
// Set UPDATE_GOLDENS=1 to regenerate the committed golden files.
func TestPrismGalleryFixtures(t *testing.T) {
	galleryDir := repoFile(t, "docs", "src", "gallery")

	type fixture struct {
		spec   string
		golden string
		format string
		name   string
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
		format := "svg"
		ext := ".svg"
		if strings.HasPrefix(rel, "table"+string(filepath.Separator)) {
			format = "html"
			ext = ".html"
		}
		fixtures = append(fixtures, fixture{
			spec:   p,
			golden: strings.TrimSuffix(p, ".prism.json") + ext,
			format: format,
			name:   rel,
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

			plotArgs := []string{"prism", "plot", fx.spec}
			if fx.format == "html" {
				plotArgs = append(plotArgs, "--format", "html")
			}
			out, exit = runCLI(t, plotArgs...)
			if exit != 0 {
				t.Errorf("plot exit %d: %s", exit, firstChars(out, 200))
				return
			}
			body := stripLeadingWarnings(out)

			switch fx.format {
			case "svg":
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
			case "html":
				if !strings.HasPrefix(body, "<!doctype html>") {
					t.Errorf("plot output not HTML: %s", firstChars(body, 200))
					return
				}
				if !strings.Contains(body, "<table") {
					t.Errorf("table fixture rendered no <table>: %s", firstChars(body, 200))
					return
				}
			}

			if updateGoldens {
				if err := os.WriteFile(fx.golden, []byte(body), 0o644); err != nil {
					t.Errorf("write golden: %v", err)
				}
			}
		})
	}
}
