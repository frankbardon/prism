//go:build !js

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	staticfs "github.com/frankbardon/prism/static"
)

// staticBundleCommand returns the `prism static-bundle <out-dir>`
// subcommand. Copies every embedded file under static/vendor/prism/
// to the target directory, preserving the directory structure so
// relative imports keep resolving after extraction (D071).
//
// Use case: downstream app wants to serve the JS port without
// vendoring the prism repo. `prism static-bundle ./public/prism`
// drops the full tree at /public/prism/{prism.mjs, d3/*, ...} →
// host page references /public/prism/prism-element.mjs.
func staticBundleCommand() *cli.Command {
	return &cli.Command{
		Name:      "static-bundle",
		Usage:     "Copy vendored browser assets (prism.mjs, d3 modules) to an output directory",
		ArgsUsage: "<out-dir>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Overwrite existing files in the output directory",
				Value: true,
			},
			&cli.BoolFlag{
				Name:  "wasm",
				Usage: "Also emit prism.wasm + wasm_exec.js + index.html loader (P17 standalone mode)",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "wasm-binary",
				Usage: "Path to a pre-built prism.wasm (default: bin/prism.wasm in cwd, else builds via `go build ./cmd/prismwasm`)",
				Value: "",
			},
		},
		Action: runStaticBundle,
	}
}

func runStaticBundle(_ context.Context, cmd *cli.Command) error {
	args := cmd.Args().Slice()
	if len(args) != 1 {
		return cli.Exit("static-bundle: exactly one positional argument required (output directory)", 2)
	}
	outDir := args[0]
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return cli.Exit(fmt.Sprintf("mkdir %s: %v", outDir, err), 1)
	}

	stripPrefix := staticfs.BundleRoot
	count := 0
	err := fs.WalkDir(staticfs.Tree, stripPrefix, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Relative path under the bundle root.
		rel := strings.TrimPrefix(path, stripPrefix)
		rel = strings.TrimPrefix(rel, "/")
		dst := filepath.Join(outDir, rel)
		if d.IsDir() {
			if rel == "" {
				return nil // already created
			}
			return os.MkdirAll(dst, 0o755)
		}
		// Skip dotfiles (defensive — embed already ignores most, but
		// .DS_Store or editor backups slipping in shouldn't ship).
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		data, err := staticfs.Tree.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embed %s: %w", path, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		count++
		return nil
	})
	if err != nil {
		return cli.Exit(fmt.Sprintf("walk embed: %v", err), 1)
	}
	fmt.Fprintf(cmd.Writer, "static-bundle: wrote %d files to %s\n", count, outDir)

	if cmd.Bool("wasm") {
		if err := emitWasmBundle(cmd, outDir); err != nil {
			return err
		}
	}
	return nil
}

// emitWasmBundle materialises the three additional artifacts that
// the `--wasm` standalone mode requires:
//
//  1. prism.wasm — copied from --wasm-binary, bin/prism.wasm, or
//     built on the fly via `go build ./cmd/prismwasm`.
//  2. wasm_exec.js — copied verbatim from
//     $(go env GOROOT)/lib/wasm/wasm_exec.js (1.24+) or
//     $(go env GOROOT)/misc/wasm/wasm_exec.js (pre-1.24).
//  3. index.html — minimal loader that mounts a <prism-chart> after
//     the WASM module has resolved `globalThis.prism`.
func emitWasmBundle(cmd *cli.Command, outDir string) error {
	srcWasm := cmd.String("wasm-binary")
	if srcWasm == "" {
		srcWasm = filepath.Join("bin", "prism.wasm")
	}
	if _, err := os.Stat(srcWasm); err != nil {
		// Try the on-the-fly build path; works when run from the
		// prism source tree where `./cmd/prismwasm` resolves.
		built, buildErr := buildWasmInline(outDir)
		if buildErr != nil {
			return cli.Exit(fmt.Sprintf("static-bundle --wasm: %s not found and `go build ./cmd/prismwasm` failed (%v); run `make build-wasm` first or pass --wasm-binary", srcWasm, buildErr), 1)
		}
		srcWasm = built
	}
	dstWasm := filepath.Join(outDir, "prism.wasm")
	if err := copyFile(srcWasm, dstWasm); err != nil {
		return cli.Exit(fmt.Sprintf("copy prism.wasm: %v", err), 1)
	}

	wasmExec, err := locateWasmExec()
	if err != nil {
		return cli.Exit(fmt.Sprintf("locate wasm_exec.js: %v", err), 1)
	}
	dstExec := filepath.Join(outDir, "wasm_exec.js")
	if err := copyFile(wasmExec, dstExec); err != nil {
		return cli.Exit(fmt.Sprintf("copy wasm_exec.js: %v", err), 1)
	}

	dstHTML := filepath.Join(outDir, "index.html")
	if err := os.WriteFile(dstHTML, []byte(standaloneLoaderHTML), 0o644); err != nil {
		return cli.Exit(fmt.Sprintf("write index.html: %v", err), 1)
	}

	fmt.Fprintf(cmd.Writer, "static-bundle: wrote prism.wasm, wasm_exec.js, index.html (standalone loader) to %s\n", outDir)
	return nil
}

// buildWasmInline runs `go build -o <outDir>/prism.wasm
// ./cmd/prismwasm` with GOOS=js GOARCH=wasm. Returns the path to
// the produced binary on success. Failure passes the go tool's
// stderr through verbatim.
func buildWasmInline(outDir string) (string, error) {
	dst := filepath.Join(outDir, "prism.wasm")
	build := exec.Command("go", "build",
		"-trimpath", "-buildvcs=false", "-ldflags=-s -w",
		"-o", dst, "./cmd/prismwasm")
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0")
	out, err := build.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return dst, nil
}

// locateWasmExec resolves the canonical wasm_exec.js shipped with
// the Go toolchain. Go 1.24 moved the file from misc/wasm/ to
// lib/wasm/; we prefer the new location and fall back to the old
// one. Returns an error if neither exists.
func locateWasmExec() (string, error) {
	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return "", fmt.Errorf("go env GOROOT: %w", err)
	}
	root := strings.TrimSpace(string(goroot))
	for _, rel := range []string{
		filepath.Join("lib", "wasm", "wasm_exec.js"),
		filepath.Join("misc", "wasm", "wasm_exec.js"),
	} {
		candidate := filepath.Join(root, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("wasm_exec.js not found under %s (looked in lib/wasm/ and misc/wasm/)", root)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// standaloneLoaderHTML is the minimal page that boots the WASM
// runtime and renders the first <prism-chart spec="..."> it finds
// in the DOM. Hosts that want a different layout edit this file or
// roll their own; the bundle just provides a working starting
// point.
const standaloneLoaderHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Prism — standalone</title>
  <link rel="preload" as="fetch" type="application/wasm" href="prism.wasm" crossorigin>
  <script src="wasm_exec.js"></script>
  <script type="module">
    const go = new Go();
    const resp = fetch("prism.wasm");
    const { instance } = await WebAssembly.instantiateStreaming(resp, go.importObject);
    go.run(instance);
  </script>
  <script type="module" src="prism-element.mjs" defer></script>
  <style>
    body { font: 14px/1.5 system-ui, sans-serif; margin: 2rem; }
    prism-chart { display: block; width: 100%; max-width: 800px; aspect-ratio: 4/3; }
  </style>
</head>
<body>
  <h1>Prism — standalone</h1>
  <p>Drop a spec into a <code>&lt;prism-chart spec="…"&gt;</code> tag and the WASM runtime will render it client-side. Replace this loader with your own page to embed Prism in any static site.</p>
  <prism-chart></prism-chart>
</body>
</html>
`
