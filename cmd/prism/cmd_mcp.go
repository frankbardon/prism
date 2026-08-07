//go:build !js

package main

import (
	"context"
	"fmt"

	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/internal/observability"
	"github.com/frankbardon/prism/mcpserve"
	"github.com/frankbardon/prism/rpc"
)

// mcpCommand returns the `prism mcp` subcommand. Runs an MCP server
// over stdio so agent hosts (Nexus, Claude Desktop, etc.) can invoke
// prism_plot / prism_validate / prism_describe / prism_examples_search.
func mcpCommand() *cli.Command {
	return &cli.Command{
		Name:  "mcp",
		Usage: "Run a Model Context Protocol server over stdio for agent integration",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "examples-root",
				Usage: "On-disk directory for prism_examples_search to walk instead of the embedded example corpus (default: embedded)",
			},
			datasetsConfigFlag(),
			geodataDirFlag(),
		},
		Action: runMCP,
	}
}

// mcpOptions is the single place `prism mcp`'s flags become mcpserve.Options.
// Every Options field the CLI can influence is populated here and nowhere
// else, so the flag surface is auditable from one function.
//
// It is a pure builder: it reads flag values only, and performs no I/O and no
// process-global mutation (applyGeodataDir and loadDatasetRegistry stay in
// runMCP for that reason). cmd_mcp_test.go guards its completeness — it drives
// every `prism mcp` flag through this function and fails when any exported
// Options field comes back at its zero value, so a newly added field cannot
// silently go unwired.
func mcpOptions(cmd *cli.Command) mcpserve.Options {
	return mcpserve.Options{
		Version:      versionString,
		ExamplesRoot: cmd.String("examples-root"),
		ExamplesFS:   afero.NewOsFs(),
	}
}

func runMCP(ctx context.Context, cmd *cli.Command) error {
	// Point the host geodata loader at the configured directory before
	// the stdio server starts, so prism_plot tool calls on geoshape /
	// geopoint specs resolve tier geometry. Process-global by design.
	applyGeodataDir(cmd)

	registry, err := loadDatasetRegistry(cmd)
	if err != nil {
		return cli.Exit(fmt.Sprintf("load --datasets-config: %v", err), 2)
	}
	impl := &rpc.PrismServer{
		DatasetRegistry: registry,
		Fs:              afero.NewOsFs(),
		ExecOpts:        observability.Hooks(),
	}
	// Hand the configured facade to the shipped in-process runner, which owns
	// the server construction, the tool registration, and the stdio transport.
	// The build version becomes the advertised server identity and the
	// examples-root override (if any) flows through into the tool catalog.
	if err := mcpserve.ServeStdio(impl, mcpOptions(cmd)); err != nil {
		return cli.Exit(fmt.Sprintf("mcp serve: %v", err), 1)
	}
	return nil
}
