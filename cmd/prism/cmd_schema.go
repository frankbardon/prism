package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/schema"
)

// schemaCommand returns the `prism schema` subcommand group:
// list, show, export, bundle. All four read from the embedded
// JSON Schema bundle baked into schema/v1/ at build time.
func schemaCommand() *cli.Command {
	return &cli.Command{
		Name:  "schema",
		Usage: "List, inspect, and export the embedded Prism JSON Schema bundle",
		Commands: []*cli.Command{
			{Name: "list", Usage: "List every embedded schema name (sorted)", Action: runSchemaList},
			{Name: "show", Usage: "Print the raw JSON for one schema by name",
				ArgsUsage: "<name>", Action: runSchemaShow},
			{Name: "export", Usage: "Write every schema file (+ _meta/*.md) to <out-dir>",
				ArgsUsage: "<out-dir>", Action: runSchemaExport},
			{Name: "bundle", Usage: "Emit a single-file bundle JSON (D087) to <out-file>",
				ArgsUsage: "<out-file>", Action: runSchemaBundle},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			_ = cli.ShowSubcommandHelp(c)
			return cli.Exit("", 2)
		},
	}
}

func runSchemaList(_ context.Context, cmd *cli.Command) error {
	all, err := schema.V1Schemas()
	if err != nil {
		return cli.Exit(fmt.Sprintf("schema list: %v", err), 1)
	}
	names := sortedKeys(all)
	for _, n := range names {
		fmt.Fprintln(cmd.Writer, n)
	}
	return nil
}

func runSchemaShow(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) != 1 {
		return cli.Exit("schema show: expected exactly one positional argument: <name>", 2)
	}
	all, err := schema.V1Schemas()
	if err != nil {
		return cli.Exit(fmt.Sprintf("schema show: %v", err), 1)
	}
	body, ok := all[args[0]]
	if !ok {
		return cli.Exit(fmt.Sprintf("schema show: no schema named %q (run `prism schema list`)", args[0]), 1)
	}
	_, _ = cmd.Writer.Write(body)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		fmt.Fprintln(cmd.Writer)
	}
	return nil
}

func runSchemaExport(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) != 1 {
		return cli.Exit("schema export: expected exactly one positional argument: <out-dir>", 2)
	}
	outDir := args[0]
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return cli.Exit(fmt.Sprintf("schema export: mkdir %s: %v", outDir, err), 1)
	}

	// Write the schema files.
	all, err := schema.V1Schemas()
	if err != nil {
		return cli.Exit(fmt.Sprintf("schema export: %v", err), 1)
	}
	for _, name := range sortedKeys(all) {
		dst := filepath.Join(outDir, name+".schema.json")
		if err := os.WriteFile(dst, all[name], 0o644); err != nil {
			return cli.Exit(fmt.Sprintf("schema export: write %s: %v", dst, err), 1)
		}
	}

	// Copy the _meta markdown files verbatim. Walk schema.FS for
	// every path under v1/_meta/; recreate the directory tree under
	// outDir.
	metaDir := path.Join(schema.Version, "_meta")
	_ = fs.WalkDir(schema.FS, metaDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		body, err := schema.FS.ReadFile(p)
		if err != nil {
			return nil
		}
		dst := filepath.Join(outDir, "_meta", filepath.Base(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil
		}
		_ = os.WriteFile(dst, body, 0o644)
		return nil
	})
	fmt.Fprintf(cmd.Writer, "wrote %d schemas to %s\n", len(all), outDir)
	return nil
}

// bundleDoc is the D087 single-file bundle wire shape.
type bundleDoc struct {
	Schema  string                     `json:"$schema"`
	ID      string                     `json:"$id"`
	Version string                     `json:"version"`
	Files   map[string]json.RawMessage `json:"files"`
}

func runSchemaBundle(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) != 1 {
		return cli.Exit("schema bundle: expected exactly one positional argument: <out-file>", 2)
	}
	outFile := args[0]

	all, err := schema.V1Schemas()
	if err != nil {
		return cli.Exit(fmt.Sprintf("schema bundle: %v", err), 1)
	}
	doc := bundleDoc{
		Schema:  "https://json-schema.org/draft/2020-12/schema",
		ID:      "urn:prism:schema:v1:bundle",
		Version: schema.Version,
		Files:   make(map[string]json.RawMessage, len(all)),
	}
	for _, name := range sortedKeys(all) {
		doc.Files[name+".schema.json"] = json.RawMessage(all[name])
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return cli.Exit(fmt.Sprintf("schema bundle: marshal: %v", err), 1)
	}
	if err := os.WriteFile(outFile, body, 0o644); err != nil {
		return cli.Exit(fmt.Sprintf("schema bundle: write %s: %v", outFile, err), 1)
	}
	fmt.Fprintf(cmd.Writer, "wrote bundle with %d schemas to %s\n", len(all), outFile)
	return nil
}

func sortedKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// _ keeps strings referenced; the sorted-keys helper has no other
// import dependency on it.
var _ = strings.TrimSpace
