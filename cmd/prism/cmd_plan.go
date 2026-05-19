package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// planCommand returns the `prism plan` subcommand.
//
// Reads a spec from stdin or a positional file path, builds the DAG,
// and emits it in the requested format. Exit codes mirror the rest of
// the CLI: 0 on success, 1 on plan-level errors (with code printed),
// 2 on usage errors.
func planCommand() *cli.Command {
	return &cli.Command{
		Name:      "plan",
		Usage:     "Compile a Prism spec into a DAG and emit it as dot/text/json",
		ArgsUsage: "[spec-file]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Value: "dot",
				Usage: "Output format: dot | text | json",
			},
			datasetsConfigFlag(),
		},
		Action: runPlan,
	}
}

func runPlan(_ context.Context, cmd *cli.Command) error {
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
	buildOpts := build.Options{
		FS:              afero.NewOsFs(),
		Resolver:        resolve.New(nil),
		DatasetRegistry: registry,
	}

	// Composite specs (P08): dump each child sub-DAG under a
	// "# child N (kind)" header in text/dot formats. JSON wraps the
	// per-child renders in an outer envelope.
	if build.IsComposite(s) {
		composite, err := build.BuildComposite(s, buildOpts)
		if err != nil {
			return reportPlanError(cmd, err, srcName)
		}
		switch format {
		case "json":
			fmt.Fprintf(cmd.Writer, `{"kind":%q,"rows":%d,"cols":%d,"children":[`,
				composite.Kind, composite.Rows, composite.Cols)
			for i, child := range composite.Children {
				if i > 0 {
					fmt.Fprint(cmd.Writer, ",")
				}
				if err := plan.RenderJSON(child.DAG, cmd.Writer); err != nil {
					return err
				}
			}
			fmt.Fprintln(cmd.Writer, "]}")
			return nil
		case "dot", "text":
			for i, child := range composite.Children {
				fmt.Fprintf(cmd.Writer, "# child %d (%s)\n", i, composite.Kind)
				switch format {
				case "dot":
					if err := plan.RenderDOT(child.DAG, cmd.Writer); err != nil {
						return err
					}
				case "text":
					if err := plan.RenderText(child.DAG, cmd.Writer); err != nil {
						return err
					}
				}
				fmt.Fprintln(cmd.Writer)
			}
			return nil
		default:
			return cli.Exit(fmt.Sprintf("unknown format %q (expected dot, text, or json)", format), 2)
		}
	}

	d, _, err := build.Build(s, buildOpts)
	if err != nil {
		return reportPlanError(cmd, err, srcName)
	}

	switch format {
	case "dot":
		return plan.RenderDOT(d, cmd.Writer)
	case "text":
		return plan.RenderText(d, cmd.Writer)
	case "json":
		return plan.RenderJSON(d, cmd.Writer)
	default:
		return cli.Exit(fmt.Sprintf("unknown format %q (expected dot, text, or json)", format), 2)
	}
}

// reportPlanError prints a PRISM_PLAN_* AppError in the same shape as
// `prism validate` does. Returns cli.Exit(1) so the process exits
// non-zero.
func reportPlanError(cmd *cli.Command, err error, srcName string) error {
	var ae *prismerrors.AppError
	if asPlanError(err, &ae) {
		fmt.Fprintf(cmd.Writer, "plan failed: %s\n", srcName)
		fmt.Fprintf(cmd.Writer, "\nERROR %s: %s\n", ae.Code, ae.Message)
		if len(ae.Fixups) > 0 {
			fmt.Fprintln(cmd.Writer, "Fixups:")
			for _, fx := range ae.Fixups {
				fmt.Fprintf(cmd.Writer, "  - %s\n", fx)
			}
		}
		if len(ae.SeeAlso) > 0 {
			fmt.Fprintf(cmd.Writer, "See also: %s\n", strings.Join(ae.SeeAlso, ", "))
		}
		fmt.Fprintf(cmd.Writer, "\nRun `prism errors lookup %s` for more detail.\n", ae.Code)
		return cli.Exit("", 1)
	}
	// Generic fallback for non-AppError errors.
	return cli.Exit(fmt.Sprintf("plan: %v", err), 1)
}

// asPlanError is errors.As specialised for *prismerrors.AppError.
// Inlined so cmd_plan.go does not need to import errors twice
// (prismerrors + the stdlib `errors` package).
func asPlanError(err error, target **prismerrors.AppError) bool {
	if err == nil {
		return false
	}
	if ae, ok := err.(*prismerrors.AppError); ok {
		*target = ae
		return true
	}
	return false
}

// Keep os import live in production by exposing a small wrapper.
// (validate.go uses os.Stdin in its openSpec; we share that helper.)
var _ = os.Stdin
