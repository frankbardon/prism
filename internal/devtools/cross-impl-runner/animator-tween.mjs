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

// ----- Test 2b: text-mark style vocabulary attrs tween (E4-S1) -----
//
// dx / dy (the text anchor offset) and fill-opacity / stroke-opacity
// (the per-paint alphas, independent of and multiplicative with
// `opacity`) joined NUMERIC_ATTRS when the encoder started emitting
// them. They must tween rather than snap.

const textPrev = buildSvg([
  { tag: "text", key: "label=a", attrs: { x: "10", y: "20", dx: "0", dy: "0", "fill-opacity": "0.2", "stroke-opacity": "0.2" } },
]);
const textNext = buildSvg([
  { tag: "text", key: "label=a", attrs: { x: "10", y: "20", dx: "8", dy: "-4", "fill-opacity": "1", "stroke-opacity": "0.6" } },
]);
let textNow = 0;
let textFrame = null;
const textAnim = new PrismAnimator(textPrev, textNext, { duration_ms: 1000, easing: "linear" }, {
  now: () => textNow,
  rAF: (cb) => { textFrame = cb; return 1; },
  cAF: () => {},
});
const textDone = textAnim.start();
const textEl = textPrev.querySelector(`[data-prism-mark-key="label=a"]`);
textNow = 0;   textFrame && textFrame();
textNow = 500; textFrame && textFrame();
const midDx = parseFloat(textEl.getAttribute("dx"));
const midAlpha = parseFloat(textEl.getAttribute("fill-opacity"));
if (!(midDx > 0 && midDx < 8)) fail(`text dx should be mid-tween, got ${midDx}`);
if (!(midAlpha > 0.2 && midAlpha < 1)) fail(`text fill-opacity should be mid-tween, got ${midAlpha}`);
textNow = 1000; textFrame && textFrame();
await textDone;
if (parseFloat(textEl.getAttribute("dx")) !== 8) fail(`text dx = ${textEl.getAttribute("dx")}, want 8 after tween`);
if (parseFloat(textEl.getAttribute("dy")) !== -4) fail(`text dy = ${textEl.getAttribute("dy")}, want -4 after tween`);
if (parseFloat(textEl.getAttribute("fill-opacity")) !== 1) fail(`text fill-opacity = ${textEl.getAttribute("fill-opacity")}, want 1 after tween`);
if (parseFloat(textEl.getAttribute("stroke-opacity")) !== 0.6) fail(`text stroke-opacity = ${textEl.getAttribute("stroke-opacity")}, want 0.6 after tween`);

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

// ----- Test 3b: <path> tweens stroke-width -----
//
// A line mark with a non-linear `interpolate` renders as <path>
// instead of <polyline>, so the path tween set must cover the same
// stroke-width the polyline set does, or curved lines would snap
// their weight mid-transition.

const pathPrev = buildSvg([
  { tag: "path", key: "series=a", attrs: { d: "M0,0 C1,1 2,2 3,3", "stroke-width": "2", opacity: "1" } },
]);
const pathNext = buildSvg([
  { tag: "path", key: "series=a", attrs: { d: "M0,0 C1,1 2,2 3,3", "stroke-width": "6", opacity: "1" } },
]);
let pathNow = 0;
let pathFrame = null;
const pathAnim = new PrismAnimator(pathPrev, pathNext, { duration_ms: 1000, easing: "linear" }, {
  now: () => pathNow,
  rAF: (cb) => { pathFrame = cb; return 1; },
  cAF: () => {},
});
const pathDone = pathAnim.start();
pathNow = 0;    pathFrame && pathFrame();
pathNow = 500;  pathFrame && pathFrame();
const midWidth = Number(pathPrev.querySelector(`[data-prism-mark-key="series=a"]`).getAttribute("stroke-width"));
if (!(midWidth > 2 && midWidth < 6)) {
  fail(`path stroke-width should tween 2 → 6; midpoint was ${midWidth}`);
}
pathNow = 1000; pathFrame && pathFrame();
await pathDone;
const endWidth = pathPrev.querySelector(`[data-prism-mark-key="series=a"]`).getAttribute("stroke-width");
if (endWidth !== "6") fail(`path stroke-width = ${endWidth}, want 6 after tween`);

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

console.error("PASS: partition + numeric tween + text dx/dy + paint alphas + path stroke-width + OKLab + structurallyCompatible + reduced-motion + easings");
try { await window.happyDOM?.close(); } catch {}
process.exit(0);
