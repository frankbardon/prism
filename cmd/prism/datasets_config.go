package main

import (
	"github.com/spf13/afero"
	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/resolve"
)

// datasetsConfigFlag returns the shared --datasets-config flag.
// The flag accepts a JSON file path holding `{"datasets": {...}}`.
// Empty value means "no file"; env-loaded aliases (PRISM_DATASETS)
// still apply.
func datasetsConfigFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:  "datasets-config",
		Value: "",
		Usage: "Path to a JSON dataset registry ({\"datasets\":{...}}); overrides PRISM_DATASETS",
	}
}

// loadDatasetRegistry builds the chained DatasetRegistry for the
// running CLI command. Precedence: --datasets-config (highest) →
// PRISM_DATASETS env. Both layers are always queried (file misses
// fall through to env).
func loadDatasetRegistry(cmd *cli.Command) (resolve.DatasetRegistry, error) {
	envReg := resolve.LoadDatasetRegistryEnv()
	cfgPath := cmd.String("datasets-config")
	if cfgPath == "" {
		return envReg, nil
	}
	fileReg, err := resolve.LoadDatasetRegistryFile(cfgPath, afero.NewOsFs())
	if err != nil {
		return nil, err
	}
	return resolve.ChainDatasetRegistries(fileReg, envReg), nil
}
