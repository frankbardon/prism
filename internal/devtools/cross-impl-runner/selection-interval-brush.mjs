// selection-interval-brush.mjs — TestPrismSelectionIntervalBrush.
//
// Render selection_interval_brush.json via <prism-chart>, fire
// synthetic mousedown / mousemove / mouseup events spanning ~25–75%
// of the plot width, and assert a prism:select event fires with
// detail.state.range = {channel:"x", min, max} where min/max are
// inside the x scale's domain and min < max.
//
// Exits 0 / 1. Driven by selection_interval_brush_test.go.

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
globalThis.MouseEvent     = window.MouseEvent;
globalThis.customElements = window.customElements;

const scenePath = resolve(REPO, "testdata", "browser", "selections-scenes", "selection_interval_brush.json");
const sceneText = await readFile(scenePath, "utf-8");
const sceneDoc  = JSON.parse(sceneText);

globalThis.fetch = async () => ({
  ok: true, status: 200,
  arrayBuffer: async () => new TextEncoder().encode(sceneText).buffer,
  text:        async () => sceneText,
  json:        async () => sceneDoc,
});

import { bootstrapWasm, tick } from "./wasm-bootstrap.mjs";
await bootstrapWasm(REPO);

await import(resolve(REPO, "static/vendor/prism/prism-element.mjs"));

const chart = document.createElement("prism-chart");
chart.id = "iv";
chart.setAttribute("src", "/scenes/selection_interval_brush.json");

let captured = null;
chart.addEventListener("prism:select", (ev) => {
  captured = ev.detail;
});

document.body.appendChild(chart);
await new Promise((r) => setTimeout(r, 80));

const root = chart.shadowRoot;
if (!root) fail("shadow root missing");
const svg = root.querySelector("svg");
if (!svg) fail("svg missing");

// happy-dom getBoundingClientRect returns zeros by default; pixel
// math still works because the brush handler subtracts the bounding
// rect's left from clientX — when both are zero, clientX is the
// in-svg coord.

const plot = sceneDoc.grid.cells[0].scene.plot;
const x0 = plot.x;
const x1 = plot.x + plot.w;
const span = x1 - x0;
const downX = x0 + span * 0.25;
const upX   = x0 + span * 0.75;
const y     = plot.y + plot.h / 2;

const mkEv = (type, x) => new window.MouseEvent(type, {
  bubbles: true, composed: true, clientX: x, clientY: y,
});

svg.dispatchEvent(mkEv("mousedown", downX));
svg.dispatchEvent(mkEv("mousemove", (downX + upX) / 2));
svg.dispatchEvent(mkEv("mouseup",   upX));
await new Promise((r) => setTimeout(r, 20));

if (!captured) fail("prism:select did not fire");
if (captured.id !== "brush") fail(`detail.id = ${captured.id}, want brush`);
const st = captured.state;
if (!st || !st.range) fail(`detail.state.range missing: ${JSON.stringify(st)}`);
if (st.range.channel !== "x") fail(`range.channel = ${st.range.channel}, want x`);
if (!(st.range.min < st.range.max)) {
  fail(`expected min < max, got min=${st.range.min}, max=${st.range.max}`);
}
// Domain on the x axis is [1, 30] (day field).
const axis = sceneDoc.grid.cells[0].scene.axes.find(a => a.channel === "x");
const dom0 = Number(axis.scale.domain[0]);
const dom1 = Number(axis.scale.domain[1]);
if (st.range.min < Math.min(dom0, dom1) || st.range.max > Math.max(dom0, dom1)) {
  fail(`range out of domain [${dom0},${dom1}]: min=${st.range.min} max=${st.range.max}`);
}

console.error(`PASS: brush 25–75% of x axis → range {channel:x, min:${st.range.min.toFixed(2)}, max:${st.range.max.toFixed(2)}}`);
try { await window.happyDOM?.close(); } catch {}
process.exit(0);
