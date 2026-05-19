package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

// TestValidateCLISmoke runs the wired-up CLI app end-to-end against a
// positive fixture (exit 0) and a negative fixture (exit 1 plus
// PRISM_SPEC_001 mentioned in stdout). Uses the real newApp() so any
// future regressions in command wiring surface here.
func TestValidateCLISmoke(t *testing.T) {
	posFixture := repoFile(t, "testdata", "specs", "bar_basic.json")
	negFixture := repoFile(t, "testdata", "specs", "invalid", "unknown_field.json")

	t.Run("valid", func(t *testing.T) {
		out, exit := runCLI(t, "prism", "validate", posFixture)
		if exit != 0 {
			t.Fatalf("expected exit 0, got %d (stdout=%q)", exit, out)
		}
		if !strings.Contains(out, "valid") {
			t.Errorf("expected stdout to contain \"valid\", got: %q", out)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		out, exit := runCLI(t, "prism", "validate", negFixture)
		if exit != 1 {
			t.Fatalf("expected exit 1, got %d (stdout=%q)", exit, out)
		}
		if !strings.Contains(out, "PRISM_SPEC_001") {
			t.Errorf("expected stdout to contain PRISM_SPEC_001, got: %q", out)
		}
	})

	t.Run("errors-lookup", func(t *testing.T) {
		out, exit := runCLI(t, "prism", "errors", "lookup", "PRISM_SPEC_001")
		if exit != 0 {
			t.Fatalf("expected exit 0, got %d (stdout=%q)", exit, out)
		}
		if !strings.Contains(out, "PRISM_SPEC_001") || !strings.Contains(out, "Fixups:") {
			t.Errorf("expected lookup to print code + fixups, got: %q", out)
		}
	})

	t.Run("errors-lookup-unknown", func(t *testing.T) {
		_, exit := runCLI(t, "prism", "errors", "lookup", "TOTALLY_BOGUS")
		if exit != 1 {
			t.Fatalf("expected exit 1 for unknown code, got %d", exit)
		}
	})
}

// runCLI invokes newApp().Run with a captured stdout and returns
// (output, exit-code). cli.Exit errors are translated to their numeric
// code; all other errors map to exit 1. The package-global cli.OsExiter
// is swapped to a no-op so the test process is not killed when a
// subcommand returns a cli.ExitCoder.
func runCLI(t *testing.T, args ...string) (string, int) {
	t.Helper()
	app := newApp()
	var buf bytes.Buffer
	setWritersRecursive(app, &buf)

	// Capture the exit code that cli would have passed to os.Exit.
	var observed int
	prev := cli.OsExiter
	cli.OsExiter = func(code int) { observed = code }
	t.Cleanup(func() { cli.OsExiter = prev })

	err := app.Run(context.Background(), args)
	if err == nil && observed == 0 {
		return buf.String(), 0
	}
	if observed != 0 {
		return buf.String(), observed
	}
	var ce cli.ExitCoder
	if errors.As(err, &ce) {
		return buf.String(), ce.ExitCode()
	}
	return buf.String(), 1
}

// setWritersRecursive walks the command tree and points every command's
// Writer / ErrWriter at the same buffer so subcommand output is captured.
func setWritersRecursive(c *cli.Command, w io.Writer) {
	c.Writer = w
	c.ErrWriter = w
	for _, sub := range c.Commands {
		setWritersRecursive(sub, w)
	}
}

func repoFile(t *testing.T, parts ...string) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return filepath.Join(append([]string{cwd}, parts...)...)
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			t.Fatalf("go.mod not found in any parent of %s", cwd)
		}
		cwd = parent
	}
}

