// selection-point-dispatch.mjs — TestPrismSelectionPointDispatch.
//
// Render selection_point_bar.json via <prism-chart>, dispatch a
// click on the first <rect data-prism-datum-row="0">, assert a
// prism:select CustomEvent fires on the host with detail
// {id: "highlight", state: {points: [{layer_id: "layer-0", row_id: 0}], range: null}}.
//
// Exits 0 / 1. Driven by
// internal/devtools/selection_point_dispatch_test.go.

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

const window = new Window({ url: "http://localhost/" });
globalThis.window         = window;
globalThis.document       = window.document;
globalThis.HTMLElement    = window.HTMLElement;
globalThis.CustomEvent    = window.CustomEvent;
globalThis.customElements = window.customElements;
globalThis.fetch          = null; // overridden below

const scenePath = resolve(REPO, "testdata", "browser", "selections-scenes", "selection_point_bar.json");
const sceneText = await readFile(scenePath, "utf-8");

globalThis.fetch = async () => ({
  ok: true, status: 200,
  arrayBuffer: async () => new TextEncoder().encode(sceneText).buffer,
  text:        async () => sceneText,
  json:        async () => JSON.parse(sceneText),
});

await import(resolve(REPO, "static/vendor/prism/prism-element.mjs"));

const chart = document.createElement("prism-chart");
chart.id = "pt";
chart.setAttribute("src", "/scenes/selection_point_bar.json");

let captured = null;
chart.addEventListener("prism:select", (ev) => {
  captured = ev.detail;
});

document.body.appendChild(chart);
await new Promise((r) => setTimeout(r, 80));

const root = chart.shadowRoot;
if (!root) fail("shadow root missing");
const target = root.querySelector('rect[data-prism-datum-row="0"]');
if (!target) {
  fail(`no <rect data-prism-datum-row="0"> in shadow tree. children=${root.childElementCount}, svg=${root.querySelector("svg")?.outerHTML?.slice(0, 200)}`);
}

target.dispatchEvent(new window.MouseEvent("click", { bubbles: true, composed: true }));
await new Promise((r) => setTimeout(r, 20));

if (!captured) fail("prism:select did not fire after click");
if (captured.id !== "highlight") fail(`detail.id = ${captured.id}, want highlight`);
const st = captured.state;
if (!st || !Array.isArray(st.points)) fail(`detail.state.points missing: ${JSON.stringify(st)}`);
if (st.points.length !== 1) fail(`points len = ${st.points.length}, want 1`);
if (st.points[0].row_id !== 0) fail(`points[0].row_id = ${st.points[0].row_id}, want 0`);
if (st.points[0].layer_id !== "layer-0") fail(`points[0].layer_id = ${st.points[0].layer_id}, want layer-0`);

console.error(`PASS: click on bar row 0 → prism:select {id:highlight, points:[{layer-0,0}]}`);
try { await window.happyDOM?.close(); } catch {}
process.exit(0);
