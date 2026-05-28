//go:build !js

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/frankbardon/prism/geodata"
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
	if err := emitGeodataBundle(cmd, outDir); err != nil {
		return err
	}
	return nil
}

// emitGeodataBundle drops the embedded geodata artifacts into
// <outDir>/geodata/. WASM consumers default to fetching from this
// path; static-bundle always emits them so the geoshape mark is
// usable out-of-the-box.
func emitGeodataBundle(cmd *cli.Command, outDir string) error {
	dir := filepath.Join(outDir, "geodata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return cli.Exit(fmt.Sprintf("mkdir %s: %v", dir, err), 1)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), geodata.EmbeddedManifestBytes(), 0o644); err != nil {
		return cli.Exit(fmt.Sprintf("write manifest.json: %v", err), 1)
	}
	count := 1
	for _, tier := range geodata.AllTiers() {
		body, err := geodata.EmbeddedTierBytes(tier)
		if err != nil {
			return cli.Exit(fmt.Sprintf("embed tier %s: %v", tier, err), 1)
		}
		dst := filepath.Join(dir, string(tier)+".geo.json")
		if err := os.WriteFile(dst, body, 0o644); err != nil {
			return cli.Exit(fmt.Sprintf("write %s: %v", dst, err), 1)
		}
		count++
	}
	fmt.Fprintf(cmd.Writer, "static-bundle: wrote %d geodata files to %s\n", count, dir)
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
	wasmBytes, err := os.ReadFile(srcWasm)
	if err != nil {
		return cli.Exit(fmt.Sprintf("read prism.wasm: %v", err), 1)
	}
	dstWasm := filepath.Join(outDir, "prism.wasm")
	if err := os.WriteFile(dstWasm, wasmBytes, 0o644); err != nil {
		return cli.Exit(fmt.Sprintf("copy prism.wasm: %v", err), 1)
	}

	// Emit a gzip companion next to the raw binary. The Go WASM target
	// is ~69 MiB raw but ~12 MiB gzipped; the standalone loader fetches
	// the .gz and decompresses via DecompressionStream so the wire size
	// stays small even on static hosts that do not negotiate
	// Content-Encoding. The raw .wasm remains as a fallback.
	dstWasmGz := filepath.Join(outDir, "prism.wasm.gz")
	gzBytes, err := gzipBytes(wasmBytes)
	if err != nil {
		return cli.Exit(fmt.Sprintf("gzip prism.wasm: %v", err), 1)
	}
	if err := os.WriteFile(dstWasmGz, gzBytes, 0o644); err != nil {
		return cli.Exit(fmt.Sprintf("write prism.wasm.gz: %v", err), 1)
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

	fmt.Fprintf(cmd.Writer, "static-bundle: wrote prism.wasm (%d B), prism.wasm.gz (%d B), wasm_exec.js, index.html (standalone loader) to %s\n", len(wasmBytes), len(gzBytes), outDir)
	return nil
}

// gzipBytes returns the gzip (BestCompression) encoding of src. Used
// to emit the prism.wasm.gz companion so the standalone loader can keep
// the wire payload small without relying on server-side compression.
func gzipBytes(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := gz.Write(src); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
  <link rel="preload" as="fetch" href="prism.wasm.gz" crossorigin>
  <script src="wasm_exec.js"></script>
  <script type="module">
    const go = new Go();
    // Prefer the pre-compressed binary (~12 MiB vs ~69 MiB raw) so the
    // bundle stays small on static hosts that do not negotiate
    // Content-Encoding. DecompressionStream("gzip") ships in current
    // Chrome/Firefox/Safari; fall back to the raw .wasm via
    // instantiateStreaming when it is unavailable or the .gz is absent.
    async function loadInstance() {
      if (typeof DecompressionStream === "function") {
        try {
          const gz = await fetch("prism.wasm.gz");
          if (gz.ok && gz.body) {
            const stream = gz.body.pipeThrough(new DecompressionStream("gzip"));
            const buf = await new Response(stream).arrayBuffer();
            return (await WebAssembly.instantiate(buf, go.importObject)).instance;
          }
        } catch (e) {
          console.warn("prism: gzip load failed, falling back to raw prism.wasm", e);
        }
      }
      return (await WebAssembly.instantiateStreaming(fetch("prism.wasm"), go.importObject)).instance;
    }
    go.run(await loadInstance());
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
