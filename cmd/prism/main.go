// Command prism is the CLI entry point for the Prism visualization library.
//
// In P00 it only responds to `prism version`. Real subcommands land in later
// phases as the spec, validator, planner, and renderers come online.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("prism v0.0.0-dev")
		return
	}
	fmt.Fprintln(os.Stderr, "prism: no subcommands yet")
	os.Exit(2)
}
