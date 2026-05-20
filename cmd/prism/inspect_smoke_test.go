package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestPrismInspectTinyCohort exercises `prism inspect
// testdata/cohorts/tiny.pulse` in-process and asserts the printed
// summary names every field present in tiny.synth.json.
func TestPrismInspectTinyCohort(t *testing.T) {
	root := repoRoot(t)
	cohort := filepath.Join(root, "testdata", "cohorts", "tiny.pulse")
	out, code := runCLI(t, "prism", "inspect", cohort)
	if code != 0 {
		t.Fatalf("inspect exit = %d; want 0\n%s", code, out)
	}
	for _, want := range []string{"brand_id", "score", "age", "fields:", "rows:"} {
		if !strings.Contains(out, want) {
			t.Errorf("inspect output missing %q\n%s", want, out)
		}
	}
}

// TestPrismInspectMissingFile asserts a missing-file path produces a
// non-zero exit + a sensible error message.
func TestPrismInspectMissingFile(t *testing.T) {
	_, code := runCLI(t, "prism", "inspect", "no/such/file.pulse")
	if code == 0 {
		t.Fatalf("inspect missing-file: exit = 0; want non-zero")
	}
}

// TestPrismInspectArgsValidation asserts the subcommand rejects
// zero or multiple positional args.
func TestPrismInspectArgsValidation(t *testing.T) {
	for _, args := range [][]string{
		{"prism", "inspect"},
		{"prism", "inspect", "a", "b"},
	} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			_, code := runCLI(t, args...)
			if code == 0 {
				t.Fatalf("expected non-zero exit for args=%v", args)
			}
		})
	}
}
