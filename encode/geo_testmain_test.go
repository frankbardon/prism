package encode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/frankbardon/prism/geodata"
)

// TestMain points the host geodata loader at the committed tier files
// under the repo's geodata/ directory. The host build no longer embeds
// tier geometry (see geodata/geometry_host.go), so geoshape encode tests
// that fall back to geodata.DefaultStore() need an explicit directory.
//
// Runtime wiring of the directory from a CLI flag / env var lands in a
// later story; this shim only serves the encode package's tests.
func TestMain(m *testing.M) {
	if dir := findGeodataDir(); dir != "" {
		geodata.SetHostBundleDir(dir)
	}
	os.Exit(m.Run())
}

func findGeodataDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(cwd, "geodata")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return ""
		}
		cwd = parent
	}
}
