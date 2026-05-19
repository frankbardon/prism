package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"

	prismerrors "github.com/frankbardon/prism/errors"
)

// errorsCommand returns the `prism errors` subcommand with its child
// `lookup` action.
func errorsCommand() *cli.Command {
	return &cli.Command{
		Name:  "errors",
		Usage: "Inspect the Prism error code catalog",
		Commands: []*cli.Command{
			{
				Name:      "lookup",
				Usage:     "Print fixup metadata for one PRISM_* error code",
				ArgsUsage: "<code>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "json",
						Usage: "Emit the catalog entry as JSON",
					},
				},
				Action: runErrorsLookup,
			},
			{
				Name:  "list",
				Usage: "List every registered PRISM_* error code",
				Action: runErrorsList,
			},
		},
	}
}

// runErrorsLookup prints metadata for a single code.
func runErrorsLookup(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) != 1 {
		return cli.Exit("usage: prism errors lookup <PRISM_*_CODE>", 2)
	}
	code := args[0]
	meta, ok := prismerrors.Codes[code]
	if !ok {
		return cli.Exit(fmt.Sprintf("unknown code: %s", code), 1)
	}
	if cmd.Bool("json") {
		enc := json.NewEncoder(cmd.Writer)
		enc.SetIndent("", "  ")
		return enc.Encode(meta)
	}
	fmt.Fprintf(cmd.Writer, "Code:    %s\n", meta.Code)
	fmt.Fprintf(cmd.Writer, "Message: %s\n", meta.Message)
	if meta.FixupNotApplicable {
		fmt.Fprintln(cmd.Writer, "Fixups:  (not applicable)")
	} else if len(meta.Fixups) > 0 {
		fmt.Fprintln(cmd.Writer, "Fixups:")
		for _, fx := range meta.Fixups {
			fmt.Fprintf(cmd.Writer, "  - %s\n", fx)
		}
	}
	if len(meta.SeeAlso) > 0 {
		fmt.Fprintf(cmd.Writer, "See also: %s\n", strings.Join(meta.SeeAlso, ", "))
	}
	return nil
}

// runErrorsList prints every registered code, one per line.
func runErrorsList(_ context.Context, cmd *cli.Command) error {
	for _, code := range prismerrors.CodesSorted() {
		fmt.Fprintln(cmd.Writer, code)
	}
	return nil
}
