//go:build !js

package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	prismschema "github.com/frankbardon/prism/schema"
)

//go:embed templates/editor/*
var editorTemplatesFS embed.FS

// initExampleFixtures is the curated set of fixtures copied into
// `.prism/examples/` by `prism init` (default mode). Eight chosen for
// breadth: one per major mark family plus one composition example.
var initExampleFixtures = []string{
	"bar_basic.json",
	"line_basic.json",
	"area_basic.json",
	"point_scatter.json",
	"layer_actual_vs_benchmark.json",
	"facet_by_region.json",
	"pie.json",
	"histogram.json",
}

const initReadme = `# .prism/

Project-local Prism configuration written by ` + "`prism init`" + `.

- ` + "`schemas/`" + ` — JSON Schema files for spec validation + editor autocomplete.
- ` + "`examples/`" + ` — curated starter specs. Copy one and edit.
- ` + "`editor/`" + ` — editor config templates (VSCode, JetBrains, Neovim, Vim).

## Quickstart

` + "```" + `
cp .prism/examples/bar_basic.json my-chart.prism.json
prism plot my-chart.prism.json > my-chart.svg
` + "```" + `

## Editor setup

Open ` + "`.prism/editor/`" + ` for one-line setup snippets per editor.
`

func initCommand() *cli.Command {
	return &cli.Command{
		Name:      "init",
		Usage:     "Bootstrap a .prism/ directory with schemas, examples, and editor configs",
		ArgsUsage: "[dir]",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "force", Usage: "Overwrite existing .prism/"},
			&cli.BoolFlag{Name: "bare", Usage: "Skip examples + editor configs (schemas only)"},
		},
		Action: runInit,
	}
}

func runInit(_ context.Context, cmd *cli.Command) error {
	target := "."
	if cmd.NArg() > 0 {
		target = cmd.Args().First()
	}
	prismDir := filepath.Join(target, ".prism")
	if _, err := os.Stat(prismDir); err == nil {
		if !cmd.Bool("force") {
			return cli.Exit(fmt.Sprintf("prism init: %s already exists; pass --force to overwrite", prismDir), 1)
		}
		if err := os.RemoveAll(prismDir); err != nil {
			return cli.Exit(fmt.Sprintf("prism init: clear existing dir: %v", err), 1)
		}
	}

	if err := writeSchemas(filepath.Join(prismDir, "schemas")); err != nil {
		return cli.Exit(fmt.Sprintf("prism init: write schemas: %v", err), 1)
	}

	if !cmd.Bool("bare") {
		if err := writeExamples(filepath.Join(prismDir, "examples")); err != nil {
			return cli.Exit(fmt.Sprintf("prism init: write examples: %v", err), 1)
		}
		if err := writeEditorConfigs(filepath.Join(prismDir, "editor")); err != nil {
			return cli.Exit(fmt.Sprintf("prism init: write editor configs: %v", err), 1)
		}
	}

	if err := os.WriteFile(filepath.Join(prismDir, "README.md"), []byte(initReadme), 0o644); err != nil {
		return cli.Exit(fmt.Sprintf("prism init: write README: %v", err), 1)
	}

	fmt.Fprintf(cmd.Writer, "wrote %s\n", prismDir)
	return nil
}

func writeSchemas(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	all, err := prismschema.V1Schemas()
	if err != nil {
		return err
	}
	for name, body := range all {
		dst := filepath.Join(dir, name+".schema.json")
		if err := os.WriteFile(dst, body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeExamples(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range initExampleFixtures {
		src := filepath.Join("testdata", "specs", name)
		body, err := os.ReadFile(src)
		if err != nil {
			// Fixture missing in the install environment (e.g. user ran
			// the binary outside the repo). Skip silently; init still
			// produces a usable schemas dir.
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeEditorConfigs(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(editorTemplatesFS, "templates/editor", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		body, err := editorTemplatesFS.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.Base(p))
		return os.WriteFile(dst, body, 0o644)
	})
}
