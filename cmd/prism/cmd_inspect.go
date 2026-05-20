package main

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/resolve"
)

// inspectCommand returns the `prism inspect <pulse-file>` subcommand.
// Resolves the file via resolve.DefaultResolver, reads the schema
// + materialises the Table, prints a column-aligned summary of
// fields + row count. Useful when authoring a spec against a new
// .pulse cohort and needing to know which fields exist + their types.
func inspectCommand() *cli.Command {
	return &cli.Command{
		Name:      "inspect",
		Usage:     "List fields, types, and row count for a .pulse cohort",
		ArgsUsage: "<pulse-file>",
		Action:    runInspect,
	}
}

func runInspect(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) != 1 {
		return cli.Exit("inspect: expected exactly one positional argument: <pulse-file>", 2)
	}
	ref := args[0]
	fs := afero.NewOsFs()

	// Materialise the Table so we can print both the schema and the
	// authoritative row count (the executor reports rowCount as
	// part of the *table.Table).
	src := nodes.New(ref, fs, resolve.New(nil))
	t, err := src.Execute(ctx, nil)
	if err != nil {
		return cli.Exit(fmt.Sprintf("inspect %s: %v", ref, err), 1)
	}
	schema := t.Schema()
	if schema == nil {
		return cli.Exit(fmt.Sprintf("inspect %s: nil schema after execute", ref), 1)
	}

	fmt.Fprintf(cmd.Writer, "%s\n", ref)
	fmt.Fprintf(cmd.Writer, "  fields: %d\n", len(schema.Fields))
	fmt.Fprintf(cmd.Writer, "  rows:   %d\n", t.NumRows())
	fmt.Fprintln(cmd.Writer)

	tw := tabwriter.NewWriter(cmd.Writer, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tDESC")
	for _, f := range schema.Fields {
		desc := strings.TrimSpace(f.Description)
		if desc == "" {
			desc = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", f.Name, f.Type.String(), desc)
	}
	_ = tw.Flush()
	return nil
}
