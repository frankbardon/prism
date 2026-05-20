// selection-url-state.mjs — TestPrismSelectionURLState (D079).
//
// Asserts:
//   1. serialiseStateMap → "" for empty map.
//   2. serialiseStateMap → "prism-sel:<base64url>" for non-empty.
//   3. deserialiseStateMap round-trips back to the original object.
//   4. Setting window.location.hash to a known payload BEFORE
//      attaching <prism-chart> seeds the chart's selection state on
//      mount.
//   5. Mutating the chart's selection (setSelection) writes the new
//      state to window.location.hash via history.replaceState.
//   6. Payloads > HASH_BUDGET (1024) overflow to localStorage with
//      hash = "prism-sel:overflow".
//
// Exits 0 / 1. Driven by Go test
// internal/devtools/selection_url_state_test.go.

import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { resolve, dirname } from "node:path";

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO = resolve(HERE, "../../..");

function fail(msg) {
  console.error(`FAIL: ${msg}`);
  process.exit(1);
}

let Window;
try {
  ({ Window } = await import("happy-dom"));
} catch (e) {
  fail("happy-dom not installed: " + e.message);
}

const window = new Window({ url: "http://localhost/page" });
globalThis.window         = window;
globalThis.document       = window.document;
globalThis.HTMLElement    = window.HTMLElement;
globalThis.CustomEvent    = window.CustomEvent;
globalThis.customElements = window.customElements;
// happy-dom ships its own history + localStorage.
globalThis.history        = window.history;
globalThis.localStorage   = window.localStorage;
globalThis.location       = window.location;

// Stub fetch — point at the bar_basic scene fixture so chart mounts.
const sceneText = await readFile(
  resolve(REPO, "testdata", "cross_impl", "bar_basic", "scene.json"),
  "utf-8",
);
globalThis.fetch = async () => ({
  ok: true, status: 200,
  arrayBuffer: async () => new TextEncoder().encode(sceneText).buffer,
  text:        async () => sceneText,
  json:        async () => JSON.parse(sceneText),
});

// ---- Test 1: serialise/deserialise round-trip ----
// WASM must be live before prism-element imports (its transitive
// prism.mjs lazy-loads prism.wasm on first render).
const { bootstrapWasm } = await import("./wasm-bootstrap.mjs");
await bootstrapWasm(REPO);

const { serialiseStateMap, deserialiseStateMap } = await import(
  resolve(REPO, "static/vendor/prism/prism-element.mjs"),
);

if (serialiseStateMap({}) !== "") fail("empty map should serialise to ''");
if (serialiseStateMap(null) !== "") fail("null should serialise to ''");

const state = { chart_a: { highlight: { points: [{ layer_id: "layer-0", row_id: 2 }], range: null } } };
const enc = serialiseStateMap(state);
if (!enc.startsWith("prism-sel:")) fail(`expected prism-sel: prefix, got ${enc}`);

const dec = deserialiseStateMap("#" + enc);
if (!dec || JSON.stringify(dec) !== JSON.stringify(state)) {
  fail(`round-trip mismatch:\n  got: ${JSON.stringify(dec)}\n  want:${JSON.stringify(state)}`);
}
if (deserialiseStateMap("#not-prism") !== null) fail("non-prism prefix should decode to null");
if (deserialiseStateMap("#prism-sel:overflow") !== null) fail("overflow marker should decode to null");
if (deserialiseStateMap("") !== null) fail("empty string should decode to null");

// ---- Test 2: chart seeds state from URL hash on mount ----
window.location.hash = "#" + enc;

const chart = document.createElement("prism-chart");
chart.id = "chart_a";
chart.setAttribute("src", "/scenes/bar_basic.json");
document.body.appendChild(chart);
await new Promise((r) => setTimeout(r, 80));

const seeded = chart._handle && chart._handle.getSelection("highlight");
if (!seeded || !Array.isArray(seeded.points) || seeded.points.length !== 1) {
  fail(`chart did not seed selection state from hash. handle.getSelection('highlight') = ${JSON.stringify(seeded)}`);
}
if (seeded.points[0].row_id !== 2) {
  fail(`seeded row_id = ${seeded.points[0].row_id}, want 2`);
}

// ---- Test 3: mutating selection writes to hash ----
const { setSelection } = await import(resolve(REPO, "static/vendor/prism/prism-selection.mjs"));
setSelection(chart._handle, "highlight", { points: [{ layer_id: "layer-0", row_id: 0 }], range: null });
await new Promise((r) => setTimeout(r, 20));

const newHash = window.location.hash;
if (!newHash.startsWith("#prism-sel:")) {
  fail(`expected hash to start with #prism-sel: after mutation, got ${newHash}`);
}
const newMap = deserialiseStateMap(newHash);
if (!newMap || !newMap.chart_a || !newMap.chart_a.highlight) {
  fail(`hash after mutation did not contain chart_a/highlight: ${JSON.stringify(newMap)}`);
}
if (newMap.chart_a.highlight.points[0].row_id !== 0) {
  fail(`hash post-mutation row_id = ${newMap.chart_a.highlight.points[0].row_id}, want 0`);
}

// ---- Test 4: overflow path → localStorage ----
const big = { chart_a: { highlight: { points: [], range: null }, big_sel: { points: [], range: null } } };
for (let i = 0; i < 200; i++) {
  big.chart_a["big_sel"].points.push({ layer_id: "layer-0", row_id: i });
}
const bigEncoded = serialiseStateMap(big);
if (bigEncoded.length <= 1024) {
  fail(`test setup: encoded big payload only ${bigEncoded.length} bytes — need > 1024 to trigger overflow`);
}
// Trigger via the chart's _persistState path by setting a big selection.
setSelection(chart._handle, "big_sel", big.chart_a.big_sel);
await new Promise((r) => setTimeout(r, 20));

const overflowHash = window.location.hash;
if (overflowHash !== "#prism-sel:overflow") {
  fail(`expected overflow hash, got ${overflowHash}`);
}
const lsRaw = window.localStorage.getItem("prism-sel:chart_a");
if (!lsRaw) fail("localStorage missing prism-sel:chart_a after overflow");
const lsParsed = JSON.parse(lsRaw);
if (!lsParsed.chart_a || !lsParsed.chart_a.big_sel || lsParsed.chart_a.big_sel.points.length !== 200) {
  fail(`localStorage overflow payload malformed: ${lsRaw.slice(0, 200)}`);
}

console.error("PASS: serialise/deserialise round-trip + hash seed + mutate-writes-hash + overflow-to-localStorage");
try { await window.happyDOM?.close(); } catch {}
process.exit(0);
