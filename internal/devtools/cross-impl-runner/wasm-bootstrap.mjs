// wasm-bootstrap.mjs — shared loader for the happy-dom selection
// harnesses. Each harness used to mount a <prism-chart> against the
// synchronous JS-port renderer; post-P17 the renderer lives in
// prism.wasm and the harness must (a) install `globalThis.Go` via
// wasm_exec.js and (b) await the new async render() before reading
// shadow roots. This helper bundles both steps so the per-test
// harness stays focused on its assertion.

import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

/**
 * bootstrapWasm loads bin/wasm_exec.js + bin/prism.wasm into the
 * current process. Idempotent; subsequent calls resolve from the
 * cached Promise.
 */
let _ready = null;
export function bootstrapWasm(repoRoot) {
  if (_ready) return _ready;
  _ready = (async () => {
    const execPath = resolve(repoRoot, "bin", "wasm_exec.js");
    const wasmPath = resolve(repoRoot, "bin", "prism.wasm");
    const execSrc = await readFile(execPath, "utf-8");
    new Function("globalThis", execSrc)(globalThis);
    const wasmBytes = await readFile(wasmPath);
    const go = new globalThis.Go();
    const { instance } = await WebAssembly.instantiate(wasmBytes, go.importObject);
    const ready = new Promise((res, rej) => {
      let n = 0;
      const tick = () => {
        if (globalThis.prism && typeof globalThis.prism.render === "function") return res();
        if (++n > 100) return rej(new Error("prism.wasm loaded but globalThis.prism not populated"));
        setTimeout(tick, 0);
      };
      tick();
    });
    go.run(instance);
    await ready;
  })();
  return _ready;
}

/**
 * tick yields to the event loop N times so the harness can wait for
 * async render() chains scheduled during connectedCallback. Replaces
 * the synchronous "read shadow root immediately after construction"
 * pattern the pre-P17 harnesses relied on.
 */
export async function tick(n = 3) {
  for (let i = 0; i < n; i++) {
    await new Promise(r => setTimeout(r, 0));
  }
}
