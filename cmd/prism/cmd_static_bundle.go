package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	staticfs "github.com/frankbardon/prism/static"
)

// staticBundleCommand returns the `prism static-bundle <out-dir>`
// subcommand. Copies every embedded file under static/vendor/prism/
// to the target directory, preserving the directory structure so
// relative imports keep resolving after extraction (D071).
//
// Use case: downstream app wants to serve the JS port without
// vendoring the prism repo. `prism static-bundle ./public/prism`
// drops the full tree at /public/prism/{prism.mjs, d3/*, ...} →
// host page references /public/prism/prism-element.mjs.
func staticBundleCommand() *cli.Command {
	return &cli.Command{
		Name:      "static-bundle",
		Usage:     "Copy vendored browser assets (prism.mjs, d3 modules) to an output directory",
		ArgsUsage: "<out-dir>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Overwrite existing files in the output directory",
				Value: true,
			},
		},
		Action: runStaticBundle,
	}
}

func runStaticBundle(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) != 1 {
		return cli.Exit("static-bundle: exactly one positional argument required (output directory)", 2)
	}
	outDir := args[0]
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return cli.Exit(fmt.Sprintf("mkdir %s: %v", outDir, err), 1)
	}

	stripPrefix := staticfs.BundleRoot
	count := 0
	err := fs.WalkDir(staticfs.Tree, stripPrefix, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Relative path under the bundle root.
		rel := strings.TrimPrefix(path, stripPrefix)
		rel = strings.TrimPrefix(rel, "/")
		dst := filepath.Join(outDir, rel)
		if d.IsDir() {
			if rel == "" {
				return nil // already created
			}
			return os.MkdirAll(dst, 0o755)
		}
		// Skip dotfiles (defensive — embed already ignores most, but
		// .DS_Store or editor backups slipping in shouldn't ship).
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		data, err := staticfs.Tree.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embed %s: %w", path, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		count++
		return nil
	})
	if err != nil {
		return cli.Exit(fmt.Sprintf("walk embed: %v", err), 1)
	}
	fmt.Fprintf(cmd.Writer, "static-bundle: wrote %d files to %s\n", count, outDir)
	return nil
}
