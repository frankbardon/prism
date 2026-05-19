package main

import (
	"bytes"
	"context"
	"encoding/json"
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

	// Pulse-backed positive + negative. The negative variant proves the
	// field-existence rule fires against the real cohort schema (P02
	// wires PulseLookup behind the existing validator).
	t.Run("valid-pulse-backed", func(t *testing.T) {
		root := repoFile(t, "")
		originalCwd, _ := os.Getwd()
		if err := os.Chdir(root); err != nil {
			t.Fatalf("chdir(%s): %v", root, err)
		}
		t.Cleanup(func() { _ = os.Chdir(originalCwd) })

		fixture := filepath.Join("testdata", "specs", "bar_pulse_backed.json")
		out, exit := runCLI(t, "prism", "validate", fixture)
		if exit != 0 {
			t.Fatalf("expected exit 0, got %d (stdout=%q)", exit, out)
		}
		if !strings.Contains(out, "valid") {
			t.Errorf("expected stdout to contain \"valid\", got: %q", out)
		}
	})

	t.Run("plan-dot", func(t *testing.T) {
		fixture := repoFile(t, "testdata", "specs", "bar_basic.json")
		out, exit := runCLI(t, "prism", "plan", fixture)
		if exit != 0 {
			t.Fatalf("expected exit 0, got %d (stdout=%q)", exit, out)
		}
		if !strings.HasPrefix(out, "digraph prism_plan") {
			t.Errorf("expected DOT output, got: %q", out)
		}
	})

	t.Run("plan-text", func(t *testing.T) {
		fixture := repoFile(t, "testdata", "specs", "bar_basic.json")
		out, exit := runCLI(t, "prism", "plan", fixture, "--format", "text")
		if exit != 0 {
			t.Fatalf("expected exit 0, got %d (stdout=%q)", exit, out)
		}
		// Tip node is the bar_basic InlineNode itself (no transforms,
		// no aggregate in the spec — so the tip = inline data source).
		if !strings.Contains(out, "InlineNode") {
			t.Errorf("expected text output containing the tip node kind, got: %q", out)
		}
	})

	t.Run("plan-json", func(t *testing.T) {
		fixture := repoFile(t, "testdata", "specs", "bar_basic.json")
		out, exit := runCLI(t, "prism", "plan", fixture, "--format", "json")
		if exit != 0 {
			t.Fatalf("expected exit 0, got %d (stdout=%q)", exit, out)
		}
		if !strings.Contains(out, `"nodes"`) {
			t.Errorf("expected JSON with nodes key, got: %q", out)
		}
	})

	t.Run("plan-missing-dataset", func(t *testing.T) {
		fixture := repoFile(t, "testdata", "specs", "invalid", "dataset_undefined.json")
		out, exit := runCLI(t, "prism", "plan", fixture)
		if exit != 1 {
			t.Fatalf("expected exit 1, got %d (stdout=%q)", exit, out)
		}
		if !strings.Contains(out, "PRISM_PLAN_003") {
			t.Errorf("expected PRISM_PLAN_003 in stdout, got: %q", out)
		}
	})

	t.Run("plan-bad-format", func(t *testing.T) {
		fixture := repoFile(t, "testdata", "specs", "bar_basic.json")
		_, exit := runCLI(t, "prism", "plan", fixture, "--format", "yaml")
		if exit != 2 {
			t.Fatalf("expected exit 2 for bad format, got %d", exit)
		}
	})

	t.Run("execute-json", func(t *testing.T) {
		fixture := repoFile(t, "testdata", "specs", "bar_basic.json")
		out, exit := runCLI(t, "prism", "execute", fixture, "--format", "json")
		if exit != 0 {
			t.Fatalf("expected exit 0, got %d (stdout=%q)", exit, out)
		}
		var rows []map[string]any
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("execute json parse: %v\n%s", err, out)
		}
		if len(rows) != 3 {
			t.Errorf("expected 3 rows, got %d (%v)", len(rows), rows)
		}
		// Sanity-check column presence.
		if _, ok := rows[0]["brand_id"]; !ok {
			t.Errorf("missing brand_id column in %v", rows[0])
		}
	})

	t.Run("execute-table", func(t *testing.T) {
		fixture := repoFile(t, "testdata", "specs", "bar_basic.json")
		out, exit := runCLI(t, "prism", "execute", fixture, "--format", "table")
		if exit != 0 {
			t.Fatalf("expected exit 0, got %d (stdout=%q)", exit, out)
		}
		if !strings.Contains(out, "brand_id") {
			t.Errorf("expected table output to contain header brand_id, got: %q", out)
		}
		if !strings.Contains(out, "alpha") {
			t.Errorf("expected table output to contain alpha row, got: %q", out)
		}
	})

	t.Run("execute-pulse-backed", func(t *testing.T) {
		root := repoFile(t, "")
		originalCwd, _ := os.Getwd()
		if err := os.Chdir(root); err != nil {
			t.Fatalf("chdir(%s): %v", root, err)
		}
		t.Cleanup(func() { _ = os.Chdir(originalCwd) })

		fixture := filepath.Join("testdata", "specs", "bar_pulse_backed.json")
		out, exit := runCLI(t, "prism", "execute", fixture, "--format", "json")
		if exit != 0 {
			t.Fatalf("expected exit 0, got %d (stdout=%q)", exit, out)
		}
		var rows []map[string]any
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("execute json parse: %v\n%s", err, out)
		}
		if len(rows) != 4 {
			t.Errorf("expected 4 brand rows, got %d (%v)", len(rows), rows)
		}
		for _, row := range rows {
			score, ok := row["score"].(float64)
			if !ok {
				t.Errorf("expected float score in row, got %T (%v)", row["score"], row)
				continue
			}
			if score < 0 || score > 1 {
				t.Errorf("score %v out of [0,1] for row %v", score, row)
			}
		}
	})

	t.Run("execute-bad-format", func(t *testing.T) {
		fixture := repoFile(t, "testdata", "specs", "bar_basic.json")
		_, exit := runCLI(t, "prism", "execute", fixture, "--format", "yaml")
		if exit != 2 {
			t.Fatalf("expected exit 2 for bad format, got %d", exit)
		}
	})

	t.Run("invalid-pulse-backed", func(t *testing.T) {
		root := repoFile(t, "")
		originalCwd, _ := os.Getwd()
		if err := os.Chdir(root); err != nil {
			t.Fatalf("chdir(%s): %v", root, err)
		}
		t.Cleanup(func() { _ = os.Chdir(originalCwd) })

		fixture := filepath.Join("testdata", "specs", "invalid", "unknown_field_pulse_backed.json")
		out, exit := runCLI(t, "prism", "validate", fixture)
		if exit != 1 {
			t.Fatalf("expected exit 1, got %d (stdout=%q)", exit, out)
		}
		if !strings.Contains(out, "PRISM_SPEC_001") {
			t.Errorf("expected stdout to mention PRISM_SPEC_001, got: %q", out)
		}
		if !strings.Contains(out, "scor") {
			t.Errorf("expected stdout to identify the typoed field 'scor', got: %q", out)
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
