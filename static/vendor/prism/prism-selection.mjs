// prism-selection.mjs — selection state plumbing + hit-test handlers.
//
// P12 stubbed the API surface; P13 wires it up:
//   - newSelectionState()              — empty per-selection state shape
//   - broadcast(target, name, detail)  — CustomEvent dispatch
//   - listen(target, name, handler)    — listener pair with dispose
//   - setSelection(handle, id, state)  — store + dispatch prism:select
//   - getSelection(handle, id)         — read state for one selection
//   - getAllSelections(handle)         — full map snapshot
//   - installHandlers(handle)          — wire DOM listeners (click +
//                                        mousedown/mousemove/mouseup)
//   - applySelection(handle, id, st)   — toggle prism-selected /
//                                        prism-deselected classes
//   - scaleInverse(axis, pixel)        — domain value at a pixel
//   - revalidateSelections(handle, newDoc) — D080 invalidation pass
//
// All exports stay synchronous; the only async surface is the user's
// handler (CustomEvent dispatch is sync).
//
// The "datum_row" + "data-prism-layer" attribute pair lands on every
// per-row mark via the Go renderer (P05) + JS renderer (T13.04)
// per D077; this module reads those attributes to build DatumRefs.

const SVG_NS = "http://www.w3.org/2000/svg";

/**
 * newSelectionState returns the empty selection-state object shape.
 * Mirrors scene.SelectionState (Go).
 */
export function newSelectionState() {
  return { points: [], range: null };
}

/**
 * broadcast dispatches a `prism:<name>` CustomEvent on the given
 * target. Always uses bubbles + composed so shadow-root events
 * surface on the host element + cross the shadow boundary.
 */
export function broadcast(target, name, detail) {
  if (!target || typeof target.dispatchEvent !== "function") return;
  const ev = new CustomEvent(name, {
    detail,
    bubbles: true,
    composed: true,
    cancelable: false,
  });
  target.dispatchEvent(ev);
}

/**
 * listen wraps addEventListener / removeEventListener. Returns the
 * dispose function for use in destructors.
 */
export function listen(target, name, handler, opts) {
  if (!target || typeof target.addEventListener !== "function") {
    return () => {};
  }
  target.addEventListener(name, handler, opts);
  return () => target.removeEventListener(name, handler, opts);
}

// ---------------------------------------------------------------- //
// Storage — per-SceneHandle Map<selectionID, state>, keyed via
// WeakMap so destroyed handles release their state automatically.
// ---------------------------------------------------------------- //

const _store = new WeakMap();

function _handleStore(handle) {
  let m = _store.get(handle);
  if (!m) {
    m = new Map();
    _store.set(handle, m);
  }
  return m;
}

/**
 * setSelection records the new selection state for the given handle
 * under the named selection ID, then broadcasts `prism:select` on
 * the handle's root target with detail `{id, state}`.
 */
export function setSelection(handle, selectionID, state) {
  const m = _handleStore(handle);
  m.set(selectionID, state);
  const target = handle && (handle._root || handle._svg);
  if (target) broadcast(target, "prism:select", { id: selectionID, state });
  // Apply client-reactive classes (D078). No-op when state lists no
  // marks (clearing selection). Best-effort: silently swallow DOM
  // hiccups so a bad selection ID can't crash the page.
  try { applySelection(handle, selectionID, state); } catch (e) { /* defensive */ }
}

/**
 * getSelection returns the current state for the named selection on
 * this handle, or null when none stored.
 */
export function getSelection(handle, selectionID) {
  const m = _store.get(handle);
  if (!m) return null;
  return m.get(selectionID) ?? null;
}

/**
 * getAllSelections returns a snapshot of every selection's state on
 * the given handle as a plain object keyed by selection ID. Returns
 * an empty object when no selections are stored.
 */
export function getAllSelections(handle) {
  const m = _store.get(handle);
  if (!m) return {};
  const out = {};
  for (const [k, v] of m.entries()) out[k] = v;
  return out;
}

/**
 * _resetForTests clears all stored selections for a handle. Test
 * escape hatch; production code should not call this.
 */
export function _resetForTests(handle) {
  if (handle) _store.delete(handle);
}

// ---------------------------------------------------------------- //
// Hit-test handlers — wired by prism.mjs after render
// ---------------------------------------------------------------- //

/**
 * installHandlers wires DOM listeners for every selection declared on
 * the handle's first scene cell. Idempotent: a second call removes
 * the prior listeners via the dispose closures returned by listen().
 *
 * @param {object} handle - SceneHandle from prism.render
 */
export function installHandlers(handle) {
  if (!handle || !handle._svg) return;
  // Tear down any prior install.
  if (Array.isArray(handle._disposers) && handle._disposers.length > 0) {
    for (const d of handle._disposers) {
      try { d(); } catch {}
    }
  }
  handle._disposers = [];
  const sels = Array.isArray(handle._selections) ? handle._selections : [];
  if (sels.length === 0) return;

  for (const sel of sels) {
    if (sel.kind === "point") {
      _installPointHandler(handle, sel);
    } else if (sel.kind === "interval") {
      _installIntervalHandler(handle, sel);
    }
  }
}

function _installPointHandler(handle, sel) {
  const dispose = listen(handle._svg, "click", (ev) => {
    const target = _hitTestDatum(ev.target);
    if (!target) return;
    const prev = getSelection(handle, sel.id);
    const newState = _togglePointSelection(prev, target);
    setSelection(handle, sel.id, newState);
  });
  handle._disposers.push(dispose);
}

function _togglePointSelection(prev, ref) {
  // Toggle semantics: if ref is already in prev.points, drop it;
  // otherwise add. Empty result → range:null,points:[] (clear).
  const have = prev && Array.isArray(prev.points) ? prev.points : [];
  const exists = have.some(p => p.layer_id === ref.layer_id && p.row_id === ref.row_id);
  let next;
  if (exists) {
    next = have.filter(p => !(p.layer_id === ref.layer_id && p.row_id === ref.row_id));
  } else {
    next = have.concat([ref]);
  }
  return { points: next, range: null };
}

/**
 * _hitTestDatum walks up from the click target until it finds an
 * element carrying both data-prism-datum-row and an ancestor with
 * data-prism-layer. Returns {layer_id, row_id} or null.
 */
function _hitTestDatum(target) {
  let node = target;
  let rowID = null;
  let layerID = null;
  while (node && node.nodeType === 1) {
    if (rowID === null && node.hasAttribute && node.hasAttribute("data-prism-datum-row")) {
      const raw = node.getAttribute("data-prism-datum-row");
      const n = Number(raw);
      if (Number.isFinite(n)) rowID = n;
    }
    if (layerID === null && node.hasAttribute && node.hasAttribute("data-prism-layer")) {
      layerID = node.getAttribute("data-prism-layer") || "";
    }
    if (rowID !== null && layerID !== null) break;
    node = node.parentNode;
  }
  if (rowID === null) return null;
  return { layer_id: layerID || "", row_id: rowID };
}

function _installIntervalHandler(handle, sel) {
  const svg = handle._svg;
  // Find the affected axis (per sel.encodings; default x). The first
  // matching axis from the first cell wins; multi-axis brushes are a
  // P14 follow-up.
  const cell = handle._doc?.grid?.cells?.[0];
  if (!cell) return;
  const plot = cell.scene.plot || { x: 0, y: 0, w: 0, h: 0 };
  const channel = (Array.isArray(sel.encodings) && sel.encodings[0]) || "x";
  const axis = (cell.scene.axes || []).find(a => a.channel === channel);
  if (!axis) return;

  let dragStart = null;
  let brushRect = null;

  const onDown = (ev) => {
    const rect = svg.getBoundingClientRect ? svg.getBoundingClientRect() : { left: 0, top: 0 };
    const x = (ev.clientX ?? 0) - rect.left;
    const y = (ev.clientY ?? 0) - rect.top;
    dragStart = { x, y };
    if (brushRect && brushRect.parentNode) brushRect.parentNode.removeChild(brushRect);
    brushRect = svg.ownerDocument.createElementNS(SVG_NS, "rect");
    brushRect.setAttribute("class", "prism-brush");
    brushRect.setAttribute("fill", "rgba(70,130,180,0.2)");
    brushRect.setAttribute("stroke", "rgba(70,130,180,0.5)");
    brushRect.setAttribute("pointer-events", "none");
    svg.appendChild(brushRect);
  };
  const onMove = (ev) => {
    if (!dragStart) return;
    const rect = svg.getBoundingClientRect ? svg.getBoundingClientRect() : { left: 0, top: 0 };
    const x = (ev.clientX ?? 0) - rect.left;
    const y = (ev.clientY ?? 0) - rect.top;
    if (channel === "x") {
      const minX = Math.min(dragStart.x, x);
      const w = Math.abs(x - dragStart.x);
      brushRect.setAttribute("x", String(minX));
      brushRect.setAttribute("y", String(plot.y));
      brushRect.setAttribute("width", String(w));
      brushRect.setAttribute("height", String(plot.h));
    } else {
      const minY = Math.min(dragStart.y, y);
      const h = Math.abs(y - dragStart.y);
      brushRect.setAttribute("x", String(plot.x));
      brushRect.setAttribute("y", String(minY));
      brushRect.setAttribute("width", String(plot.w));
      brushRect.setAttribute("height", String(h));
    }
  };
  const onUp = (ev) => {
    if (!dragStart) return;
    const rect = svg.getBoundingClientRect ? svg.getBoundingClientRect() : { left: 0, top: 0 };
    const x = (ev.clientX ?? 0) - rect.left;
    const y = (ev.clientY ?? 0) - rect.top;
    const startPx = channel === "x" ? dragStart.x : dragStart.y;
    const endPx = channel === "x" ? x : y;
    const p1 = scaleInverse(axis, Math.min(startPx, endPx));
    const p2 = scaleInverse(axis, Math.max(startPx, endPx));
    // For y axes pixel→domain is inverted; ensure min < max in value space.
    const lo = Math.min(p1, p2);
    const hi = Math.max(p1, p2);
    setSelection(handle, sel.id, {
      points: [],
      range: { channel, min: lo, max: hi },
    });
    dragStart = null;
    if (brushRect && brushRect.parentNode) {
      brushRect.parentNode.removeChild(brushRect);
      brushRect = null;
    }
  };
  handle._disposers.push(listen(svg, "mousedown", onDown));
  handle._disposers.push(listen(svg, "mousemove", onMove));
  handle._disposers.push(listen(svg, "mouseup", onUp));
}

/**
 * scaleInverse derives the data-space value at a pixel position for
 * one axis. Supports linear, time, band, point inverses. Log / pow
 * fall back to linear (rare for selections; full support in P14).
 */
export function scaleInverse(axis, pixel) {
  if (!axis || !axis.scale) return pixel;
  const sc = axis.scale;
  const r0 = sc.range?.[0] ?? 0;
  const r1 = sc.range?.[1] ?? 0;
  switch (sc.type) {
    case "linear":
    case "log":  // P13 stop-gap; linear interpolation in pixel space
    case "pow":
    case "sqrt": {
      const d0 = Number(sc.domain?.[0] ?? 0);
      const d1 = Number(sc.domain?.[1] ?? 0);
      if (r1 === r0) return d0;
      const t = (pixel - r0) / (r1 - r0);
      return d0 + t * (d1 - d0);
    }
    case "time": {
      // Domain entries can be ISO strings, ms epochs, or Date objects;
      // normalise to ms for interpolation, return ms.
      const d0 = _toMs(sc.domain?.[0]);
      const d1 = _toMs(sc.domain?.[1]);
      if (r1 === r0) return d0;
      const t = (pixel - r0) / (r1 - r0);
      return d0 + t * (d1 - d0);
    }
    case "band":
    case "point": {
      // Bucket the pixel into the nearest band index → return the
      // categorical domain entry. Range typically rises (r0 < r1).
      const dom = Array.isArray(sc.domain) ? sc.domain : [];
      if (dom.length === 0) return null;
      const span = r1 - r0;
      const step = span / dom.length;
      const idx = Math.max(0, Math.min(dom.length - 1, Math.floor((pixel - r0) / step)));
      return dom[idx];
    }
    default:
      return pixel;
  }
}

function _toMs(v) {
  if (v == null) return 0;
  if (typeof v === "number") return v;
  if (typeof v === "string") {
    const d = Date.parse(v);
    return Number.isFinite(d) ? d : 0;
  }
  if (v instanceof Date) return v.getTime();
  return 0;
}

// ---------------------------------------------------------------- //
// applySelection — client-reactive CSS class application (D078)
// ---------------------------------------------------------------- //

/**
 * applySelection walks every per-row mark in the affected layer and
 * toggles `prism-selected` / `prism-deselected` classes based on
 * whether the mark matches the current state. Empty state → strip
 * both classes (return to neutral).
 */
export function applySelection(handle, selectionID, state) {
  const svg = handle && handle._svg;
  if (!svg) return;
  // Locate the selection descriptor to honour its scope.
  const sel = (handle._selections || []).find(s => s.id === selectionID);
  if (!sel) return;

  // Enumerate marks across every layer (selections are scene-wide in v1).
  const layers = svg.querySelectorAll ? svg.querySelectorAll("g.prism-layer") : [];
  const hasState = state && (
    (Array.isArray(state.points) && state.points.length > 0) ||
    (state.range && (state.range.min !== undefined || state.range.max !== undefined))
  );

  for (const lg of layers) {
    const layerID = lg.getAttribute("data-prism-layer") || "";
    const marks = lg.querySelectorAll ? lg.querySelectorAll("[data-prism-datum-row]") : [];
    for (const mk of marks) {
      // Strip both classes when no state is active.
      _toggleClass(mk, "prism-selected", false);
      _toggleClass(mk, "prism-deselected", false);
      if (!hasState) continue;
      const rowID = Number(mk.getAttribute("data-prism-datum-row"));
      const selected = _markMatchesState(layerID, rowID, sel, state, handle);
      _toggleClass(mk, selected ? "prism-selected" : "prism-deselected", true);
    }
  }
}

function _markMatchesState(layerID, rowID, sel, state, handle) {
  if (sel.kind === "point") {
    const pts = Array.isArray(state.points) ? state.points : [];
    return pts.some(p => (p.layer_id === layerID || p.layer_id === "") && p.row_id === rowID);
  }
  if (sel.kind === "interval" && state.range) {
    // Interval matching requires the value-space lookup. In v1 we
    // approximate by treating the absence of a per-mark value map
    // as "match-all-in-layer"; richer match falls out of P14's
    // server-reactive mode which filters by query.
    // The lookup table lives on the handle when present (built at
    // installHandlers time).
    const lookup = handle && handle._rowValues && handle._rowValues[layerID];
    if (!lookup) return true;
    const v = lookup[rowID];
    if (v === undefined) return true;
    return v >= state.range.min && v <= state.range.max;
  }
  return false;
}

function _toggleClass(el, cls, force) {
  if (!el || !el.classList) {
    // Fallback for environments without DOMTokenList: hand-edit class attr.
    const cur = el.getAttribute ? (el.getAttribute("class") || "") : "";
    const parts = new Set(cur.split(/\s+/).filter(Boolean));
    if (force) parts.add(cls); else parts.delete(cls);
    if (el.setAttribute) el.setAttribute("class", Array.from(parts).join(" "));
    return;
  }
  el.classList.toggle(cls, !!force);
}

// ---------------------------------------------------------------- //
// revalidateSelections — D080 invalidation policy on SceneHandle.update
// ---------------------------------------------------------------- //

/**
 * revalidateSelections walks each stored selection and prunes any
 * entries that no longer make sense against newDoc:
 *
 *   - point: drop DatumRefs whose (layer_id, row_id) is missing in
 *     the new doc's per-layer mark inventory.
 *   - interval: clear the range when it falls entirely outside the
 *     new axis's domain.
 *
 * Fires prism:select with the post-prune state for each selection
 * touched so downstream listeners (URL serialiser, cross-chart bus)
 * see the cleanup.
 */
export function revalidateSelections(handle, newDoc) {
  const m = _store.get(handle);
  if (!m || m.size === 0) return;
  const cell = newDoc?.grid?.cells?.[0];
  if (!cell) return;

  // Build a per-layer set of (row_id) present in the new doc.
  const present = {};
  for (const layer of (cell.scene.layers || [])) {
    const set = new Set();
    for (const mk of (layer.marks || [])) {
      if (mk.datum && typeof mk.datum.row_id !== "undefined") set.add(Number(mk.datum.row_id));
    }
    present[layer.id || ""] = set;
  }

  const newSels = Array.isArray(cell.scene.selections) ? cell.scene.selections : [];

  for (const [id, state] of Array.from(m.entries())) {
    const descriptor = newSels.find(s => s.id === id);
    if (!descriptor) {
      // Selection no longer declared in new doc → drop entirely.
      m.delete(id);
      const target = handle._root || handle._svg;
      if (target) broadcast(target, "prism:select", { id, state: newSelectionState() });
      continue;
    }
    let changed = false;
    let nextPoints = Array.isArray(state.points) ? state.points : [];
    if (descriptor.kind === "point" && nextPoints.length > 0) {
      const before = nextPoints.length;
      nextPoints = nextPoints.filter(p => {
        const set = present[p.layer_id] || present[""];
        return set && set.has(Number(p.row_id));
      });
      if (nextPoints.length !== before) changed = true;
    }
    let nextRange = state.range || null;
    if (descriptor.kind === "interval" && nextRange) {
      const channel = nextRange.channel || "x";
      const axis = (cell.scene.axes || []).find(a => a.channel === channel);
      if (axis && axis.scale && axis.scale.domain) {
        const d0 = Number(axis.scale.domain[0]);
        const d1 = Number(axis.scale.domain[1]);
        const lo = Math.min(d0, d1);
        const hi = Math.max(d0, d1);
        if (nextRange.max < lo || nextRange.min > hi) {
          nextRange = null;
          changed = true;
        }
      }
    }
    if (changed) {
      const nextState = { points: nextPoints, range: nextRange };
      m.set(id, nextState);
      const target = handle._root || handle._svg;
      if (target) broadcast(target, "prism:select", { id, state: nextState });
    }
  }
}
