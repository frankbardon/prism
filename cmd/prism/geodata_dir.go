package main

import (
	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/geodata"
)

// geodataDirFlag returns the shared --geodata-dir flag used by the
// encode-time CLI leaves (plot, scene). It points the host geodata
// loader at a directory of committed tier bundles
// ("<dir>/<tier>.geo.json") so geoshape / geopoint marks can materialise
// geometry. The PRISM_GEODATA environment variable is the env-var source
// for the flag value.
//
// Only plot and scene materialise geometry at encode time. The
// no-execute leaves (validate, plan, inspect) and the data-only leaf
// (execute) consult the embedded manifest and never touch tier geometry,
// so they do not carry this flag.
func geodataDirFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:    "geodata-dir",
		Value:   "",
		Usage:   "Directory of geodata tier bundles (<dir>/<tier>.geo.json) for geoshape/geopoint marks",
		Sources: cli.EnvVars("PRISM_GEODATA"),
	}
}

// applyGeodataDir forwards the resolved --geodata-dir / PRISM_GEODATA
// value into the host geodata loader before any geoshape / geopoint mark
// is encoded. A thin adapter: it hands the path to the library and adds
// no loading logic. An empty value is left untouched so an ambient
// directory (e.g. a test harness) is not clobbered; a geo mark with no
// directory configured surfaces PRISM_GEODATA_DIR_UNSET at encode time.
func applyGeodataDir(cmd *cli.Command) {
	if dir := cmd.String("geodata-dir"); dir != "" {
		geodata.SetHostBundleDir(dir)
	}
}
