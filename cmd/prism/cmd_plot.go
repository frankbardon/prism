package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/render"
	"github.com/frankbardon/prism/render/pdf"
	"github.com/frankbardon/prism/render/svg"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
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
		Usage:     "Compile a Prism spec and render it to SVG (default) or PDF (P15)",
		ArgsUsage: "[spec-file]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Value: "svg",
				Usage: "Output format: svg | pdf | png (V2) | canvas-json (browser-only via prism scene)",
			},
			&cli.FloatFlag{
				Name:  "width",
				Value: 0,
				Usage: "Width in pixels (0 = SVG: viewBox-only / responsive; PDF: use scene's natural frame)",
			},
			&cli.FloatFlag{
				Name:  "height",
				Value: 0,
				Usage: "Height in pixels (0 = SVG: viewBox-only / responsive; PDF: use scene's natural frame)",
			},
			&cli.StringFlag{
				Name:  "out",
				Value: "-",
				Usage: "Output path (default: - for stdout)",
			},
			&cli.StringFlag{
				Name:  "theme",
				Value: "light",
				Usage: "Theme name: light | dark | print",
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
			&cli.BoolFlag{
				Name:  "paginate",
				Usage: "PDF only — emit one page per SceneGrid cell (default: single page sized to the outer grid frame)",
			},
			&cli.StringFlag{
				Name:  "page-size",
				Value: "a4",
				Usage: "PDF only — page size: a4 | letter | legal | <W>x<H>pt (e.g. 612x792pt = US Letter)",
			},
			&cli.StringFlag{
				Name:  "font-dir",
				Value: "",
				Usage: "PDF only — override bundled fonts with .ttf files from this directory (canonical names: prism-sans-regular.ttf, prism-sans-bold.ttf, prism-mono-regular.ttf)",
			},
			datasetsConfigFlag(),
		},
		Action: runPlot,
	}
}

func runPlot(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	format := strings.ToLower(cmd.String("format"))

	// Gate on unsupported formats early. PDF lands in P15; PNG +
	// canvas-json defer per .planning/STATE.md deferred items.
	switch format {
	case "svg", "pdf":
		// supported below
	default:
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

	registry, err := loadDatasetRegistry(cmd)
	if err != nil {
		return cli.Exit(fmt.Sprintf("load --datasets-config: %v", err), 2)
	}
	buildOpts := build.Options{
		FS:              afero.NewOsFs(),
		Resolver:        resolve.New(nil),
		Backend:         inmem.New(),
		DatasetRegistry: registry,
	}
	execOpts := plan.ExecOpts{
		Workers:      cmd.Int("workers"),
		AbortOnError: cmd.Bool("abort-on-error"),
	}
	width := cmd.Float("width")
	height := cmd.Float("height")
	encOpts := encode.EncodeOpts{
		Width:     width,
		Height:    height,
		ThemeName: cmd.String("theme"),
	}

	// Branch by spec shape (flat vs composite per P08 — D049/D050).
	doc, exitErr := plotPipeline(ctx, s, buildOpts, execOpts, encOpts, cmd, srcName)
	if exitErr != nil {
		return exitErr
	}

	var bytes []byte
	switch format {
	case "svg":
		bytes, err = svg.New().Render(doc, render.RenderOpts{
			Format: format,
			Width:  width,
			Height: height,
		})
	case "pdf":
		pageW, pageH, perr := parsePageSize(cmd.String("page-size"))
		if perr != nil {
			return cli.Exit(fmt.Sprintf("--page-size: %v", perr), 2)
		}
		pdfOpts := []pdf.Option{}
		if fd := cmd.String("font-dir"); fd != "" {
			pdfOpts = append(pdfOpts, pdf.WithFontDir(fd))
		}
		bytes, err = pdf.New(pdfOpts...).Render(doc, render.RenderOpts{
			Format:   format,
			Width:    pageW,
			Height:   pageH,
			Paginate: cmd.Bool("paginate"),
		})
	}
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

// plotPipeline drives the build/execute/encode chain for both flat
// and composite specs. Flat specs hit Build+Encode unchanged from
// P05–P07; composite specs route through BuildComposite + per-child
// Execute + EncodeComposite (D049 / D050). The returned cli.ExitError
// is already formatted for the caller.
func plotPipeline(
	ctx context.Context,
	s *spec.Spec,
	buildOpts build.Options,
	execOpts plan.ExecOpts,
	encOpts encode.EncodeOpts,
	cmd *cli.Command,
	srcName string,
) (*scene.SceneDoc, error) {
	if build.IsComposite(s) {
		composite, err := build.BuildComposite(s, buildOpts)
		if err != nil {
			return nil, reportPlanError(cmd, err, srcName)
		}
		per := make([]map[plan.NodeID]*table.Table, len(composite.Children))
		for i, child := range composite.Children {
			res, err := plan.Execute(ctx, child.DAG, execOpts)
			if err != nil {
				return nil, cli.Exit(fmt.Sprintf("execute child %d: %v", i, err), 1)
			}
			if len(res.Errors) > 0 {
				fmt.Fprintf(cmd.Writer, "plot failed: %s (child %d)\n", srcName, i)
				for _, ne := range res.Errors {
					fmt.Fprintf(cmd.Writer, "\nERROR %s (node %s): %v\n", ne.Code, ne.Node, ne.Err)
				}
				return nil, cli.Exit("", 1)
			}
			per[i] = res.Tables
		}
		doc, err := encode.EncodeComposite(s, composite, per, encOpts)
		if err != nil {
			return nil, cli.Exit(fmt.Sprintf("encode: %v", err), 1)
		}
		return doc, nil
	}

	// Flat path — unchanged from P07.
	dag, tipID, err := build.Build(s, buildOpts)
	if err != nil {
		return nil, reportPlanError(cmd, err, srcName)
	}
	res, err := plan.Execute(ctx, dag, execOpts)
	if err != nil {
		return nil, cli.Exit(fmt.Sprintf("execute: %v", err), 1)
	}
	if len(res.Errors) > 0 {
		fmt.Fprintf(cmd.Writer, "plot failed: %s\n", srcName)
		for _, ne := range res.Errors {
			fmt.Fprintf(cmd.Writer, "\nERROR %s (node %s): %v\n", ne.Code, ne.Node, ne.Err)
		}
		return nil, cli.Exit("", 1)
	}
	doc, err := encode.Encode(s, res.Tables, tipID, encOpts)
	if err != nil {
		return nil, cli.Exit(fmt.Sprintf("encode: %v", err), 1)
	}
	return doc, nil
}

// parsePageSize resolves a --page-size flag value into width / height
// in PDF points. Recognised forms:
//
//	a4      -> 595 x 842 (portrait)
//	letter  -> 612 x 792
//	legal   -> 612 x 1008
//	<W>x<H>pt — explicit (e.g. "612x792pt")
//
// Case-insensitive on the named sizes. Returns an error message
// suitable for surfacing to the user when parsing fails.
func parsePageSize(s string) (w, h float64, err error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "a4":
		return 595, 842, nil
	case "letter", "us-letter":
		return 612, 792, nil
	case "legal", "us-legal":
		return 612, 1008, nil
	}
	// Explicit "<W>x<H>pt" form.
	body := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), "pt")
	parts := strings.Split(body, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected a4 | letter | legal | <W>x<H>pt, got %q", s)
	}
	wv, e1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	hv, e2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if e1 != nil || e2 != nil || wv <= 0 || hv <= 0 {
		return 0, 0, fmt.Errorf("expected positive numbers in <W>x<H>pt, got %q", s)
	}
	return wv, hv, nil
}

// reportUnsupportedFormat emits PRISM_RENDER_FORMAT_UNAVAILABLE for
// non-svg/non-pdf formats with the appropriate landing-phase fixup.
func reportUnsupportedFormat(cmd *cli.Command, format string) error {
	phase := "V2"
	switch format {
	case "canvas-json":
		// canvas-json consumes the Scene IR through `prism scene` +
		// prism.mjs in the browser; there's no Go-side renderer.
		phase = "P12"
	case "png":
		phase = "V2"
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
