package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli/v3"
)

// TestPrismCLIPlotPDFEmitsPDFBytes — `prism plot bar_basic --format
// pdf` writes a file starting with %PDF-.
func TestPrismCLIPlotPDFEmitsPDFBytes(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "bar.pdf")
	app := newApp()
	if err := app.Run(context.Background(), []string{
		"prism", "plot",
		"../../examples/specs/bar_basic.json",
		"--format", "pdf",
		"--out", out,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read %s: %v", out, err)
	}
	if len(body) < 1000 {
		t.Fatalf("PDF unexpectedly small: %d bytes", len(body))
	}
	if !bytes.HasPrefix(body, []byte("%PDF-")) {
		t.Fatalf("missing PDF magic; first bytes = %q", string(body[:16]))
	}
}

// TestPrismCLIPlotPDFPaginateMultipage — paginated PDF produces N
// pages from an N-cell SceneGrid. concat_v.json is a 2-cell stack.
func TestPrismCLIPlotPDFPaginateMultipage(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "concat.pdf")
	app := newApp()
	if err := app.Run(context.Background(), []string{
		"prism", "plot",
		"../../examples/specs/concat_v.json",
		"--format", "pdf",
		"--paginate",
		"--out", out,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	pages := countPDFPages(body)
	if pages != 2 {
		t.Fatalf("page count = %d, want 2 (concat_v.json has 2 cells)", pages)
	}
}

// TestPrismCLIPlotPDFPageSizeLetter — --page-size letter produces a
// PDF (size assertion is left to byte-stream sniffing in a fuller
// gate; this test just verifies the flag is accepted and the binary
// is well-formed).
func TestPrismCLIPlotPDFPageSizeLetter(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "letter.pdf")
	app := newApp()
	if err := app.Run(context.Background(), []string{
		"prism", "plot",
		"../../examples/specs/bar_basic.json",
		"--format", "pdf",
		"--page-size", "letter",
		"--out", out,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil || !bytes.HasPrefix(body, []byte("%PDF-")) {
		t.Fatalf("--page-size letter did not produce a valid PDF")
	}
}

// TestPrismCLIPlotPDFRejectsBadPageSize — malformed --page-size
// value exits non-zero with a parse error on stderr. urfave/cli/v3's
// default ExitErrHandler calls os.Exit on cli.ExitError; override
// cli.OsExiter so the test can observe the code without exiting the
// runner.
func TestPrismCLIPlotPDFRejectsBadPageSize(t *testing.T) {
	prev := cli.OsExiter
	observed := -1
	cli.OsExiter = func(code int) { observed = code }
	t.Cleanup(func() { cli.OsExiter = prev })

	app := newApp()
	_ = app.Run(context.Background(), []string{
		"prism", "plot",
		"../../examples/specs/bar_basic.json",
		"--format", "pdf",
		"--page-size", "not-a-size",
	})
	if observed != 2 {
		t.Fatalf("OsExiter observed code = %d, want 2 (cli.Exit with code 2 for parse error)", observed)
	}
}

// TestPrismCLIPlotPDFFontDirOverride — --font-dir pointed at the
// committed bundle still renders successfully. The test directory
// satisfies the canonical-name lookup via the fonts.go scanFontDir
// case-insensitive walk.
func TestPrismCLIPlotPDFFontDirOverride(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "fontdir.pdf")
	app := newApp()
	if err := app.Run(context.Background(), []string{
		"prism", "plot",
		"../../examples/specs/bar_basic.json",
		"--format", "pdf",
		"--font-dir", "../../render/pdf/fonts",
		"--out", out,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil || !bytes.HasPrefix(body, []byte("%PDF-")) {
		t.Fatalf("--font-dir override produced bad PDF")
	}
}

// countPDFPages counts /Type /Page (not /Pages) occurrences in the
// PDF byte stream. Sufficient for gate assertions even when content
// streams are flate-compressed, because the page-object dictionaries
// themselves stay uncompressed in gopdf's output.
func countPDFPages(b []byte) int {
	n := 0
	for i := 0; i+5 < len(b); i++ {
		if !bytes.HasPrefix(b[i:], []byte("/Type")) {
			continue
		}
		j := i + len("/Type")
		for j < len(b) && (b[j] == ' ' || b[j] == '\t' || b[j] == '\n' || b[j] == '\r') {
			j++
		}
		if !bytes.HasPrefix(b[j:], []byte("/Page")) {
			continue
		}
		after := j + len("/Page")
		if after < len(b) && b[after] == 's' {
			continue
		}
		n++
	}
	return n
}
