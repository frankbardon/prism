// conditions-brush.mjs — TestPrismConditionsBrush (tier1-01).
//
// Boots happy-dom, imports prism-selection.mjs directly, builds a
// minimal synthetic SceneHandle with a single layer holding two marks.
// Both marks carry a fill-condition keyed on "brush". Asserts:
//   1. setSelection(handle, "brush", {points: [{layer_id:"layer-0", row_id:1}]})
//      flips mark[1]'s fill to WhenValue while leaving mark[0] alone…
//      because applyConditions writes WhenValue when state is non-empty
//      and Otherwise when empty. (Per-mark match-by-row is a job for
//      applySelection's classes; conditions key on selection-name +
//      state-active, not per-mark, in v1.)
//   2. setSelection(handle, "brush", {}) reverts both marks to Otherwise.

import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

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

const w = new Window({ url: "http://localhost/" });
globalThis.window         = w;
globalThis.document       = w.document;
globalThis.HTMLElement    = w.HTMLElement;
globalThis.CustomEvent    = w.CustomEvent;
globalThis.CSS            = w.CSS;

const sel = await import(resolve(REPO, "static/vendor/prism/prism-selection.mjs"));

// Build a fake SVG with two rects carrying data-prism-datum-row.
const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
const layerG = document.createElementNS("http://www.w3.org/2000/svg", "g");
layerG.setAttribute("class", "prism-layer");
layerG.setAttribute("data-prism-layer", "layer-0");
svg.appendChild(layerG);
const rectA = document.createElementNS("http://www.w3.org/2000/svg", "rect");
rectA.setAttribute("data-prism-datum-row", "0");
rectA.setAttribute("fill", "#cbd5e1");
const rectB = document.createElementNS("http://www.w3.org/2000/svg", "rect");
rectB.setAttribute("data-prism-datum-row", "1");
rectB.setAttribute("fill", "#cbd5e1");
layerG.appendChild(rectA);
layerG.appendChild(rectB);
document.body.appendChild(svg);

// Synthetic SceneHandle with the conditions slice populated.
const handle = {
  _svg: svg,
  _root: svg,
  _selections: [{ id: "brush", kind: "interval" }],
  _doc: {
    grid: {
      cells: [{
        scene: {
          selections: [{ id: "brush", kind: "interval" }],
          layers: [{
            id: "layer-0",
            marks: [
              {
                datum: { layer_id: "layer-0", row_id: 0 },
                conditions: [{
                  attr: "fill",
                  selection: "brush",
                  when_value: "#22c55e",
                  otherwise: "#cbd5e1",
                }],
              },
              {
                datum: { layer_id: "layer-0", row_id: 1 },
                conditions: [{
                  attr: "fill",
                  selection: "brush",
                  when_value: "#22c55e",
                  otherwise: "#cbd5e1",
                }],
              },
            ],
          }],
        },
      }],
    },
  },
};

// Fire a brush selection — both marks should switch to WhenValue.
sel.setSelection(handle, "brush", { range: { min: 0.5, max: 1.0 } }, { silent: true });
if (rectA.getAttribute("fill") !== "#22c55e") fail(`rectA fill after brush = ${rectA.getAttribute("fill")}; want #22c55e`);
if (rectB.getAttribute("fill") !== "#22c55e") fail(`rectB fill after brush = ${rectB.getAttribute("fill")}; want #22c55e`);

// Clear → revert to Otherwise.
sel.setSelection(handle, "brush", {}, { silent: true });
if (rectA.getAttribute("fill") !== "#cbd5e1") fail(`rectA fill after clear = ${rectA.getAttribute("fill")}; want #cbd5e1`);
if (rectB.getAttribute("fill") !== "#cbd5e1") fail(`rectB fill after clear = ${rectB.getAttribute("fill")}; want #cbd5e1`);

console.error("PASS: conditions-brush flipped fill on selection + reverted on clear");
try { await w.happyDOM?.close(); } catch {}
process.exit(0);
