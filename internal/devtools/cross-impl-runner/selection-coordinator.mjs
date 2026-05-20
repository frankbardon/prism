// selection-coordinator.mjs — TestPrismSelectionCoordinator (D082).
//
// Wraps two <prism-chart> instances (overview + detail) sharing
// selection id "brand_focus" under a <prism-coordinator>. Fires a
// click on a bar in the overview chart, asserts:
//   1. detail chart's prism:select listener receives the re-dispatch
//      with the same state and detail.__prism_coordinated__: true.
//   2. detail chart's handle now stores the same selection state.
//   3. No infinite loop — the coordinator's loop guard ignores
//      events it re-dispatches itself.

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

// Load both scene fixtures; route the right one per fetch URL.
const overviewPath = resolve(REPO, "testdata", "browser", "selections-scenes", "selection_cross_chart_overview.json");
const detailPath   = resolve(REPO, "testdata", "browser", "selections-scenes", "selection_cross_chart_detail.json");
const overviewText = await readFile(overviewPath, "utf-8");
const detailText   = await readFile(detailPath,   "utf-8");

globalThis.fetch = async (url) => {
  const txt = String(url).includes("overview") ? overviewText : detailText;
  return {
    ok: true, status: 200,
    arrayBuffer: async () => new TextEncoder().encode(txt).buffer,
    text:        async () => txt,
    json:        async () => JSON.parse(txt),
  };
};

import { bootstrapWasm, tick } from "./wasm-bootstrap.mjs";
await bootstrapWasm(REPO);

await import(resolve(REPO, "static/vendor/prism/prism-element.mjs"));

const coord = document.createElement("prism-coordinator");
const overview = document.createElement("prism-chart");
overview.id = "overview";
overview.setAttribute("src", "/scenes/overview.json");
const detail = document.createElement("prism-chart");
detail.id = "detail";
detail.setAttribute("src", "/scenes/detail.json");

coord.appendChild(overview);
coord.appendChild(detail);
document.body.appendChild(coord);
await new Promise((r) => setTimeout(r, 100));

if (!overview._handle || !detail._handle) fail("charts did not mount");

let detailEvents = 0;
let coordinatedFlag = false;
detail.addEventListener("prism:select", (ev) => {
  detailEvents++;
  if (ev.detail && ev.detail["__prism_coordinated__"]) coordinatedFlag = true;
});

let loopGuardCount = 0;
overview.addEventListener("prism:select", (ev) => {
  loopGuardCount++;
});

// Find a clickable bar in overview's shadow tree.
const overviewBar = overview.shadowRoot.querySelector('rect[data-prism-datum-row="1"]');
if (!overviewBar) fail("overview shadow has no bar row 1");

overviewBar.dispatchEvent(new window.MouseEvent("click", { bubbles: true, composed: true }));
await new Promise((r) => setTimeout(r, 50));

if (loopGuardCount === 0) fail("overview itself didn't fire prism:select (sanity)");
if (loopGuardCount > 5) fail(`overview received ${loopGuardCount} events — likely an event storm`);

if (detailEvents === 0) {
  fail(`detail chart received 0 prism:select events; expected >= 1 re-dispatch`);
}
if (!coordinatedFlag) {
  fail("detail did not see the __prism_coordinated__ marker on the re-dispatch");
}

const detailState = detail._handle.getSelection("brand_focus");
if (!detailState || !Array.isArray(detailState.points) || detailState.points.length === 0) {
  fail(`detail handle.getSelection(brand_focus) = ${JSON.stringify(detailState)}; expected populated`);
}
if (detailState.points[0].row_id !== 1) {
  fail(`detail.points[0].row_id = ${detailState.points[0].row_id}, want 1`);
}

console.error(`PASS: click on overview row 1 → detail received ${detailEvents} re-dispatch(es), no loop (overview events=${loopGuardCount})`);
try { await window.happyDOM?.close(); } catch {}
process.exit(0);
