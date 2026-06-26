// mcp_firewall_test.go enforces the importable-core insulation: the
// SDK-agnostic `github.com/frankbardon/prism/mcp` package must NOT pull
// any MCP SDK into its transitive import set. The go-sdk wiring lives
// only in the `mcp/gosdk` adapter subpackage, which is intentionally out
// of scope here.
//
// The gate shells out to `go list -deps` (both the regular and the
// `-test` import graphs, so the test binary is covered too) and fails if
// any dependency path matches a known MCP SDK. This is the literal "done"
// criterion: consumers that import the core do not transitively acquire an
// MCP SDK.
package gates

import (
	"os/exec"
	"strings"
	"testing"
)

// mcpSDKImportPaths lists the module roots that signal an MCP SDK leak into
// the core. A match on any of these substrings fails the firewall.
var mcpSDKImportPaths = []string{
	"github.com/modelcontextprotocol/go-sdk",
	"github.com/mark3labs/mcp-go",
}

// corePackage is the SDK-free package whose import set is firewalled.
const corePackage = "github.com/frankbardon/prism/mcp"

func TestMCPCoreImportFirewall(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}

	// Cover both the package import graph and the test binary's graph: a
	// _test.go file that imports an MCP SDK would couple the core's test
	// build to it just as surely as a non-test import.
	for _, args := range [][]string{
		{"list", "-deps", corePackage},
		{"list", "-deps", "-test", corePackage},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			cmd := exec.Command("go", args...)
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go %s failed: %v\n%s", strings.Join(args, " "), err, out)
			}
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				dep := strings.TrimSpace(line)
				if dep == "" {
					continue
				}
				for _, sdk := range mcpSDKImportPaths {
					if strings.Contains(dep, sdk) {
						t.Errorf("MCP SDK leaked into %s transitive imports: %q matches %q.\n"+
							"The core must stay SDK-free — keep MCP SDK wiring in mcp/gosdk.",
							corePackage, dep, sdk)
					}
				}
			}
		})
	}
}
