// selection-hit-test-attrs.mjs — TestPrismSelectionHitTestAttrs (D077).
//
// Renders the bar_basic scene fixture via prism.mjs into happy-dom,
// asserts every <rect class="prism-mark-bar"> carries
// data-prism-datum-row + its parent <g> carries data-prism-layer.
//
// Exits 0 / 1.

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

const scenePath = resolve(REPO, "testdata", "cross_impl", "bar_basic", "scene.json");
const scene = JSON.parse(await readFile(scenePath, "utf-8"));

const { render } = await import(resolve(REPO, "static/vendor/prism/prism.mjs"));
const handle = render(scene, document.body);
if (!handle) fail("render returned no handle");

const layers = document.querySelectorAll('g[data-prism-layer]');
if (layers.length === 0) fail("no <g data-prism-layer> in rendered SVG");
for (const lg of layers) {
  const lid = lg.getAttribute("data-prism-layer");
  if (lid !== "layer-0") fail(`unexpected data-prism-layer value: ${lid}`);
}

const bars = document.querySelectorAll('rect.prism-mark-bar');
if (bars.length !== 3) fail(`expected 3 bars, got ${bars.length}`);
for (let i = 0; i < bars.length; i++) {
  const raw = bars[i].getAttribute("data-prism-datum-row");
  if (raw !== String(i)) fail(`bar ${i} data-prism-datum-row = ${raw}, want ${i}`);
}

console.error(`PASS: 3 bars + 1 layer carry hit-test attrs`);
try { await window.happyDOM?.close(); } catch {}
process.exit(0);
