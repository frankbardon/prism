// main.mjs — Node entrypoint for TestCrossImplSVGParity (P17).
//
// Usage:
//   node main.mjs <fixture-name>
//
// Reads:    testdata/cross_impl/<fixture>/scene.json
// Renders:  via bin/prism.wasm (loaded into Node through
//           bin/wasm_exec.js)
// Writes:   testdata/cross_impl/<fixture>/wasm.svg
//
// The harness replaced its P12 JS-port path in P17: now it loads
// the same Go binary that runs in browsers and asserts byte-equal
// SVG against `prism plot`. Drift signals a Go compiler regression
// or a non-deterministic stage, not a JS port mistake.

import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { resolve, dirname } from "node:path";

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO = resolve(HERE, "../../..");

const fixture = process.argv[2];
if (!fixture) {
  console.error("usage: main.mjs <fixture>");
  process.exit(2);
}

const scenePath = resolve(REPO, "testdata", "cross_impl", fixture, "scene.json");
const outPath   = resolve(REPO, "testdata", "cross_impl", fixture, "wasm.svg");
const wasmPath  = resolve(REPO, "bin", "prism.wasm");
const execPath  = resolve(REPO, "bin", "wasm_exec.js");

const sceneText = await readFile(scenePath, "utf-8");

// wasm_exec.js installs `globalThis.Go`. The harness loads it via
// dynamic eval rather than `await import(...)` because Go ships it
// as a classic script, not an ES module.
const execSource = await readFile(execPath, "utf-8");
new Function("globalThis", execSource)(globalThis);

const wasmBytes = await readFile(wasmPath);
const go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);

// go.run blocks until the WASM module returns. Our cmd/prismwasm
// main() ends with `select{}` so the run promise never resolves;
// we register a Promise that fulfils as soon as the exports show
// up on globalThis.prism, and race them.
const ready = new Promise((res, rej) => {
  let attempts = 0;
  const tick = () => {
    if (globalThis.prism && typeof globalThis.prism.render === "function") return res();
    if (++attempts > 100) return rej(new Error("prism.wasm loaded but globalThis.prism never populated"));
    setTimeout(tick, 0);
  };
  tick();
});
go.run(instance); // fire and forget
await ready;

const svg = globalThis.prism.render(sceneText);
if (typeof svg !== "string" || svg.startsWith(`{"ok":false`)) {
  console.error("main.mjs: prism.render returned an error envelope:", svg);
  process.exit(4);
}

await writeFile(outPath, svg.endsWith("\n") ? svg : svg + "\n", "utf-8");
console.error(`main.mjs: wrote ${outPath} (${svg.length} bytes)`);
process.exit(0);
