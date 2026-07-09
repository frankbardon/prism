// pulse_firewall_test.go locks in the headline outcome of the Pulse-eviction
// effort: the Prism core is Pulse-free, and it stays that way. Once
// github.com/frankbardon/pulse leaves go.mod, nothing may pull it — or its
// former expression engine, github.com/expr-lang/expr — back into the import
// graph. expr-lang was only ever present transitively through Pulse, so
// dropping Pulse must also shed it; firewalling both prevents a silent
// re-entry that would undo the wasm-size win.
//
// The gate shells out to `go list -deps` over the whole host module graph
// (both the regular and the -test graphs) plus the wasm entry point under
// GOOS=js/GOARCH=wasm, and fails if any dependency path matches a forbidden
// module root. This is the literal "done" criterion: the core does not
// transitively acquire Pulse or expr-lang on any platform.
package gates

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// forbiddenImportPaths lists the module roots that must never re-enter the
// Prism import graph. A match on any substring fails the firewall.
var forbiddenImportPaths = []string{
	"github.com/frankbardon/pulse",
	"github.com/expr-lang/expr",
}

// firewallTarget describes one `go list -deps` invocation and the extra
// environment it runs under (e.g. the wasm platform for the browser entry).
type firewallTarget struct {
	name     string
	args     []string
	extraEnv []string
}

func TestPulseFreeImportFirewall(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}

	targets := []firewallTarget{
		// Whole host module: covers the core packages and every host
		// command. The -test variant additionally covers imports pulled in
		// only by _test.go files.
		{name: "host", args: []string{"list", "-deps", "./..."}},
		{name: "host_test", args: []string{"list", "-deps", "-test", "./..."}},
		// The WASM entry is build-gated to js/wasm, so it is invisible to a
		// default-platform `go list`. List it explicitly under the wasm
		// platform to firewall the browser build too.
		{
			name:     "wasm_entry",
			args:     []string{"list", "-deps", "./cmd/prismwasm"},
			extraEnv: []string{"GOOS=js", "GOARCH=wasm"},
		},
	}

	for _, tgt := range targets {
		t.Run(tgt.name, func(t *testing.T) {
			cmd := exec.Command("go", tgt.args...)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), tgt.extraEnv...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go %s failed: %v\n%s", strings.Join(tgt.args, " "), err, out)
			}
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				dep := strings.TrimSpace(line)
				if dep == "" {
					continue
				}
				for _, forbidden := range forbiddenImportPaths {
					if strings.Contains(dep, forbidden) {
						t.Errorf("forbidden module leaked into %s import graph: %q matches %q.\n"+
							"The Prism core is Pulse-free — do not reintroduce %s or its transitive deps.",
							tgt.name, dep, forbidden, forbidden)
					}
				}
			}
		})
	}
}
