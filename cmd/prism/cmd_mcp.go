//go:build !js

package main

import (
	"context"
	"fmt"

	gosdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/internal/observability"
	prismmcp "github.com/frankbardon/prism/mcp"
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
	srv := prismmcp.New(prismmcp.Options{
		PrismServer:  impl,
		ExamplesRoot: cmd.String("examples-root"),
		ExamplesFS:   afero.NewOsFs(),
	})
	if err := srv.Run(ctx, &gosdk.StdioTransport{}); err != nil {
		return cli.Exit(fmt.Sprintf("mcp serve: %v", err), 1)
	}
	return nil
}
