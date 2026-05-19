// Command gen-fixtures regenerates the deterministic .pulse fixtures
// used by Prism tests. Run from the repo root:
//
//	go run ./internal/devtools/gen-fixtures
//
// The seed (42) is hard-coded so re-runs produce byte-identical output
// — fixture diffs in git always reflect a real spec/seed change.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/frankbardon/pulse"
	"github.com/frankbardon/pulse/synth"
	"github.com/spf13/afero"
)

const fixtureSeed int64 = 42

// fixture pairs an on-disk synth spec with the .pulse output path.
type fixture struct {
	specPath   string
	outputPath string
}

var fixtures = []fixture{
	{
		specPath:   "testdata/cohorts/tiny.synth.json",
		outputPath: "testdata/cohorts/tiny.pulse",
	},
}

func main() {
	root, err := repoRoot()
	if err != nil {
		log.Fatalf("locate repo root: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		log.Fatalf("chdir(%s): %v", root, err)
	}

	p, err := pulse.New(pulse.Options{FS: afero.NewOsFs()})
	if err != nil {
		log.Fatalf("pulse.New: %v", err)
	}

	for _, fx := range fixtures {
		if err := generate(p, fx); err != nil {
			log.Fatalf("generate %s: %v", fx.outputPath, err)
		}
		fmt.Printf("ok %s\n", fx.outputPath)
	}
}

func generate(p *pulse.Pulse, fx fixture) error {
	raw, err := os.ReadFile(fx.specPath)
	if err != nil {
		return fmt.Errorf("read spec: %w", err)
	}
	spec, err := synth.ParseSpec(raw)
	if err != nil {
		return fmt.Errorf("parse spec: %w", err)
	}
	if _, err := p.Synth(context.Background(), spec, fx.outputPath, pulse.SynthOptions{Seed: fixtureSeed}); err != nil {
		return fmt.Errorf("synth: %w", err)
	}
	return nil
}

// repoRoot walks up from the working directory until it finds a go.mod
// with the prism module path. Lets `go run` work from any subdirectory.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
			if len(data) > 0 && containsModule(string(data), "github.com/frankbardon/prism") {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod containing prism module found above %s", dir)
		}
		dir = parent
	}
}

func containsModule(goMod, mod string) bool {
	for i := 0; i+len("module ")+len(mod) <= len(goMod); i++ {
		if goMod[i:i+len("module ")] == "module " {
			rest := goMod[i+len("module "):]
			if len(rest) >= len(mod) && rest[:len(mod)] == mod {
				return true
			}
		}
	}
	return false
}
