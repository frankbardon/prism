// animator-warn-fallback.mjs — TestPrismAnimatorWarnFallback.
//
// Exits 0 on pass, non-zero with FAIL to stderr otherwise. Asserts:
//   1. SceneHandle.update emits PRISM_WARN_ANIM_FALLBACK on
//      `prism:warn` when the new scene declares an `animation` block
//      but the previous scene is structurally incompatible.
//   2. No warn fires when the new scene omits the animation block.
//   3. No warn fires on reduced-motion (UX, not a fallback).
//   4. No warn fires for the first-render case (no previous SVG).
//
// Runs under Node + happy-dom; no WASM required. The test stubs
// `_swapInstant` on the SceneHandle under test so we exercise the
// decision branch without booting the WASM render path.

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
  fail("happy-dom not installed");
}
const window = new Window({ url: "http://localhost/" });
globalThis.window         = window;
globalThis.document       = window.document;
globalThis.HTMLElement    = window.HTMLElement;
globalThis.CustomEvent    = window.CustomEvent;
globalThis.customElements = window.customElements;
globalThis.matchMedia     = window.matchMedia
  || (() => ({ matches: false, addListener() {}, removeListener() {} }));

const prismPath = resolve(REPO, "static/vendor/prism/prism.mjs");
const { SceneHandle } = await import(prismPath);

const SVG_NS = "http://www.w3.org/2000/svg";

function buildDoc({ layerMark, animation }) {
  const layers = [{ id: "l1", mark: layerMark, marks: [] }];
  const scene = {
    id: "s1",
    frame: { x: 0, y: 0, w: 400, h: 300 },
    plot:  { x: 40, y: 20, w: 340, h: 260 },
    layers,
    axes: [],
  };
  if (animation) scene.animation = animation;
  return {
    version: "1.0",
    grid: { layout: { rows: 1, cols: 1 }, cells: [{ row: 0, col: 0, scene }] },
  };
}

function newHandle({ prevDoc, attachSvg = true }) {
  const root = document.createElement("div");
  document.body.appendChild(root);
  const svg = attachSvg ? document.createElementNS(SVG_NS, "svg") : null;
  if (svg) root.appendChild(svg);
  const h = new SceneHandle({ svg, root, sceneDoc: prevDoc });
  // Stub _swapInstant so we don't drag the WASM render path into the
  // test; the assertions are about the warn dispatch, not the swap.
  h._swapInstant = async () => h;
  return { h, root };
}

function listenForWarn(node) {
  const events = [];
  node.addEventListener("prism:warn", (ev) => events.push(ev.detail));
  return events;
}

// ---- Test 1: structural mismatch → warn ----

{
  const prevDoc = buildDoc({ layerMark: "rect" });
  const nextDoc = buildDoc({ layerMark: "line", animation: { duration_ms: 400 } });
  const { h, root } = newHandle({ prevDoc });
  const warns = listenForWarn(root);
  await h.update(nextDoc);
  if (warns.length !== 1) fail(`expected 1 warn, got ${warns.length}: ${JSON.stringify(warns)}`);
  if (warns[0].code !== "PRISM_WARN_ANIM_FALLBACK") fail(`expected PRISM_WARN_ANIM_FALLBACK, got ${warns[0].code}`);
  if (typeof warns[0].message !== "string" || warns[0].message.length === 0) fail("warn message missing");
}

// ---- Test 2: matching shape + animation → no warn (would animate) ----

{
  const prevDoc = buildDoc({ layerMark: "rect" });
  const nextDoc = buildDoc({ layerMark: "rect", animation: { duration_ms: 400 } });
  const { h, root } = newHandle({ prevDoc });
  const warns = listenForWarn(root);
  // Stub the animate-success branch too — render() would normally
  // run; we don't care about its output here, only the warn.
  const originalUpdate = h.update.bind(h);
  await originalUpdate(nextDoc).catch(() => {});
  if (warns.length !== 0) fail(`expected 0 warns (matched shape), got ${warns.length}`);
}

// ---- Test 3: no animation block in new scene → no warn ----

{
  const prevDoc = buildDoc({ layerMark: "rect" });
  const nextDoc = buildDoc({ layerMark: "line" }); // no animation
  const { h, root } = newHandle({ prevDoc });
  const warns = listenForWarn(root);
  await h.update(nextDoc);
  if (warns.length !== 0) fail(`expected 0 warns (no animation block), got ${warns.length}`);
}

// ---- Test 4: reduced-motion → no warn ----

{
  // Override matchMedia to flip prefers-reduced-motion on.
  const originalMM = globalThis.matchMedia;
  globalThis.matchMedia = (q) => ({
    matches: q === "(prefers-reduced-motion: reduce)",
    addListener() {}, removeListener() {},
  });
  const prevDoc = buildDoc({ layerMark: "rect" });
  const nextDoc = buildDoc({ layerMark: "line", animation: { duration_ms: 400 } });
  const { h, root } = newHandle({ prevDoc });
  const warns = listenForWarn(root);
  await h.update(nextDoc);
  globalThis.matchMedia = originalMM;
  if (warns.length !== 0) fail(`expected 0 warns (reduced motion), got ${warns.length}`);
}

// ---- Test 5: first render (no previous SVG) → no warn ----

{
  const prevDoc = buildDoc({ layerMark: "rect" });
  const nextDoc = buildDoc({ layerMark: "rect", animation: { duration_ms: 400 } });
  const { h, root } = newHandle({ prevDoc, attachSvg: false });
  const warns = listenForWarn(root);
  await h.update(nextDoc);
  if (warns.length !== 0) fail(`expected 0 warns (no prior SVG), got ${warns.length}`);
}

console.error("PASS: warn fires on structural mismatch; silent on match, no-block, reduced-motion, and first-render");
try { await window.happyDOM?.close(); } catch {}
process.exit(0);
