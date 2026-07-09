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
//
//   probe-runner.mjs error <wasm> <exec> <specPath>
//       loads the wasm and calls globalThis.prism.validate(spec) on an
//       INVALID spec, expecting a structured {"ok":false,...} error
//       envelope (with rendered fixups) to be RETURNED — not thrown.
//       This is the E6-S2 regression guard: rendering a fixup used to
//       reach text/template → reflect.Value.MethodByName, which TinyGo
//       does not implement, so any error path trapped the wasm with
//       "RuntimeError: unreachable" and no envelope ever reached JS. If
//       the module traps, prism.validate throws and node exits non-zero
//       (stderr carries the RuntimeError). On success the envelope JSON
//       is printed to stdout. Exits non-zero if validate returned a
//       non-envelope or an ok result.
//
//   probe-runner.mjs pipeline <wasm> <exec> <specPath> [sidecarPath]
//       loads the wasm and drives the FULL pipeline over the exported
//       globalThis.prism surface: validate(spec) → plan(spec, datasets)
//       → execute(spec, datasets) → render(scene) → SVG, printed to
//       stdout. The optional sidecar JSON carries
//           {"datasets": {name: ref}, "resolver": {ref: {values, fields}}}
//       — `datasets` is forwarded as the datasetsJSON registry argument
//       (a `data: {name}` reference then resolves to a SourceNode), and
//       `resolver` backs a synchronous prism.setDataResolver serving
//       those rows. This exercises both the JS-supplied DataResolver
//       seam and the previously-Pulse transforms (crosstab/regression)
//       that require a source-rooted chain. Any error envelope returned
//       by an export aborts with a non-zero exit.

import { readFile } from "node:fs/promises";

const [, , mode, wasmPath, execPath, arg, arg2] = process.argv;
if (!mode || !wasmPath || !execPath || !arg) {
  console.error("usage: probe-runner.mjs <render|global|pipeline> <wasm> <exec> <scenePath|globalName|specPath> [resolverDataPath]");
  process.exit(2);
}

// isErrorEnvelope reports whether an export return value is one of the
// {ok:false,...} JSON envelopes the wasm boundary emits on failure.
function isErrorEnvelope(s) {
  return typeof s !== "string" || s.startsWith(`{"ok":false`);
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
} else if (mode === "error") {
  await waitFor(
    () => globalThis.prism && typeof globalThis.prism.validate === "function",
    "globalThis.prism.validate",
  );
  const specText = await readFile(arg, "utf-8");
  // If the wasm traps (the pre-E6-S2 TinyGo behaviour), this call throws
  // a WebAssembly RuntimeError and the process exits non-zero — the Go
  // harness treats that as a failure.
  const res = globalThis.prism.validate(specText);
  if (typeof res !== "string") {
    console.error("probe-runner: prism.validate returned a non-string:", res);
    process.exit(4);
  }
  if (!res.startsWith(`{"ok":false`)) {
    console.error("probe-runner: expected an {ok:false} error envelope, got:", res);
    process.exit(4);
  }
  process.stdout.write(res.endsWith("\n") ? res : res + "\n");
  process.exit(0);
} else if (mode === "pipeline") {
  await waitFor(
    () =>
      globalThis.prism &&
      typeof globalThis.prism.validate === "function" &&
      typeof globalThis.prism.plan === "function" &&
      typeof globalThis.prism.execute === "function" &&
      typeof globalThis.prism.render === "function" &&
      typeof globalThis.prism.setDataResolver === "function",
    "globalThis.prism pipeline exports",
  );

  const specText = await readFile(arg, "utf-8");

  // Optional sidecar: a datasets registry (name → ref) plus the
  // JS-supplied DataResolver rows the wasm bridge serves synchronously
  // (it cannot await a Promise).
  let datasetsJSON = "";
  if (arg2) {
    const sidecar = JSON.parse(await readFile(arg2, "utf-8"));
    if (sidecar.datasets) {
      datasetsJSON = JSON.stringify(sidecar.datasets);
    }
    if (sidecar.resolver) {
      globalThis.prism.setDataResolver((ref) => sidecar.resolver[ref] ?? null);
    }
  }

  const validated = globalThis.prism.validate(specText);
  if (isErrorEnvelope(validated)) {
    console.error("probe-runner: prism.validate returned an error envelope:", validated);
    process.exit(4);
  }

  const planned = globalThis.prism.plan(specText, datasetsJSON);
  if (isErrorEnvelope(planned)) {
    console.error("probe-runner: prism.plan returned an error envelope:", planned);
    process.exit(4);
  }

  const sceneJSON = globalThis.prism.execute(specText, datasetsJSON);
  if (isErrorEnvelope(sceneJSON)) {
    console.error("probe-runner: prism.execute returned an error envelope:", sceneJSON);
    process.exit(4);
  }

  const svg = globalThis.prism.render(sceneJSON);
  if (isErrorEnvelope(svg)) {
    console.error("probe-runner: prism.render returned an error envelope:", svg);
    process.exit(4);
  }

  process.stdout.write(svg.endsWith("\n") ? svg : svg + "\n");
  process.exit(0);
} else {
  console.error(`probe-runner: unknown mode ${mode}`);
  process.exit(2);
}
