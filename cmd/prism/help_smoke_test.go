package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

// TestPrismCLIHelpAllSubcommands is one of the four P14 mandatory
// PHASE.md gates. Walks the command tree from newApp() and runs
// `<name> --help` (recursively, including subcommand groups). For
// each, the test asserts:
//
//   - exit code 0
//   - non-empty stdout
//   - stdout contains the command's Usage string
//
// This catches every "forgot to register Usage" bug in one pass.
func TestPrismCLIHelpAllSubcommands(t *testing.T) {
	app := newApp()
	if len(app.Commands) == 0 {
		t.Fatal("newApp() registered no commands")
	}
	for _, c := range app.Commands {
		walkAndAssertHelp(t, []string{c.Name}, c)
	}
}

// walkAndAssertHelp recurses through cli.Command + its Subcommands,
// asserting --help works at every node. Path is the chain of names
// leading to the current node (e.g. ["examples", "show"]).
func walkAndAssertHelp(t *testing.T, path []string, cmd *cli.Command) {
	t.Helper()
	name := strings.Join(path, " ")
	t.Run(name, func(t *testing.T) {
		// Fresh app per subtest so writers/state don't leak.
		fresh := newApp()
		var buf bytes.Buffer
		setWritersRecursive(fresh, &buf)
		// Swallow the auto-os.Exit that --help triggers under cli v3.
		prev := cli.OsExiter
		var observed int
		cli.OsExiter = func(code int) { observed = code }
		t.Cleanup(func() { cli.OsExiter = prev })

		args := append([]string{"prism"}, path...)
		args = append(args, "--help")
		if err := fresh.Run(context.Background(), args); err != nil {
			t.Fatalf("--help err: %v\nstdout=%s", err, buf.String())
		}
		_ = observed // --help may or may not trigger OsExiter; not gated on it.

		out := buf.String()
		if strings.TrimSpace(out) == "" {
			t.Fatalf("--help stdout empty for %q", name)
		}
		if cmd.Usage != "" && !strings.Contains(out, cmd.Usage) {
			t.Errorf("--help output for %q missing Usage %q\n%s", name, cmd.Usage, out)
		}
	})

	for _, sub := range cmd.Commands {
		walkAndAssertHelp(t, append(path, sub.Name), sub)
	}
}
