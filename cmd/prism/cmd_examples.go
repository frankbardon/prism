package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"
)

// examplesCommand returns the `prism examples` subcommand group:
// `list` enumerates fixture specs under --root with a one-line
// summary; `show <name>` pretty-prints a single spec.
func examplesCommand() *cli.Command {
	return &cli.Command{
		Name:  "examples",
		Usage: "Browse the curated example spec library",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "root",
				Value: "examples/specs/",
				Usage: "Directory holding example spec files",
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List every example spec under --root with a one-line summary",
				Action: runExamplesList,
			},
			{
				Name:      "show",
				Usage:     "Print the raw JSON for one example spec (matched by stem)",
				ArgsUsage: "<name>",
				Action:    runExamplesShow,
			},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			_ = cli.ShowSubcommandHelp(c)
			return cli.Exit("", 2)
		},
	}
}

func runExamplesList(_ context.Context, cmd *cli.Command) error {
	root := cmd.String("root")
	if root == "" {
		root = "examples/specs/"
	}
	osfs := afero.NewOsFs()

	type entry struct{ name, summary string }
	var entries []entry
	err := afero.Walk(osfs, root, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".json") {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"invalid"+string(filepath.Separator)) {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(path), ".json")
		body, err := afero.ReadFile(osfs, path)
		if err != nil {
			return nil
		}
		entries = append(entries, entry{
			name:    base,
			summary: examplesOneLineSummary(body),
		})
		return nil
	})
	if err != nil {
		return cli.Exit(fmt.Sprintf("examples list: walk %s: %v", root, err), 1)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	tw := tabwriter.NewWriter(cmd.Writer, 0, 2, 2, ' ', 0)
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\n", e.name, e.summary)
	}
	_ = tw.Flush()
	return nil
}

func runExamplesShow(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) != 1 {
		return cli.Exit("examples show: expected exactly one positional argument: <name>", 2)
	}
	name := args[0]
	root := cmd.String("root")
	if root == "" {
		root = "examples/specs/"
	}
	osfs := afero.NewOsFs()

	var match string
	_ = afero.Walk(osfs, root, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".json") {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(path), ".json")
		if base == name {
			match = path
		}
		return nil
	})
	if match == "" {
		return cli.Exit(fmt.Sprintf("examples show: no spec named %q under %s", name, root), 1)
	}
	body, err := afero.ReadFile(osfs, match)
	if err != nil {
		return cli.Exit(fmt.Sprintf("examples show: read %s: %v", match, err), 1)
	}
	// Pretty-print so the user sees a nicely-formatted JSON regardless
	// of how the fixture happens to be authored on disk.
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		// Fall back to raw bytes when the fixture has a comment or
		// other JSON-superset feature the standard parser rejects.
		_, _ = cmd.Writer.Write(body)
		return nil
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		_, _ = cmd.Writer.Write(body)
		return nil
	}
	_, _ = cmd.Writer.Write(out)
	fmt.Fprintln(cmd.Writer)
	return nil
}

// examplesOneLineSummary extracts a short description for a fixture:
// the spec's `title` field when present; else "<mark> chart".
func examplesOneLineSummary(body []byte) string {
	var doc struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Mark        any    `json:"mark"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return ""
	}
	if doc.Title != "" {
		return doc.Title
	}
	if doc.Description != "" {
		return doc.Description
	}
	if s, ok := doc.Mark.(string); ok && s != "" {
		return s + " chart"
	}
	if m, ok := doc.Mark.(map[string]any); ok {
		if t, ok := m["type"].(string); ok && t != "" {
			return t + " chart"
		}
	}
	return "(no title)"
}
