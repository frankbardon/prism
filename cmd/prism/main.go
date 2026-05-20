// Command prism is the CLI entry point for the Prism visualization library.
//
// In P00 it only responds to `prism version`. Real subcommands land in later
// phases as the spec, validator, planner, and renderers come online.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

const versionString = "prism v1.0.0"

func main() {
	app := newApp()
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newApp() *cli.Command {
	return &cli.Command{
		Name:    "prism",
		Usage:   "Visualization library for .pulse files",
		Version: versionString,
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "Print the prism version",
				Action: func(_ context.Context, _ *cli.Command) error {
					fmt.Println(versionString)
					return nil
				},
			},
			validateCommand(),
			errorsCommand(),
			planCommand(),
			executeCommand(),
			plotCommand(),
			sceneCommand(),
			staticBundleCommand(),
			serveCommand(),
			mcpCommand(),
			inspectCommand(),
			examplesCommand(),
			schemaCommand(),
			initCommand(),
		},
		Action: func(_ context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				_ = cli.ShowAppHelp(c)
				return cli.Exit("", 2)
			}
			return cli.Exit(fmt.Sprintf("prism: unknown command %q", c.Args().First()), 2)
		},
	}
}
