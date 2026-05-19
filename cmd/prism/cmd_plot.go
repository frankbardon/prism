package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/render/svg"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// plotCommand returns the `prism plot` subcommand. Reads a spec from
// stdin or a positional file path, builds the DAG, executes via the
// in-memory backend, encodes to a SceneDoc, renders to bytes via the
// selected format's Renderer (SVG only in P05), and writes to stdout
// or --out. Render warnings stream to stderr as `WARN PRISM_WARN_*`
// lines so the user sees deferred-feature notices without blocking
// the byte stream.
func plotCommand() *cli.Command {
	return &cli.Command{
		Name:      "plot",
		Usage:     "Compile a Prism spec and render it to SVG (default), PNG (P12), or PDF (P15)",
		ArgsUsage: "[spec-file]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Value: "svg",
				Usage: "Output format: svg | png (P12) | pdf (P15) | canvas-json (P12)",
			},
			&cli.FloatFlag{
				Name:  "width",
				Value: 800,
				Usage: "Width in pixels",
			},
			&cli.FloatFlag{
				Name:  "height",
				Value: 600,
				Usage: "Height in pixels",
			},
			&cli.StringFlag{
				Name:  "out",
				Value: "-",
				Usage: "Output path (default: - for stdout)",
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
		},
		Action: runPlot,
	}
}

func runPlot(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	format := strings.ToLower(cmd.String("format"))

	// Gate on unsupported formats early.
	if format != "svg" {
		return reportUnsupportedFormat(cmd, format)
	}

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

	dag, tipID, err := build.Build(s, build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
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
		fmt.Fprintf(cmd.Writer, "plot failed: %s\n", srcName)
		for _, ne := range res.Errors {
			fmt.Fprintf(cmd.Writer, "\nERROR %s (node %s): %v\n", ne.Code, ne.Node, ne.Err)
		}
		return cli.Exit("", 1)
	}

	width := cmd.Float("width")
	height := cmd.Float("height")
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{
		Width:  width,
		Height: height,
	})
	if err != nil {
		return cli.Exit(fmt.Sprintf("encode: %v", err), 1)
	}

	bytes, err := svg.New().Render(doc, render.RenderOpts{
		Format: format,
		Width:  width,
		Height: height,
	})
	if err != nil {
		return cli.Exit(fmt.Sprintf("render: %v", err), 1)
	}

	// Write output.
	outPath := cmd.String("out")
	if outPath == "" || outPath == "-" {
		if _, err := cmd.Writer.Write(bytes); err != nil {
			return cli.Exit(fmt.Sprintf("write stdout: %v", err), 1)
		}
	} else {
		if err := os.WriteFile(outPath, bytes, 0o644); err != nil {
			return cli.Exit(fmt.Sprintf("write %s: %v", outPath, err), 1)
		}
	}

	// Surface warnings to stderr so the byte stream stays clean.
	for _, warn := range doc.Warnings {
		fmt.Fprintf(cmd.ErrWriter, "WARN %s: %s\n", warn.Code, warn.Message)
	}
	return nil
}

// reportUnsupportedFormat emits PRISM_RENDER_FORMAT_UNAVAILABLE for
// non-svg formats with the appropriate landing-phase fixup.
func reportUnsupportedFormat(cmd *cli.Command, format string) error {
	phase := "P12"
	switch format {
	case "pdf":
		phase = "P15"
	case "canvas-json":
		phase = "P12"
	case "png":
		phase = "P12"
	}
	err := prismerrors.New(
		"PRISM_RENDER_FORMAT_UNAVAILABLE",
		fmt.Sprintf("Render format %s is not available in the current Prism build (lands in %s).", format, phase),
		map[string]any{"Format": format, "Phase": phase},
	)
	var ae *prismerrors.AppError
	if asPlotError(err, &ae) {
		fmt.Fprintf(cmd.Writer, "plot failed: unsupported format %s\n", format)
		fmt.Fprintf(cmd.Writer, "\nERROR %s: %s\n", ae.Code, ae.Message)
		if len(ae.Fixups) > 0 {
			fmt.Fprintln(cmd.Writer, "Fixups:")
			for _, fx := range ae.Fixups {
				fmt.Fprintf(cmd.Writer, "  - %s\n", fx)
			}
		}
		return cli.Exit("", 1)
	}
	return cli.Exit(err.Error(), 1)
}

// asPlotError mirrors asPlanError; inlined to keep import minimal.
func asPlotError(err error, target **prismerrors.AppError) bool {
	if err == nil {
		return false
	}
	if ae, ok := err.(*prismerrors.AppError); ok {
		*target = ae
		return true
	}
	return false
}
