// probe-runner.mjs — parametric wasm driver for the E5-S2 TinyGo↔Go
// float-parity harness. Unlike main.mjs (which is hardwired to
// bin/prism.wasm), this takes the wasm binary + its MATCHING
// wasm_exec.js as arguments so the caller can point it at either a
// standard-Go build or a TinyGo build. TinyGo ships its own
// wasm_exec.js that is NOT byte-compatible with Go's, so the loader
// must always be paired with the binary that produced it.
//
// Two modes:
//
//   probe-runner.mjs render <wasm> <exec> <scenePath>
//       loads the wasm, calls globalThis.prism.render(sceneJSON), and
//       prints the SVG to stdout (newline-terminated, matching
//       `prism plot`).
//
//   probe-runner.mjs global <wasm> <exec> <globalName>
//       loads the wasm and prints the string value the module parked on
//       globalThis[globalName] (used by the floatprobe entrypoint).

import { readFile } from "node:fs/promises";

const [, , mode, wasmPath, execPath, arg] = process.argv;
if (!mode || !wasmPath || !execPath || !arg) {
  console.error("usage: probe-runner.mjs <render|global> <wasm> <exec> <scenePath|globalName>");
  process.exit(2);
}

// wasm_exec.js installs `globalThis.Go`. Both Go's and TinyGo's copies
// are classic scripts, not ES modules, so load via `new Function` eval
// rather than `import`.
const execSource = await readFile(execPath, "utf-8");
new Function("globalThis", execSource)(globalThis);

const wasmBytes = await readFile(wasmPath);
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);

// The probe/main entrypoints park on `select {}`, so go.run never
// resolves. Poll globalThis for the signal the module has finished
// wiring its exports (works identically for the Go and TinyGo loaders).
function waitFor(predicate, label) {
  return new Promise((res, rej) => {
    let attempts = 0;
    const tick = () => {
      if (predicate()) return res();
      if (++attempts > 200) return rej(new Error(`timed out waiting for ${label}`));
      setTimeout(tick, 0);
    };
    tick();
  });
}

go.run(instance); // fire and forget

if (mode === "render") {
  await waitFor(
    () => globalThis.prism && typeof globalThis.prism.render === "function",
    "globalThis.prism.render",
  );
  const sceneText = await readFile(arg, "utf-8");
  const svg = globalThis.prism.render(sceneText);
  if (typeof svg !== "string" || svg.startsWith(`{"ok":false`)) {
    console.error("probe-runner: prism.render returned an error envelope:", svg);
    process.exit(4);
  }
  process.stdout.write(svg.endsWith("\n") ? svg : svg + "\n");
  process.exit(0);
} else if (mode === "global") {
  await waitFor(() => typeof globalThis[arg] === "string", `globalThis.${arg}`);
  process.stdout.write(globalThis[arg]);
  process.exit(0);
} else {
  console.error(`probe-runner: unknown mode ${mode}`);
  process.exit(2);
}
