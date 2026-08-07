package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/mcpserve"
)

// mcpOptionsAllowlist names mcpserve.Options fields that legitimately must NOT
// be reachable from a `prism mcp` flag, mapped to the written reason why.
//
// It starts empty and should stay that way. Adding an entry is an intentional,
// review-visible act: it declares that a field of the runner's public options
// is permanently out of the CLI's reach, and the map value must record why.
// Silencing TestMCPOptionsCoversEveryOptionsField by adding a field here
// without a real justification defeats the point of the guard.
var mcpOptionsAllowlist = map[string]string{}

// TestMCPOptionsCoversEveryOptionsField is the enforced form of CLAUDE.md's
// prose warning that a new mcpserve.Options field can silently miss its CLI
// flag.
//
// It drives every `prism mcp` flag at a non-zero test value through the real
// command's flag parsing, calls mcpOptions, and asserts that every exported
// field of the returned struct came back non-zero. The assertion is on the
// produced values rather than on a hardcoded list of field names: a
// name-based check degenerates into "add the field to the list to silence the
// test", which is exactly the failure mode this guards against.
func TestMCPOptionsCoversEveryOptionsField(t *testing.T) {
	got := captureMCPOptions(t)

	v := reflect.ValueOf(got)
	typ := v.Type()
	for i := range typ.NumField() {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if reason, ok := mcpOptionsAllowlist[field.Name]; ok {
			if reason == "" {
				t.Errorf("mcpOptionsAllowlist[%q] has no recorded reason; an allowlist entry must state why the field is not CLI-reachable", field.Name)
			}
			continue
		}
		if v.Field(i).IsZero() {
			t.Errorf(
				"mcpserve.Options.%s came back at its zero value from mcpOptions even though every `prism mcp` flag was set.\n"+
					"That field is not reachable from the CLI, so the behaviour it controls silently does not exist for `prism mcp`.\n"+
					"Resolve it one of two ways:\n"+
					"  1. Wire it in mcpOptions (cmd/prism/cmd_mcp.go), from an existing flag or a new one, and set that flag in captureMCPOptions below; or\n"+
					"  2. Add %q to mcpOptionsAllowlist in this file with a written reason why it must not be CLI-reachable.",
				field.Name, field.Name,
			)
		}
	}
}

// captureMCPOptions runs the real `prism mcp` command with every one of its
// flags set to a non-zero test value and returns the mcpserve.Options that
// mcpOptions builds from it.
//
// The command's Action is swapped for a capturing closure so the options are
// observed after urfave/cli's own flag parsing, without invoking runMCP —
// which would mutate process-global geodata state and then block forever on
// stdio. Flag values only have to be non-empty: mcpOptions is a pure builder
// and never reads the paths.
func captureMCPOptions(t *testing.T) mcpserve.Options {
	t.Helper()

	cmd := mcpCommand()
	var got mcpserve.Options
	var ran bool
	cmd.Action = func(_ context.Context, c *cli.Command) error {
		got = mcpOptions(c)
		ran = true
		return nil
	}

	// Every flag declared by mcpCommand, each at a non-zero value. When a flag
	// is added to `prism mcp`, add it here too so the guard keeps exercising
	// the full surface.
	args := []string{
		"prism", "mcp",
		"--examples-root", t.TempDir(),
		"--datasets-config", t.TempDir() + "/datasets.json",
		"--geodata-dir", t.TempDir(),
	}
	assertEveryMCPFlagCovered(t, cmd, args)

	root := &cli.Command{Name: "prism", Commands: []*cli.Command{cmd}}
	if err := root.Run(context.Background(), args); err != nil {
		t.Fatalf("run prism mcp: %v", err)
	}
	if !ran {
		t.Fatal("prism mcp action did not run; mcpOptions was never called")
	}
	return got
}

// assertEveryMCPFlagCovered fails when a flag declared by mcpCommand is absent
// from the argument list above. Without it, a flag added to `prism mcp` and
// wired into mcpOptions would leave its Options field at zero here and report
// as an unwired field, pointing the next maintainer at the wrong problem.
func assertEveryMCPFlagCovered(t *testing.T, cmd *cli.Command, args []string) {
	t.Helper()

	supplied := make(map[string]bool, len(args))
	for _, a := range args {
		supplied[a] = true
	}
	for _, f := range cmd.Flags {
		names := f.Names()
		if len(names) == 0 {
			continue
		}
		var covered bool
		for _, n := range names {
			if supplied["--"+n] {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("`prism mcp` declares flag --%s but captureMCPOptions does not set it; add it to the args list so the guard exercises the whole flag surface", names[0])
		}
	}
}
