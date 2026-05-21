// prism.mjs — thin WASM bootstrapper for the Prism browser surface.
//
// P17 collapsed the JS-port pipeline into the Go pipeline: the
// compile/encode/render stages run in `prism.wasm`, and this module
// is a 60-line shim that loads the WASM module, marshals JSON across
// the syscall/js bridge, and mounts the Go-rendered SVG into the DOM.
//
// Public exports (unchanged from P12 so prism-element.mjs and host
// pages keep working):
//   - render(sceneDoc, target) → SceneHandle
//   - validate(sceneDoc) → string[]    // empty = valid
//   - executeSpec(spec, datasets?, opts?) → Promise<sceneDoc>
//   - SceneHandle
//   - fmt(n) → string  (3-decimal precision helper for rare host
//     callers that want to align labels with the rendered SVG)
//
// JS-side scale/axis/tick/palette/format duplication (~1000 LOC of
// reimplemented Go) was deleted in P17 — that surface lives only in
// the WASM binary now. The vendored D3 modules under ./d3/ were
// removed alongside it.

import {
  getSelection as _getSelection,
  installHandlers as _installHandlers,
  getAllSelections as _getAllSelections,
  revalidateSelections as _revalidateSelections,
} from "./prism-selection.mjs";
import {
  PrismAnimator,
  structurallyCompatible,
  prefersReducedMotion,
} from "./prism-animator.mjs";

// ---------------------------------------------------------------- //
// Lazy WASM bootstrap — the WASM module loads on first call to a
// pipeline-touching function (render / executeSpec). Utility
// exports (serialiseStateMap, fmt, validate, SceneHandle) work
// without WASM so the cross-impl selection harnesses can import
// prism-element.mjs without a wasm_exec.js + prism.wasm payload.
// ---------------------------------------------------------------- //

let _wasmReadyPromise = null;

/**
 * ensureWasmReady kicks off `prism.wasm` instantiation (idempotent;
 * concurrent callers share one Promise). Resolves once
 * `globalThis.prism.render` is callable.
 */
export function ensureWasmReady() {
  if (_wasmReadyPromise) return _wasmReadyPromise;
  _wasmReadyPromise = (async () => {
    if (globalThis.prism && typeof globalThis.prism.render === "function") {
      return;
    }
    if (typeof globalThis.Go === "undefined") {
      throw new Error("prism.mjs: wasm_exec.js must load before calling prism.render / executeSpec (`globalThis.Go` is undefined). Add `<script src=\"wasm_exec.js\"></script>` before the module import.");
    }
    const go = new globalThis.Go();
    const url = new URL("./prism.wasm", import.meta.url);
    let instantiated;
    if (typeof WebAssembly.instantiateStreaming === "function") {
      instantiated = WebAssembly.instantiateStreaming(fetch(url), go.importObject);
    } else {
      const buf = await fetch(url).then(r => r.arrayBuffer());
      instantiated = WebAssembly.instantiate(buf, go.importObject);
    }
    const { instance } = await instantiated;
    go.run(instance); // fire-and-forget; prism.* funcs registered in main()
    for (let i = 0; i < 50; i++) {
      if (globalThis.prism && typeof globalThis.prism.render === "function") return;
      await new Promise(r => setTimeout(r, 0));
    }
    throw new Error("prism.mjs: prism.wasm loaded but `globalThis.prism` was never populated");
  })();
  return _wasmReadyPromise;
}

const SVG_NS = "http://www.w3.org/2000/svg";

// ---------------------------------------------------------------- //
// Public API
// ---------------------------------------------------------------- //

/**
 * render mounts the Go-rendered SVG for `sceneDoc` into `target`.
 *
 * Returns a Promise<SceneHandle>; render became async in P17 when
 * the renderer moved to WASM. Callers that previously held the
 * synchronous return must `await render(...)`.
 *
 * @param {object} sceneDoc - SceneDoc JSON (Scene IR v1.0).
 * @param {HTMLElement|ShadowRoot|string} target - mount point.
 * @returns {Promise<SceneHandle>}
 */
export async function render(sceneDoc, target) {
  if (!sceneDoc || typeof sceneDoc !== "object") {
    throw new TypeError("prism.render: sceneDoc must be a non-null object");
  }
  const errs = validate(sceneDoc);
  if (errs.length > 0) {
    throw new Error("prism.render: invalid SceneDoc: " + errs.join("; "));
  }

  const root = _resolveTarget(target);
  const doc = root.ownerDocument || globalThis.document;
  if (!doc) {
    throw new Error("prism.render: target has no ownerDocument");
  }

  await ensureWasmReady();

  // Call WASM bridge. Returns SVG string or an error envelope JSON.
  const out = globalThis.prism.render(JSON.stringify(sceneDoc));
  _throwIfErrorEnvelope(out, "prism.render");

  // Parse the SVG string into a live DOM node. We use a throwaway
  // wrapper element's innerHTML rather than DOMParser because
  // happy-dom's DOMParser doesn't implement "image/svg+xml" (it
  // returns an empty document). innerHTML parsing works in real
  // browsers and in happy-dom, and preserves the data-prism-*
  // attributes the selection layer hit-tests against (D077/D078).
  const wrapper = doc.createElement("div");
  wrapper.innerHTML = out;
  const svg = wrapper.querySelector("svg");
  if (!svg) {
    throw new Error("prism.render: WASM returned malformed SVG (no <svg> after innerHTML parse)");
  }
  root.appendChild(svg);

  const handle = new SceneHandle({ svg, root, sceneDoc });
  try { _installHandlers(handle); } catch (e) { /* defensive */ }
  return handle;
}

/**
 * validate performs a quick JS-side shape check (version field,
 * grid.cells array). Deep validation runs on the WASM side at
 * compile time; this exists for downstream callers that want a
 * cheap pre-flight without paying the WASM round-trip.
 */
export function validate(sceneDoc) {
  if (sceneDoc == null || typeof sceneDoc !== "object") {
    throw new TypeError("prism.validate: sceneDoc must be an object");
  }
  const errs = [];
  if (sceneDoc.version !== "1.0") {
    errs.push(`version: expected "1.0", got ${JSON.stringify(sceneDoc.version)}`);
  }
  if (!sceneDoc.grid || typeof sceneDoc.grid !== "object") {
    errs.push("grid: missing or not an object");
    return errs;
  }
  if (!Array.isArray(sceneDoc.grid.cells)) {
    errs.push("grid.cells: missing or not an array");
    return errs;
  }
  sceneDoc.grid.cells.forEach((cell, i) => {
    if (!cell.scene || typeof cell.scene !== "object") {
      errs.push(`grid.cells[${i}].scene: missing or not an object`);
      return;
    }
    if (!Array.isArray(cell.scene.layers)) {
      errs.push(`grid.cells[${i}].scene.layers: missing or not an array`);
    }
  });
  return errs;
}

/**
 * executeSpec runs the full pipeline (validate → plan → execute →
 * encode) against a Prism spec, returning the resulting SceneDoc.
 * Async wrapper around the synchronous WASM bridge.
 *
 * @param {object} spec - Prism spec JSON (already parsed).
 * @param {object} [datasets] - alias → URL map.
 * @param {object} [opts] - {width, height, theme}.
 * @returns {Promise<object>} resolved SceneDoc.
 */
export async function executeSpec(spec, datasets, opts) {
  await ensureWasmReady();
  const specJSON = JSON.stringify(spec);
  const dsJSON = datasets ? JSON.stringify(datasets) : "";
  const optsJSON = opts ? JSON.stringify(opts) : "";
  const out = globalThis.prism.execute(specJSON, dsJSON, optsJSON);
  _throwIfErrorEnvelope(out, "prism.executeSpec");
  return JSON.parse(out);
}

/**
 * SceneHandle is the live handle returned by render(). Behaviour
 * preserved from the P12 implementation; the only thing that
 * changed is _how_ the SVG was produced (WASM, not JS port).
 */
export class SceneHandle {
  constructor({ svg, root, sceneDoc }) {
    this._svg = svg;
    this._root = root;
    this._doc = sceneDoc;
    this._listeners = new Map();
    const firstCell = sceneDoc?.grid?.cells?.[0];
    this._selections = firstCell?.scene?.selections || [];
    this._disposers = [];
  }

  async update(newSceneDoc, opts = {}) {
    const errs = validate(newSceneDoc);
    if (errs.length > 0) {
      throw new Error("SceneHandle.update: invalid SceneDoc: " + errs.join("; "));
    }
    try { _revalidateSelections(this, newSceneDoc); } catch (e) { /* defensive */ }

    const animBlock = _sceneAnimation(newSceneDoc);
    const canAnimate = opts.animate !== false
      && animBlock
      && this._svg
      && structurallyCompatible(this._doc, newSceneDoc)
      && !prefersReducedMotion();

    if (!canAnimate) {
      return this._swapInstant(newSceneDoc);
    }

    const oldSvg = this._svg;
    const replacement = await render(newSceneDoc, this._root);
    // Hide the freshly mounted SVG until the tween completes so the
    // user only sees the live tween on the old SVG; the new SVG
    // reveals at t=1.
    if (replacement._svg && replacement._svg.style) {
      replacement._svg.style.visibility = "hidden";
    }
    const animator = new PrismAnimator(oldSvg, replacement._svg, animBlock, opts.animatorOpts || {});
    try {
      await animator.start();
    } finally {
      if (replacement._svg && replacement._svg.style) {
        replacement._svg.style.visibility = "";
      }
      if (oldSvg && oldSvg.parentNode) {
        oldSvg.parentNode.removeChild(oldSvg);
      }
      if (this._disposers && this._disposers.length > 0) {
        for (const d of this._disposers) {
          try { d(); } catch {}
        }
      }
    }
    this._svg = replacement._svg;
    this._doc = replacement._doc;
    this._selections = replacement._selections;
    this._disposers = replacement._disposers;
    return this;
  }

  async _swapInstant(newSceneDoc) {
    if (this._svg && this._svg.parentNode) {
      this._svg.parentNode.removeChild(this._svg);
    }
    if (this._disposers && this._disposers.length > 0) {
      for (const d of this._disposers) {
        try { d(); } catch {}
      }
      this._disposers = [];
    }
    const replacement = await render(newSceneDoc, this._root);
    this._svg = replacement._svg;
    this._doc = replacement._doc;
    this._selections = replacement._selections;
    this._disposers = replacement._disposers;
    return this;
  }

  on(eventName, handler) {
    if (!this._listeners.has(eventName)) {
      this._listeners.set(eventName, new Set());
    }
    this._listeners.get(eventName).add(handler);
    return this;
  }

  off(eventName, handler) {
    const set = this._listeners.get(eventName);
    if (set) set.delete(handler);
    return this;
  }

  setTheme(themeName) {
    if (this._root && this._root.host && this._root.host.setAttribute) {
      this._root.host.setAttribute("theme", themeName);
    }
    return this;
  }

  getSelection(selectionID) {
    if (selectionID === undefined) {
      return _getAllSelections(this);
    }
    return _getSelection(this, selectionID);
  }

  destroy() {
    if (this._disposers && this._disposers.length > 0) {
      for (const d of this._disposers) {
        try { d(); } catch (e) {}
      }
      this._disposers = [];
    }
    if (this._svg && this._svg.parentNode) {
      this._svg.parentNode.removeChild(this._svg);
    }
    this._svg = null;
    this._listeners.clear();
    return this;
  }
}

/**
 * fmt rounds n to 3 decimals, trims trailing zeros + dot. Mirrors
 * render.FormatFloat (D072). Kept on the public surface because a
 * few host pages aligning custom overlays to mark coordinates were
 * documented against this helper in P12.
 */
export function fmt(n) {
  if (!Number.isFinite(n)) return "0";
  let s = n.toFixed(3);
  if (s.indexOf(".") >= 0) {
    s = s.replace(/0+$/, "").replace(/\.$/, "");
  }
  return s;
}

// ---------------------------------------------------------------- //
// Helpers
// ---------------------------------------------------------------- //

function _resolveTarget(target) {
  if (typeof target === "string") {
    const found = document.querySelector(target);
    if (!found) throw new Error(`prism.render: no element matches selector ${target}`);
    return found;
  }
  if (target && typeof target.appendChild === "function") return target;
  throw new TypeError("prism.render: target must be an Element, ShadowRoot, or CSS selector string");
}

// WASM bridge returns either a raw string payload (SVG, JSON of a
// successful scene/plan) or an `{ok:false, ...}` envelope. The
// envelope is a JSON string starting with `{"ok":false`; sniff for
// that prefix to avoid parsing every SVG body just to check.
function _throwIfErrorEnvelope(payload, where) {
  if (typeof payload !== "string") return;
  if (!payload.startsWith(`{"ok":false`)) return;
  let parsed;
  try { parsed = JSON.parse(payload); } catch { return; }
  if (parsed && parsed.ok === false) {
    const err = parsed.error || parsed;
    const code = err.code || err.Code || "PRISM_WASM_001";
    const msg  = err.message || err.Message || "WASM bridge error";
    const fixups = err.fixups || err.Fixups || [];
    const composed = new Error(`${where}: ${code}: ${msg}`);
    composed.prismCode = code;
    composed.prismFixups = fixups;
    throw composed;
  }
}

// Keep SVG_NS reachable as an export so a few existing call sites in
// prism-selection.mjs that walked the legacy node tree continue to
// resolve the constant.
export { SVG_NS };

// Re-export the animator surface so host pages can drive a tween on
// a bare SVG without going through SceneHandle.
export { PrismAnimator, structurallyCompatible, prefersReducedMotion };

/**
 * _sceneAnimation returns the animation block from the first cell's
 * scene, or null when no block is set. Used by SceneHandle.update to
 * decide whether to animate the swap.
 */
function _sceneAnimation(doc) {
  const a = doc?.grid?.cells?.[0]?.scene?.animation;
  if (!a || typeof a !== "object") return null;
  return a;
}
