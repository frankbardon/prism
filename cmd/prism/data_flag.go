package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/resolve"
)

// dataFlag returns the shared --data flag. It accepts a path to a JSON
// file holding a flat array of row objects (`[{"col": val}, ...]`).
// The loaded rows satisfy the spec's `data.source` / `data.ref`
// binding so a Pulse-free spec that names an external cohort can still
// be plotted, planned, executed, and encoded from the host CLI. Inline
// `data.values` in the spec needs no flag and always works.
func dataFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:  "data",
		Value: "",
		Usage: "Path to a JSON rows file ([{col: val}, ...]) supplying data for the spec's source/ref binding",
	}
}

// loadDataResolver reads the --data file (if any) and adapts its rows
// into a resolve.DataResolver. An empty flag yields a nil resolver
// (inline `data.values` still works). The resolver serves the loaded
// rows for every ref: the host CLI supplies a single materialized
// dataset, so whichever source/ref/name the spec declares binds to it.
func loadDataResolver(cmd *cli.Command) (resolve.DataResolver, error) {
	path := cmd.String("data")
	if path == "" {
		return nil, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read --data %s: %w", path, err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("decode --data %s: %w", path, err)
	}
	ds := &resolve.Dataset{Values: rows}
	return resolve.DataResolverFunc(func(_ context.Context, _ string) (*resolve.Dataset, error) {
		return ds, nil
	}), nil
}

// hostResolver builds the DefaultResolver the host CLI wires into
// build.Options.Resolver. When --data supplied rows the resolver's
// inline path serves them for `data.source` refs; otherwise it is the
// plain Pulse-free resolver (every ref reports
// PRISM_RESOLVE_REF_UNRESOLVED).
func hostResolver(dr resolve.DataResolver) *resolve.DefaultResolver {
	if dr == nil {
		return resolve.New(nil)
	}
	return resolve.NewWithData(nil, dr)
}
