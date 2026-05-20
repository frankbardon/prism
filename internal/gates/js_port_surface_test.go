// js_port_surface_test.go enforces the P17 JS-port trim. After P17,
// the vendored `static/vendor/prism/` directory carries exactly four
// `.mjs` files + a README. JS-side reimplementations of Go pipeline
// stages (scale/axis/ticks/palette/format) and the vendored D3
// modules were deleted; the gate fails the build if any of them
// reappear by accident.
package gates

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestPrismJSPortSurfaceTrimmed(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Walk up to repo root.
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("go.mod not in any parent")
		}
		root = parent
	}

	dir := filepath.Join(root, "static", "vendor", "prism")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}

	got := []string{}
	for _, e := range entries {
		got = append(got, e.Name())
	}
	sort.Strings(got)

	want := []string{
		"README.md",
		"prism-element.mjs",
		"prism-resolver.mjs",
		"prism-selection.mjs",
		"prism.mjs",
	}

	wantSet := make(map[string]bool, len(want))
	for _, n := range want {
		wantSet[n] = true
	}

	// prism.wasm + wasm_exec.js are transient build outputs staged
	// into this directory by `make docs` so mdBook picks them up;
	// .gitignore keeps them out of source control. Allow but do not
	// require their presence so the gate stays green whether or not
	// docs have been built.
	allowedTransient := map[string]bool{
		"prism.wasm":   true,
		"wasm_exec.js": true,
	}

	for _, name := range got {
		if !wantSet[name] && !allowedTransient[name] {
			t.Errorf("unexpected file under static/vendor/prism/: %s (post-P17 surface is %v; remove or document the addition in CLAUDE.md)", name, want)
		}
	}
	for _, name := range want {
		found := false
		for _, g := range got {
			if g == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected file under static/vendor/prism/ is missing: %s", name)
		}
	}
}
