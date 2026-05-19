package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/compile/inmem"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// executeCommand returns the `prism execute` subcommand. Reads a spec
// from stdin or a positional file path, builds the DAG, executes it
// through the in-memory backend, and prints the terminal Sink's
// table in the requested format (json default; table for a
// human-readable text grid). Exit codes mirror the rest of the CLI:
// 0 on success, 1 on plan/compile errors, 2 on usage errors.
func executeCommand() *cli.Command {
	return &cli.Command{
		Name:      "execute",
		Usage:     "Compile a Prism spec and execute it; print the final table as JSON rows or a text grid",
		ArgsUsage: "[spec-file]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Value: "json",
				Usage: "Output format: json | table",
			},
			&cli.IntFlag{
				Name:  "workers",
				Value: 0,
				Usage: "Worker count for the executor (0 = NumCPU; 1 = serial)",
			},
			&cli.BoolFlag{
				Name:  "abort-on-error",
				Usage: "Stop on the first node error instead of skipping dependents",
			},
			datasetsConfigFlag(),
		},
		Action: runExecute,
	}
}

func runExecute(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	format := strings.ToLower(cmd.String("format"))

	src, srcName, err := openSpec(args)
	if err != nil {
		return cli.Exit(err.Error(), 2)
	}
	defer src.Close()
	body, err := io.ReadAll(src)
	if err != nil {
		return cli.Exit(fmt.Sprintf("read %s: %v", srcName, err), 2)
	}

	s, err := spec.DecodeBytes(body)
	if err != nil {
		return cli.Exit(fmt.Sprintf("decode %s: %v", srcName, err), 2)
	}

	registry, err := loadDatasetRegistry(cmd)
	if err != nil {
		return cli.Exit(fmt.Sprintf("load --datasets-config: %v", err), 2)
	}
	dag, tipID, err := build.Build(s, build.Options{
		FS:              afero.NewOsFs(),
		Resolver:        resolve.New(nil),
		Backend:         inmem.New(),
		DatasetRegistry: registry,
	})
	if err != nil {
		return reportPlanError(cmd, err, srcName)
	}

	res, err := plan.Execute(ctx, dag, plan.ExecOpts{
		Workers:      cmd.Int("workers"),
		AbortOnError: cmd.Bool("abort-on-error"),
	})
	if err != nil {
		return cli.Exit(fmt.Sprintf("execute: %v", err), 1)
	}
	if len(res.Errors) > 0 {
		// Surface each node error in the same shape as `prism validate`.
		fmt.Fprintf(cmd.Writer, "execute failed: %s\n", srcName)
		for _, ne := range res.Errors {
			fmt.Fprintf(cmd.Writer, "\nERROR %s (node %s): %v\n", ne.Code, ne.Node, ne.Err)
		}
		return cli.Exit("", 1)
	}

	final, ok := res.Tables[tipID]
	if !ok || final == nil {
		return cli.Exit(fmt.Sprintf("execute: tip node %q produced no table", tipID), 1)
	}

	switch format {
	case "json":
		return renderTableJSON(cmd.Writer, final)
	case "table":
		return renderTableText(cmd.Writer, final)
	default:
		return cli.Exit(fmt.Sprintf("unknown format %q (expected json or table)", format), 2)
	}
}

// renderTableJSON prints the table as a pretty-printed JSON array of
// row objects. Column order follows the table's schema declaration.
func renderTableJSON(w io.Writer, tbl *table.Table) error {
	rows := make([]map[string]any, 0, tbl.NumRows())
	for i := 0; i < tbl.NumRows(); i++ {
		row := make(map[string]any, len(tbl.FieldNames()))
		for _, name := range tbl.FieldNames() {
			col, _ := tbl.Column(name)
			row[name] = col.ValueAt(i)
		}
		rows = append(rows, row)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

// renderTableText prints the table as a simple two-space-separated
// text grid. Header row, then one line per data row. Used in smoke
// tests because the output is trivial to parse back.
func renderTableText(w io.Writer, tbl *table.Table) error {
	cols := tbl.FieldNames()
	if _, err := fmt.Fprintln(w, strings.Join(cols, "  ")); err != nil {
		return err
	}
	for i := 0; i < tbl.NumRows(); i++ {
		parts := make([]string, len(cols))
		for j, name := range cols {
			c, _ := tbl.Column(name)
			parts[j] = fmt.Sprintf("%v", c.ValueAt(i))
		}
		if _, err := fmt.Fprintln(w, strings.Join(parts, "  ")); err != nil {
			return err
		}
	}
	return nil
}

// prismErrAs is errors.As for *prismerrors.AppError without pulling
// the stdlib errors package twice into this file.
func prismErrAs(err error) (*prismerrors.AppError, bool) {
	if err == nil {
		return nil, false
	}
	if ae, ok := err.(*prismerrors.AppError); ok {
		return ae, true
	}
	return nil, false
}

// guard against unused imports when refactoring.
var _ = prismErrAs
