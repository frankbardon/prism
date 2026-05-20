package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// sceneCommand returns the `prism scene` subcommand. Compiles a
// spec all the way through encode (build → execute → encode) and
// emits the resulting SceneDoc as pretty-printed JSON to stdout
// (or --out path). Consumed by the JS port (prism.mjs in
// static/vendor/prism/) and by the cross-impl parity harness
// (D076 / TestCrossImplSVGParity).
//
// Per D011, SceneDoc JSON is the cross-implementation contract:
// Go produces it, JS consumes it (P12), Canvas + PDF consume it
// (P12/P15). This subcommand is the explicit entrypoint for
// generating the canonical form.
func sceneCommand() *cli.Command {
	return &cli.Command{
		Name:      "scene",
		Usage:     "Compile a Prism spec to Scene IR JSON (for the JS renderer)",
		ArgsUsage: "[spec-file]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "out",
				Value: "-",
				Usage: "Output path (- = stdout)",
			},
			&cli.FloatFlag{
				Name:  "width",
				Value: 0,
				Usage: "Width in pixels (0 = use scene's natural frame; downstream SVG render is viewBox-only)",
			},
			&cli.FloatFlag{
				Name:  "height",
				Value: 0,
				Usage: "Height in pixels (0 = use scene's natural frame; downstream SVG render is viewBox-only)",
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
				Name:  "compact",
				Usage: "Emit minified JSON (default: pretty-printed with 2-space indent)",
			},
			datasetsConfigFlag(),
		},
		Action: runScene,
	}
}

func runScene(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()

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
	encOpts := encode.EncodeOpts{
		Width:     cmd.Float("width"),
		Height:    cmd.Float("height"),
		ThemeName: cmd.String("theme"),
	}

	// Reuse plotPipeline to drive build/execute/encode through both
	// flat and composite spec shapes. We don't call render() —
	// SceneDoc itself is the artifact.
	doc, exitErr := plotPipeline(ctx, s, buildOpts, execOpts, encOpts, cmd, srcName)
	if exitErr != nil {
		return exitErr
	}

	var out []byte
	if cmd.Bool("compact") {
		out, err = json.Marshal(doc)
	} else {
		out, err = json.MarshalIndent(doc, "", "  ")
	}
	if err != nil {
		return cli.Exit(fmt.Sprintf("marshal scene: %v", err), 1)
	}
	// JSON marshallers omit the trailing newline; match other CLI
	// tools by appending one for clean stdout / file writes.
	out = append(out, '\n')

	outPath := cmd.String("out")
	if outPath == "" || outPath == "-" {
		if _, err := cmd.Writer.Write(out); err != nil {
			return cli.Exit(fmt.Sprintf("write stdout: %v", err), 1)
		}
	} else {
		if err := os.WriteFile(outPath, out, 0o644); err != nil {
			return cli.Exit(fmt.Sprintf("write %s: %v", outPath, err), 1)
		}
	}

	for _, warn := range doc.Warnings {
		fmt.Fprintf(cmd.ErrWriter, "WARN %s: %s\n", warn.Code, warn.Message)
	}
	return nil
}
