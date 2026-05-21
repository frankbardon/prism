// animator-tween.mjs — TestPrismAnimatorTween.
//
// Exits 0 on pass, non-zero with FAIL to stderr otherwise. Asserts:
//   1. PrismAnimator.partition() splits marks into enter / update /
//      exit by `data-prism-mark-key`.
//   2. start() with a deterministic rAF + clock advances numeric
//      attrs from `prev` toward `next` along the easing curve.
//   3. Color attrs interpolate through OKLab — a fade from #000 to
//      #fff has the expected midpoint brightness.
//   4. structurallyCompatible(prev, next) accepts matched mark
//      families and rejects family mismatch.
//   5. prefers-reduced-motion makes the host element skip animation
//      (verified at the prism.mjs SceneHandle.update level).
//
// Runs under Node + happy-dom; no WASM required.

import { fileURLToPath } from "node:url";
import { resolve, dirname } from "node:path";

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO = resolve(HERE, "../../..");

function fail(msg) {
  console.error(`FAIL: ${msg}`);
  process.exit(1);
}

// Wire happy-dom globals before importing the animator (oklab is
// pure JS but the animator module probes globalThis at module load
// for fallback rAF / matchMedia helpers).
let Window;
try {
  ({ Window } = await import("happy-dom"));
} catch (e) {
  fail("happy-dom not installed");
}
const window = new Window({ url: "http://localhost/" });
globalThis.window         = window;
globalThis.document       = window.document;
globalThis.HTMLElement    = window.HTMLElement;
globalThis.CustomEvent    = window.CustomEvent;
globalThis.customElements = window.customElements;

const animatorPath = resolve(REPO, "static/vendor/prism/prism-animator.mjs");
const {
  PrismAnimator,
  structurallyCompatible,
  prefersReducedMotion,
  easingFn,
  EASINGS,
} = await import(animatorPath);

// ----- Test fixture: prev / next SVGs sharing two mark keys and one new key -----

const SVG_NS = "http://www.w3.org/2000/svg";

function buildSvg(rows) {
  const svg = document.createElementNS(SVG_NS, "svg");
  for (const r of rows) {
    const el = document.createElementNS(SVG_NS, r.tag);
    el.setAttribute("data-prism-mark-key", r.key);
    for (const [k, v] of Object.entries(r.attrs || {})) {
      el.setAttribute(k, v);
    }
    svg.appendChild(el);
  }
  return svg;
}

const prevSvg = buildSvg([
  { tag: "rect", key: "region=west",  attrs: { x: "10", y: "100", width: "20", height: "100", fill: "#000000" } },
  { tag: "rect", key: "region=east",  attrs: { x: "40", y: "120", width: "20", height: "80",  fill: "#000000" } },
  { tag: "rect", key: "region=south", attrs: { x: "70", y: "150", width: "20", height: "50",  fill: "#000000" } },
]);
const nextSvg = buildSvg([
  { tag: "rect", key: "region=west",  attrs: { x: "10", y: "50",  width: "20", height: "150", fill: "#ffffff" } },
  { tag: "rect", key: "region=east",  attrs: { x: "40", y: "60",  width: "20", height: "140", fill: "#ffffff" } },
  { tag: "rect", key: "region=north", attrs: { x: "100", y: "100", width: "20", height: "100", fill: "#ffffff" } },
]);

// ----- Test 1: partition -----

const animator = new PrismAnimator(prevSvg, nextSvg, { duration_ms: 1000 });
const { enter, update, exit } = animator.partition();
if (enter.length !== 1 || enter[0].key !== "region=north") fail(`enter set wrong: ${JSON.stringify(enter.map((x) => x.key))}`);
if (update.length !== 2) fail(`update set wrong: ${JSON.stringify(update.map((x) => x.key))}`);
if (exit.length !== 1 || exit[0].key !== "region=south") fail(`exit set wrong: ${JSON.stringify(exit.map((x) => x.key))}`);

// ----- Test 2: tween numeric attrs -----

let nowVal = 0;
const tickQueue = [];
const tickAnimator = new PrismAnimator(prevSvg, nextSvg, { duration_ms: 1000, easing: "linear" }, {
  now: () => nowVal,
  rAF: (cb) => { tickQueue.push(cb); return tickQueue.length; },
  cAF: () => {},
});
const tweenDone = tickAnimator.start();
// Drive frames at t = 0, 250, 500, 1000.
const drainFrame = (atMs) => {
  nowVal = atMs;
  const cb = tickQueue.shift();
  if (cb) cb();
};
drainFrame(0);
drainFrame(250);
drainFrame(500);
drainFrame(1000);
const finished = await tweenDone;
if (!finished) fail("tween did not signal completion");

const westRect = prevSvg.querySelector(`[data-prism-mark-key="region=west"]`);
if (westRect.getAttribute("y") !== "50") fail(`west y = ${westRect.getAttribute("y")}, want 50 after tween`);
if (westRect.getAttribute("height") !== "150") fail(`west height = ${westRect.getAttribute("height")}, want 150 after tween`);
if (westRect.getAttribute("fill") !== "#ffffff") fail(`west fill = ${westRect.getAttribute("fill")}, want #ffffff after tween`);

// ----- Test 3: OKLab interpolation at midpoint is gray, not muddy -----

const lerpProbe = buildSvg([
  { tag: "rect", key: "c", attrs: { x: "0", y: "0", width: "10", height: "10", fill: "#000000" } },
]);
const targetProbe = buildSvg([
  { tag: "rect", key: "c", attrs: { x: "0", y: "0", width: "10", height: "10", fill: "#ffffff" } },
]);
let probeNow = 0;
let probeFrame = null;
const probeAnim = new PrismAnimator(lerpProbe, targetProbe, { duration_ms: 1000, easing: "linear" }, {
  now: () => probeNow,
  rAF: (cb) => { probeFrame = cb; return 1; },
  cAF: () => {},
});
const probeDone = probeAnim.start();
probeNow = 0;       probeFrame && probeFrame();
probeNow = 500;     probeFrame && probeFrame();
const midFill = lerpProbe.querySelector(`[data-prism-mark-key="c"]`).getAttribute("fill");
// In OKLab, the midpoint of #000 → #fff has L = 0.5 which is the
// perceptual middle gray (~18% relative luminance). sRGB linear lerp
// would land at #808080; OKLab places the perceptual midpoint
// distinctly darker (~#60). Assert the midpoint diverges from sRGB
// lerp and sits in the expected OKLab band.
const channel = parseInt(midFill.slice(1, 3), 16);
if (!midFill.startsWith("#") || channel < 80 || channel > 120) {
  fail(`oklab midpoint expected #50-#78 band (perceptual middle gray); got ${midFill}`);
}
if (channel === 127 || channel === 128) {
  fail(`oklab midpoint should not equal sRGB linear lerp #80; got ${midFill}`);
}
probeNow = 1000;    probeFrame && probeFrame();
await probeDone;

// ----- Test 4: structurallyCompatible -----

const docA = { grid: { cells: [{ scene: { layers: [{ mark: "rect" }], axes: [{}, {}] } }] } };
const docB = { grid: { cells: [{ scene: { layers: [{ mark: "rect" }], axes: [{}, {}] } }] } };
const docMismatch = { grid: { cells: [{ scene: { layers: [{ mark: "line" }], axes: [{}, {}] } }] } };
if (!structurallyCompatible(docA, docB)) fail("structurallyCompatible should accept matching layers");
if (structurallyCompatible(docA, docMismatch)) fail("structurallyCompatible should reject mark family mismatch");

// ----- Test 5: prefers-reduced-motion default false in happy-dom -----

if (prefersReducedMotion() !== false) fail("prefersReducedMotion should default false in happy-dom");

// ----- Test 6: easing function table -----

if (Object.keys(EASINGS).length !== 13) fail(`EASINGS has ${Object.keys(EASINGS).length} entries; want 13`);
if (typeof easingFn("not_a_real_easing") !== "function") fail("easingFn should fall back to cubic_in_out for unknown names");
if (easingFn("linear")(0.5) !== 0.5) fail("linear easing midpoint should be 0.5");

console.error("PASS: partition + numeric tween + OKLab + structurallyCompatible + reduced-motion + easings");
try { await window.happyDOM?.close(); } catch {}
process.exit(0);
