// prism.mjs — SceneDoc → SVG renderer for the browser.
//
// Public exports:
//   - render(sceneDoc, target) → SceneHandle
//   - validate(sceneDoc) → string[]   // empty = valid
//   - SceneHandle                      // handle returned by render
//
// This module mirrors render/svg/renderer.go function-for-function so
// the cross-impl harness (D076) can byte-diff the two outputs.
// Coordinate precision pins to 3 decimals via fmt() per D072.
//
// No D3 imports yet — the renderer only emits SVG primitives; the
// vendored d3-* modules in ./d3/ ride along for future client-side
// scale recomputation (post-v1).

import {
  getSelection as _getSelection,
  installHandlers as _installHandlers,
  getAllSelections as _getAllSelections,
  revalidateSelections as _revalidateSelections,
} from "./prism-selection.mjs";

const SVG_NS  = "http://www.w3.org/2000/svg";
const XHTML_NS = "http://www.w3.org/1999/xhtml";

// ---------------------------------------------------------------- //
// Public API
// ---------------------------------------------------------------- //

/**
 * render mounts an SVG into `target`, walking the SceneDoc top-down
 * exactly as render/svg/renderer.go does.
 *
 * @param {object} sceneDoc - SceneDoc JSON (Scene IR v1.0).
 * @param {HTMLElement|ShadowRoot|string} target - mount point.
 * @returns {SceneHandle}
 */
export function render(sceneDoc, target) {
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

  // Compute viewBox from the outer frame (mirror outerFrame in
  // renderer.go). 1×1 grids → first cell's Frame; multi-cell grids
  // → max bottom-right corner.
  const frame = _outerFrame(sceneDoc.grid);
  const W = frame.w || 800;
  const H = frame.h || 600;

  const svg = doc.createElementNS(SVG_NS, "svg");
  svg.setAttribute("xmlns", SVG_NS);
  svg.setAttribute("viewBox", `0 0 ${fmt(W)} ${fmt(H)}`);
  svg.setAttribute("width", fmt(W));
  svg.setAttribute("height", fmt(H));

  // Style block. Prefer theme.css verbatim (D075); fall back to
  // hand-built block when absent (parity with writeStyleBlock).
  _appendStyleBlock(svg, sceneDoc.theme || null, doc);

  // Walk cells row-major.
  const cells = (sceneDoc.grid && sceneDoc.grid.cells) || [];
  for (const cell of cells) {
    _renderScene(svg, cell.scene, doc);
  }

  // Shared axes (D051 parity).
  _renderSharedAxes(svg, sceneDoc.grid, doc);

  // Facet headers.
  _renderGridHeaders(svg, sceneDoc.grid, doc);

  root.appendChild(svg);
  const handle = new SceneHandle({ svg, root, sceneDoc });
  // Selection hit-test handlers (D077/D078). installHandlers is a
  // no-op when the first cell has no selections declared, so the cost
  // is one Map lookup per render in the no-selection case.
  try { _installHandlers(handle); } catch (e) { /* defensive */ }
  return handle;
}

/**
 * validate performs a quick shape check. Returns an array of error
 * messages (empty = valid). Throws TypeError on null / non-object.
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
 * SceneHandle is the live handle returned by render(). Mirrors the
 * API sketched in design/08-browser.md.
 */
export class SceneHandle {
  constructor({ svg, root, sceneDoc }) {
    this._svg = svg;
    this._root = root;
    this._doc = sceneDoc;
    this._listeners = new Map(); // eventName → Set<handler>
    // Selection descriptors come from the first cell. v1 keeps
    // selections per-cell (composite cross-cell sharing → P14).
    const firstCell = sceneDoc?.grid?.cells?.[0];
    this._selections = firstCell?.scene?.selections || [];
    // Per-handle dispose closures returned by installHandlers; reset
    // on every render so SceneHandle.update doesn't double-bind.
    this._disposers = [];
  }

  /** update re-renders with a new SceneDoc. Runs selection
   * invalidation (D080) before mounting the new DOM so the
   * post-render classList pass sees the cleaned-up state. */
  update(newSceneDoc) {
    const errs = validate(newSceneDoc);
    if (errs.length > 0) {
      throw new Error("SceneHandle.update: invalid SceneDoc: " + errs.join("; "));
    }
    try { _revalidateSelections(this, newSceneDoc); } catch (e) { /* defensive */ }
    if (this._svg && this._svg.parentNode) {
      this._svg.parentNode.removeChild(this._svg);
    }
    // Tear down listeners from the previous render before swapping
    // the SVG; otherwise dispose closures hold stale references.
    if (this._disposers && this._disposers.length > 0) {
      for (const d of this._disposers) {
        try { d(); } catch {}
      }
      this._disposers = [];
    }
    const replacement = render(newSceneDoc, this._root);
    this._svg = replacement._svg;
    this._doc = replacement._doc;
    this._selections = replacement._selections;
    this._disposers = replacement._disposers;
    return this;
  }

  /** on registers a handler for 'select', 'hover', 'click'. */
  on(eventName, handler) {
    if (!this._listeners.has(eventName)) {
      this._listeners.set(eventName, new Set());
    }
    this._listeners.get(eventName).add(handler);
    return this;
  }

  /** off removes a handler. */
  off(eventName, handler) {
    const set = this._listeners.get(eventName);
    if (set) set.delete(handler);
    return this;
  }

  /** setTheme is a v1 stub. Theme switching requires a server re-
   * render in v1 (D075); host page should re-fetch the SceneDoc and
   * call update(). The web component handles the round trip. */
  setTheme(themeName) {
    if (this._root && this._root.host && this._root.host.setAttribute) {
      this._root.host.setAttribute("theme", themeName);
    }
    return this;
  }

  /** getSelection returns the current selection state for the named
   * selection ID, or null when none stored. Wired in P13 per D077/D078.
   * Pass no args (or undefined) to get the full Map snapshot via
   * getAllSelections. */
  getSelection(selectionID) {
    if (selectionID === undefined) {
      return _getAllSelections(this);
    }
    return _getSelection(this, selectionID);
  }

  /** destroy removes the SVG + clears listeners + tears down
   *  hit-test handlers. */
  destroy() {
    if (this._disposers && this._disposers.length > 0) {
      for (const d of this._disposers) {
        try { d(); } catch (e) { /* swallow */ }
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

// ---------------------------------------------------------------- //
// fmt + escape helpers — parity with render/svg/writer.go
// ---------------------------------------------------------------- //

/** fmt rounds n to 3 decimals, trims trailing zeros + dot. Mirrors
 *  render.FormatFloat (D072). */
export function fmt(n) {
  if (!Number.isFinite(n)) return "0";
  if (Number.isInteger(n)) return String(n);
  const s = n.toFixed(3);
  // Trim trailing zeros + a dangling decimal point. Examples:
  //   3.142 -> 3.142
  //   3.140 -> 3.14
  //   3.000 -> 3
  return s.replace(/\.?0+$/, "");
}

/** escapeAttr mirrors writer.go escapeAttr — XML attribute escaping
 *  for &, <, >, ", '. Used by string-built attributes (path d=,
 *  href=) that don't go through setAttribute. */
export function escapeAttr(s) {
  if (s == null) return "";
  let out = "";
  for (const ch of String(s)) {
    switch (ch) {
      case "&": out += "&amp;"; break;
      case "<": out += "&lt;"; break;
      case ">": out += "&gt;"; break;
      case '"': out += "&quot;"; break;
      case "'": out += "&#39;"; break;
      default:  out += ch;
    }
  }
  return out;
}

// ---------------------------------------------------------------- //
// Internal helpers — DOM construction
// ---------------------------------------------------------------- //

function _resolveTarget(target) {
  if (target == null) {
    throw new TypeError("prism.render: target is null");
  }
  if (typeof target === "string") {
    const el = (globalThis.document || {}).querySelector?.(target);
    if (!el) throw new Error(`prism.render: selector ${target} matched nothing`);
    return el;
  }
  return target;
}

function _outerFrame(grid) {
  if (!grid || !Array.isArray(grid.cells) || grid.cells.length === 0) {
    return { x: 0, y: 0, w: 0, h: 0 };
  }
  if (grid.cells.length === 1) {
    return grid.cells[0].scene.frame || { x: 0, y: 0, w: 0, h: 0 };
  }
  let maxX = 0, maxY = 0;
  for (const c of grid.cells) {
    const f = c.scene.frame || { x: 0, y: 0, w: 0, h: 0 };
    const right = f.x + f.w;
    const bottom = f.y + f.h;
    if (right > maxX) maxX = right;
    if (bottom > maxY) maxY = bottom;
  }
  return { x: 0, y: 0, w: maxX, h: maxY };
}

function _appendStyleBlock(svg, theme, doc) {
  // Mirror writeStyleBlock in render/svg/style.go:
  // - When theme.css present, embed it verbatim (D075).
  // - Otherwise, build the default block from individual fields.
  // We use innerHTML so the resulting CSS text isn't double-escaped
  // (style content is CDATA-like in SVG; mirrors w.Raw in Go).
  const css = (theme && theme.css) ? theme.css : _defaultStyleBlock(theme);
  // css already includes the <style>...</style> wrapper when from
  // theme.css. innerHTML on the parent treats it as parsed markup.
  // To stay structurally identical to Go output we set svg.innerHTML
  // to the css fragment? No — we have to keep other children. Use
  // DOMParser-free path: create a <style> element ourselves when
  // css is a bare CSS string (default path) or when css starts with
  // "<style>" strip the wrapper + use textContent.
  let inner;
  if (css.startsWith("<style>") && css.endsWith("</style>")) {
    inner = css.slice("<style>".length, css.length - "</style>".length);
  } else {
    inner = css;
  }
  // Append D078 selection defaults. Theme authors override via the
  // documented CSS-variable contract; the JS port stamps the defaults
  // verbatim so unstyled charts get a sensible look out of the box.
  const selectionDefaults = `.prism-selected{opacity:var(--prism-selected-opacity,1);}.prism-deselected{opacity:var(--prism-deselected-opacity,0.3);}`;
  const styleEl = doc.createElementNS(SVG_NS, "style");
  styleEl.textContent = inner + selectionDefaults;
  svg.appendChild(styleEl);
}

function _defaultStyleBlock(theme) {
  // Mirrors writeStyleBlock fallback path in style.go. Used only
  // when theme.css is empty (back-compat with scene.Default()).
  const parts = [":root{"];
  if (theme) {
    if (theme.color_axis) parts.push(`--prism-color-axis:${_colorCSS(theme.color_axis)};`);
    if (theme.color_grid) parts.push(`--prism-color-grid:${_colorCSS(theme.color_grid)};`);
    if (theme.color_text) parts.push(`--prism-color-text:${_colorCSS(theme.color_text)};`);
    if (theme.font_sans)  parts.push(`--prism-font-sans:${theme.font_sans};`);
    if (theme.font_mono)  parts.push(`--prism-font-mono:${theme.font_mono};`);
  }
  parts.push("}");
  parts.push(".prism-axis-domain{stroke:var(--prism-color-axis);fill:none;}");
  parts.push(".prism-axis-tick{stroke:var(--prism-color-axis);}");
  parts.push(".prism-axis-label{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:11px;}");
  parts.push(".prism-axis-title{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:12px;font-weight:600;}");
  parts.push(".prism-grid-line{stroke:var(--prism-color-grid);}");
  parts.push(".prism-title{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:16px;font-weight:600;}");
  parts.push(".prism-legend-title{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:12px;font-weight:600;}");
  parts.push(".prism-legend-label{fill:var(--prism-color-text);font-family:var(--prism-font-sans);font-size:11px;}");
  parts.push(".prism-legend-swatch{stroke:none;}");
  return parts.join("");
}

function _colorCSS(c) {
  if (!c) return "transparent";
  if (c.a === 255 || c.a === undefined) {
    return `#${_hex2(c.r)}${_hex2(c.g)}${_hex2(c.b)}`;
  }
  return `rgba(${c.r},${c.g},${c.b},${c.a / 255})`;
}

function _hex2(n) {
  return (n & 0xff).toString(16).padStart(2, "0");
}

// ---------------------------------------------------------------- //
// Scene + layer walkers
// ---------------------------------------------------------------- //

function _renderScene(svg, scene, doc) {
  const g = doc.createElementNS(SVG_NS, "g");
  g.setAttribute("class", "prism-scene");
  if (scene.id) g.setAttribute("data-scene-id", scene.id);

  if (scene.title) {
    const t = doc.createElementNS(SVG_NS, "text");
    t.setAttribute("class", "prism-title");
    t.setAttribute("x", fmt(scene.title.x));
    t.setAttribute("y", fmt(scene.title.y));
    t.setAttribute("text-anchor", "middle");
    t.textContent = scene.title.content || "";
    g.appendChild(t);
  }

  if (Array.isArray(scene.axes) && scene.axes.length > 0) {
    const axesG = doc.createElementNS(SVG_NS, "g");
    axesG.setAttribute("class", "prism-axes");
    for (const a of scene.axes) {
      _renderAxis(axesG, a, scene.plot || { x: 0, y: 0, w: 0, h: 0 }, doc);
    }
    g.appendChild(axesG);
  }

  if (scene.defs) {
    _renderDefs(g, scene.defs, doc);
  }

  const plot = doc.createElementNS(SVG_NS, "g");
  plot.setAttribute("class", "prism-plot");
  const layers = Array.isArray(scene.layers) ? scene.layers : [];
  for (const layer of layers) {
    if (layer.hidden) continue;
    const lg = doc.createElementNS(SVG_NS, "g");
    lg.setAttribute("class", "prism-layer");
    if (layer.id) lg.setAttribute("data-layer-id", layer.id);
    // Hit-test scope per D077. Always emit (empty string when layer.id
    // is unset) so selector queries stay stable.
    lg.setAttribute("data-prism-layer", layer.id || "");
    const marks = Array.isArray(layer.marks) ? layer.marks : [];
    for (const m of marks) {
      _renderMark(lg, m, doc);
    }
    plot.appendChild(lg);
  }
  g.appendChild(plot);

  if (Array.isArray(scene.legends) && scene.legends.length > 0) {
    _renderLegends(g, scene.legends, doc);
  }

  svg.appendChild(g);
}

function _renderSharedAxes(svg, grid, doc) {
  const shared = grid && grid.shared;
  if (!shared || (!shared.x && !shared.y)) return;
  const cells = grid.cells || [];
  const plot = cells[0] ? (cells[0].scene.plot || { x: 0, y: 0, w: 0, h: 0 }) : { x: 0, y: 0, w: 0, h: 0 };
  const g = doc.createElementNS(SVG_NS, "g");
  g.setAttribute("class", "prism-axes");
  g.setAttribute("data-shared", "true");
  if (shared.x) _renderAxis(g, shared.x, plot, doc);
  if (shared.y) _renderAxis(g, shared.y, plot, doc);
  svg.appendChild(g);
}

function _renderGridHeaders(svg, grid, doc) {
  const headers = grid && grid.layout && grid.layout.headers;
  if (!headers) return;
  const top = headers.top || [];
  const left = headers.left || [];
  if (top.length === 0 && left.length === 0) return;
  const cells = grid.cells || [];
  if (cells.length === 0) return;

  const g = doc.createElementNS(SVG_NS, "g");
  g.setAttribute("class", "prism-grid-headers");

  if (top.length > 0) {
    const colCenters = new Map();
    for (const c of cells) {
      if (!colCenters.has(c.col)) {
        const f = c.scene.frame;
        colCenters.set(c.col, f.x + f.w / 2);
      }
    }
    top.forEach((label, ci) => {
      const cx = colCenters.get(ci);
      if (cx === undefined) return;
      const t = doc.createElementNS(SVG_NS, "text");
      t.setAttribute("class", "prism-facet-header prism-facet-header-top");
      t.setAttribute("x", fmt(cx));
      t.setAttribute("y", fmt(14));
      t.setAttribute("text-anchor", "middle");
      t.textContent = label;
      g.appendChild(t);
    });
  }
  if (left.length > 0) {
    const rowCenters = new Map();
    for (const c of cells) {
      if (!rowCenters.has(c.row)) {
        const f = c.scene.frame;
        rowCenters.set(c.row, f.y + f.h / 2);
      }
    }
    left.forEach((label, ri) => {
      const cy = rowCenters.get(ri);
      if (cy === undefined) return;
      const t = doc.createElementNS(SVG_NS, "text");
      t.setAttribute("class", "prism-facet-header prism-facet-header-left");
      t.setAttribute("x", fmt(6));
      t.setAttribute("y", fmt(cy));
      t.setAttribute("text-anchor", "start");
      t.textContent = label;
      g.appendChild(t);
    });
  }
  svg.appendChild(g);
}

// ---------------------------------------------------------------- //
// Axis renderer (mirrors render/svg/axes.go renderAxis)
// ---------------------------------------------------------------- //

function _renderAxis(parent, axis, plot, doc) {
  const g = doc.createElementNS(SVG_NS, "g");
  g.setAttribute("class", `prism-axis prism-axis-${axis.channel || ""}`);
  g.setAttribute("data-prism-axis-id", axis.id || "");

  // Grid lines first.
  const grid = Array.isArray(axis.grid) ? axis.grid : [];
  for (const ln of grid) {
    const el = doc.createElementNS(SVG_NS, "line");
    el.setAttribute("class", "prism-grid-line");
    el.setAttribute("x1", fmt(ln.x1));
    el.setAttribute("y1", fmt(ln.y1));
    el.setAttribute("x2", fmt(ln.x2));
    el.setAttribute("y2", fmt(ln.y2));
    g.appendChild(el);
  }

  // Domain line.
  if (axis.domain) {
    const el = doc.createElementNS(SVG_NS, "line");
    el.setAttribute("class", "prism-axis-domain");
    el.setAttribute("x1", fmt(axis.domain.x1));
    el.setAttribute("y1", fmt(axis.domain.y1));
    el.setAttribute("x2", fmt(axis.domain.x2));
    el.setAttribute("y2", fmt(axis.domain.y2));
    g.appendChild(el);
  }

  // Ticks + labels.
  const ticks = Array.isArray(axis.ticks) ? axis.ticks : [];
  const pos = axis.position;
  const plotBottom = (plot.y || 0) + (plot.h || 0);
  const plotRight = (plot.x || 0) + (plot.w || 0);
  for (const t of ticks) {
    const tlen = t.minor ? 3 : 5;
    if (pos === "bottom") {
      _tickMark(g, t.pixel, plotBottom, 0, tlen, true, doc);
      if (t.label && !t.label_hidden) {
        _tickLabel(g, t.label, t.pixel, plotBottom + 18, "middle", axis.label_angle || 0, doc);
      }
    } else if (pos === "top") {
      _tickMark(g, t.pixel, plot.y || 0, 0, -tlen, true, doc);
      if (t.label && !t.label_hidden) {
        _tickLabel(g, t.label, t.pixel, (plot.y || 0) - 8, "middle", axis.label_angle || 0, doc);
      }
    } else if (pos === "left") {
      _tickMark(g, plot.x || 0, t.pixel, -tlen, 0, false, doc);
      if (t.label && !t.label_hidden) {
        _tickLabel(g, t.label, (plot.x || 0) - 8, t.pixel + 4, "end", axis.label_angle || 0, doc);
      }
    } else if (pos === "right") {
      _tickMark(g, plotRight, t.pixel, tlen, 0, false, doc);
      if (t.label && !t.label_hidden) {
        _tickLabel(g, t.label, plotRight + 8, t.pixel + 4, "start", axis.label_angle || 0, doc);
      }
    }
  }

  // Title.
  if (axis.title) {
    const tt = doc.createElementNS(SVG_NS, "text");
    tt.setAttribute("class", "prism-axis-title");
    const cx = (plot.x || 0) + (plot.w || 0) / 2;
    const cy = (plot.y || 0) + (plot.h || 0) / 2;
    if (pos === "bottom") {
      tt.setAttribute("x", fmt(cx));
      tt.setAttribute("y", fmt(plotBottom + 34));
      tt.setAttribute("text-anchor", "middle");
    } else if (pos === "top") {
      tt.setAttribute("x", fmt(cx));
      tt.setAttribute("y", fmt((plot.y || 0) - 28));
      tt.setAttribute("text-anchor", "middle");
    } else if (pos === "left") {
      tt.setAttribute("x", fmt((plot.x || 0) - 30));
      tt.setAttribute("y", fmt(cy));
      tt.setAttribute("text-anchor", "middle");
      tt.setAttribute("transform", `rotate(-90 ${fmt((plot.x || 0) - 30)} ${fmt(cy)})`);
    } else if (pos === "right") {
      tt.setAttribute("x", fmt(plotRight + 30));
      tt.setAttribute("y", fmt(cy));
      tt.setAttribute("text-anchor", "middle");
      tt.setAttribute("transform", `rotate(90 ${fmt(plotRight + 30)} ${fmt(cy)})`);
    }
    tt.textContent = axis.title;
    g.appendChild(tt);
  }

  parent.appendChild(g);
}

function _tickMark(parent, px, py, dx, dy, horizontal, doc) {
  const el = doc.createElementNS(SVG_NS, "line");
  el.setAttribute("class", "prism-axis-tick");
  el.setAttribute("x1", fmt(px));
  el.setAttribute("y1", fmt(py));
  el.setAttribute("x2", fmt(px + dx));
  el.setAttribute("y2", fmt(py + dy));
  parent.appendChild(el);
  // Suppress unused-var warning in lint.
  void horizontal;
}

function _tickLabel(parent, label, x, y, anchor, angle, doc) {
  const el = doc.createElementNS(SVG_NS, "text");
  el.setAttribute("class", "prism-axis-label");
  el.setAttribute("x", fmt(x));
  el.setAttribute("y", fmt(y));
  el.setAttribute("text-anchor", anchor);
  if (angle) {
    el.setAttribute("transform", `rotate(${fmt(angle)} ${fmt(x)} ${fmt(y)})`);
  }
  el.textContent = label;
  parent.appendChild(el);
}

// ---------------------------------------------------------------- //
// Legends (basic — solid + symbol swatches; gradient skipped in v1)
// ---------------------------------------------------------------- //

// _renderLegends mirrors render/svg/legends.go renderLegends.
// Wraps every Scene.Legend in a single <g class="prism-legends">
// container, then emits one <g class="prism-legend prism-legend-{channel}">
// per legend with title + entries (solid + gradient + symbol).
function _renderLegends(parent, legends, doc) {
  if (!legends || legends.length === 0) return;
  const wrap = doc.createElementNS(SVG_NS, "g");
  wrap.setAttribute("class", "prism-legends");
  for (const lg of legends) {
    _renderLegend(wrap, lg, doc);
  }
  parent.appendChild(wrap);
}

function _renderLegend(parent, lg, doc) {
  const g = doc.createElementNS(SVG_NS, "g");
  g.setAttribute("class", `prism-legend prism-legend-${lg.channel || ""}`);
  g.setAttribute("data-prism-legend-id", lg.id || "");

  const frame = lg.frame || { x: 0, y: 0, w: 0, h: 0 };
  const titleH = 14;

  if (lg.title) {
    const t = doc.createElementNS(SVG_NS, "text");
    t.setAttribute("class", "prism-legend-title");
    t.setAttribute("x", fmt(frame.x + 4));
    t.setAttribute("y", fmt(frame.y + titleH));
    t.textContent = lg.title;
    g.appendChild(t);
  }
  const rowOffset = lg.title ? (titleH + 4) : 0;

  const entries = Array.isArray(lg.entries) ? lg.entries : [];
  entries.forEach((entry, i) => {
    const y = frame.y + rowOffset + i * 18 + 8;
    const swatch = entry.swatch || {};
    switch (swatch.type) {
      case "solid": {
        const sw = doc.createElementNS(SVG_NS, "rect");
        sw.setAttribute("class", "prism-legend-swatch");
        sw.setAttribute("x", fmt(frame.x + 4));
        sw.setAttribute("y", fmt(y));
        sw.setAttribute("width",  fmt(12));
        sw.setAttribute("height", fmt(12));
        if (swatch.color) sw.setAttribute("fill", _colorCSS(swatch.color));
        g.appendChild(sw);
        const tt = doc.createElementNS(SVG_NS, "text");
        tt.setAttribute("class", "prism-legend-label");
        tt.setAttribute("x", fmt(frame.x + 22));
        tt.setAttribute("y", fmt(y + 10));
        tt.textContent = entry.label || "";
        g.appendChild(tt);
        break;
      }
      case "gradient": {
        const sw = doc.createElementNS(SVG_NS, "rect");
        sw.setAttribute("class", "prism-legend-swatch");
        sw.setAttribute("x", fmt(frame.x + 4));
        sw.setAttribute("y", fmt(y));
        sw.setAttribute("width",  fmt(12));
        sw.setAttribute("height", fmt(frame.h - rowOffset - 16));
        if (swatch.gradient_id) sw.setAttribute("fill", `url(#${swatch.gradient_id})`);
        g.appendChild(sw);
        const tt = doc.createElementNS(SVG_NS, "text");
        tt.setAttribute("class", "prism-legend-label");
        tt.setAttribute("x", fmt(frame.x + 22));
        tt.setAttribute("y", fmt(y + 10));
        tt.textContent = entry.label || "";
        g.appendChild(tt);
        break;
      }
      case "symbol": {
        const sym = doc.createElementNS(SVG_NS, "circle");
        sym.setAttribute("class", "prism-legend-symbol");
        sym.setAttribute("cx", fmt(frame.x + 10));
        sym.setAttribute("cy", fmt(y + 6));
        sym.setAttribute("r",  fmt(5));
        if (swatch.color) sym.setAttribute("fill", _colorCSS(swatch.color));
        g.appendChild(sym);
        const tt = doc.createElementNS(SVG_NS, "text");
        tt.setAttribute("class", "prism-legend-label");
        tt.setAttribute("x", fmt(frame.x + 22));
        tt.setAttribute("y", fmt(y + 10));
        tt.textContent = entry.label || "";
        g.appendChild(tt);
        break;
      }
    }
  });

  parent.appendChild(g);
}

// ---------------------------------------------------------------- //
// Defs (gradients / patterns / clips) — pass-through for v1
// ---------------------------------------------------------------- //

// _renderDefs mirrors render/svg/legends.go renderDefs. Skipped
// when defs is empty (no gradients, no patterns, no clips) — the
// Go renderer's guard matches.
function _renderDefs(parent, defs, doc) {
  if (!defs) return;
  const grads = defs.gradients || {};
  const pats  = defs.patterns  || {};
  const clips = defs.clips     || {};
  if (Object.keys(grads).length === 0 && Object.keys(pats).length === 0 && Object.keys(clips).length === 0) {
    return;
  }
  const el = doc.createElementNS(SVG_NS, "defs");
  for (const [id, g] of Object.entries(grads)) {
    if (g.type === "linear") {
      const grad = doc.createElementNS(SVG_NS, "linearGradient");
      grad.setAttribute("id", id);
      grad.setAttribute("x1", fmt(g.x1 || 0));
      grad.setAttribute("y1", fmt(g.y1 || 0));
      grad.setAttribute("x2", fmt(g.x2 || 0));
      grad.setAttribute("y2", fmt(g.y2 || 0));
      for (const s of (g.stops || [])) {
        const stop = doc.createElementNS(SVG_NS, "stop");
        stop.setAttribute("offset", fmt(s.offset));
        stop.setAttribute("stop-color", _colorCSS(s.color));
        grad.appendChild(stop);
      }
      el.appendChild(grad);
    } else if (g.type === "radial") {
      const grad = doc.createElementNS(SVG_NS, "radialGradient");
      grad.setAttribute("id", id);
      grad.setAttribute("cx", fmt(g.x1 || 0));
      grad.setAttribute("cy", fmt(g.y1 || 0));
      grad.setAttribute("r",  fmt(g.x2 || 0));
      for (const s of (g.stops || [])) {
        const stop = doc.createElementNS(SVG_NS, "stop");
        stop.setAttribute("offset", fmt(s.offset));
        stop.setAttribute("stop-color", _colorCSS(s.color));
        grad.appendChild(stop);
      }
      el.appendChild(grad);
    }
  }
  for (const [id, c] of Object.entries(clips)) {
    const cp = doc.createElementNS(SVG_NS, "clipPath");
    cp.setAttribute("id", id);
    const rect = doc.createElementNS(SVG_NS, "rect");
    rect.setAttribute("x", fmt(c.x || 0));
    rect.setAttribute("y", fmt(c.y || 0));
    rect.setAttribute("width",  fmt(c.w || 0));
    rect.setAttribute("height", fmt(c.h || 0));
    cp.appendChild(rect);
    el.appendChild(cp);
  }
  parent.appendChild(el);
}

// ---------------------------------------------------------------- //
// Mark dispatch (T12.04)
// ---------------------------------------------------------------- //

function _renderMark(parent, mark, doc) {
  if (mark.rect)  return _renderRect(parent, mark, doc);
  if (mark.arc)   return _renderArc(parent, mark, doc);
  if (mark.line)  return _renderLine(parent, mark, doc);
  if (mark.area)  return _renderArea(parent, mark, doc);
  if (mark.point) return _renderPoint(parent, mark, doc);
  if (mark.rule)  return _renderRule(parent, mark, doc);
  if (mark.text)  return _renderTextMark(parent, mark, doc);
  if (mark.path)  return _renderPath(parent, mark, doc);
  if (mark.image) return _renderImage(parent, mark, doc);
  // Unknown geometry: emit comment node for debugging parity.
  const cmt = doc.createComment(` mark type ${mark.type || ""} not rendered `);
  parent.appendChild(cmt);
}

// _setDatum stamps the per-mark hit-test attribute (D077). Called by
// every mark renderer; no-op when mark.datum is absent (composite
// helper marks that don't map back to a row).
function _setDatum(el, mark) {
  if (!mark || !mark.datum) return;
  if (mark.datum.row_id !== undefined && mark.datum.row_id !== null) {
    el.setAttribute("data-prism-datum-row", String(mark.datum.row_id));
  }
}

function _styleAttrs(el, style) {
  if (!style) return;
  if (style.fill)         el.setAttribute("fill",   _colorCSS(style.fill));
  if (style.stroke)       el.setAttribute("stroke", _colorCSS(style.stroke));
  if (style.stroke_width) el.setAttribute("stroke-width", fmt(style.stroke_width));
  if (style.opacity && style.opacity > 0 && style.opacity < 1) {
    el.setAttribute("opacity", fmt(style.opacity));
  }
}

function _maybeTooltip(parent, mark, doc) {
  if (!mark.tooltip || !Array.isArray(mark.tooltip.lines) || mark.tooltip.lines.length === 0) {
    return false;
  }
  const parts = mark.tooltip.lines.map(ln => ln.label || "");
  const title = doc.createElementNS(SVG_NS, "title");
  title.textContent = parts.join("\n");
  parent.appendChild(title);
  return true;
}

function _renderRect(parent, m, doc) {
  const g = m.rect;
  const el = doc.createElementNS(SVG_NS, "rect");
  el.setAttribute("class", "prism-mark-bar");
  if (m.id) el.setAttribute("data-prism-id", m.id);
  el.setAttribute("x", fmt(g.x));
  el.setAttribute("y", fmt(g.y));
  el.setAttribute("width",  fmt(g.w));
  el.setAttribute("height", fmt(g.h));
  if (g.corner_r) el.setAttribute("rx", fmt(g.corner_r));
  _styleAttrs(el, m.style);
  _setDatum(el, m);
  _maybeTooltip(el, m, doc);
  parent.appendChild(el);
}

function _renderLine(parent, m, doc) {
  const g = m.line;
  const pts = Array.isArray(g.points) ? g.points : [];
  if (pts.length === 0) return;
  const el = doc.createElementNS(SVG_NS, "polyline");
  el.setAttribute("class", "prism-mark-line");
  if (m.id) el.setAttribute("data-prism-id", m.id);
  const ptsStr = pts.map(p => `${fmt(p[0])},${fmt(p[1])}`).join(" ");
  el.setAttribute("points", ptsStr);
  el.setAttribute("fill", "none");
  _styleAttrs(el, m.style);
  _setDatum(el, m);
  _maybeTooltip(el, m, doc);
  parent.appendChild(el);
}

function _renderArea(parent, m, doc) {
  const g = m.area;
  const upper = Array.isArray(g.upper) ? g.upper : [];
  if (upper.length === 0) return;
  const el = doc.createElementNS(SVG_NS, "path");
  el.setAttribute("class", "prism-mark-area");
  if (m.id) el.setAttribute("data-prism-id", m.id);
  let d = `M${fmt(upper[0][0])},${fmt(upper[0][1])}`;
  for (let i = 1; i < upper.length; i++) {
    d += ` L${fmt(upper[i][0])},${fmt(upper[i][1])}`;
  }
  if (Array.isArray(g.lower) && g.lower.length > 0) {
    for (let i = g.lower.length - 1; i >= 0; i--) {
      d += ` L${fmt(g.lower[i][0])},${fmt(g.lower[i][1])}`;
    }
  } else {
    // Baseline fallback — mirror Go's max-y close path.
    const lastX = upper[upper.length - 1][0];
    const firstX = upper[0][0];
    let maxY = upper[0][1];
    for (const p of upper) { if (p[1] > maxY) maxY = p[1]; }
    d += ` L${fmt(lastX)},${fmt(maxY)} L${fmt(firstX)},${fmt(maxY)}`;
  }
  d += " Z";
  el.setAttribute("d", d);
  _styleAttrs(el, m.style);
  _setDatum(el, m);
  _maybeTooltip(el, m, doc);
  parent.appendChild(el);
}

function _renderPoint(parent, m, doc) {
  const g = m.point;
  const el = doc.createElementNS(SVG_NS, "circle");
  el.setAttribute("class", "prism-mark-point");
  if (m.id) el.setAttribute("data-prism-id", m.id);
  el.setAttribute("cx", fmt(g.cx));
  el.setAttribute("cy", fmt(g.cy));
  el.setAttribute("r",  fmt(g.r));
  _styleAttrs(el, m.style);
  _setDatum(el, m);
  _maybeTooltip(el, m, doc);
  parent.appendChild(el);
}

function _renderRule(parent, m, doc) {
  const g = m.rule;
  const el = doc.createElementNS(SVG_NS, "line");
  el.setAttribute("class", "prism-mark-rule");
  if (m.id) el.setAttribute("data-prism-id", m.id);
  el.setAttribute("x1", fmt(g.x1));
  el.setAttribute("y1", fmt(g.y1));
  el.setAttribute("x2", fmt(g.x2));
  el.setAttribute("y2", fmt(g.y2));
  _styleAttrs(el, m.style);
  _setDatum(el, m);
  _maybeTooltip(el, m, doc);
  parent.appendChild(el);
}

function _renderTextMark(parent, m, doc) {
  const g = m.text;
  const el = doc.createElementNS(SVG_NS, "text");
  el.setAttribute("class", "prism-mark-text");
  if (m.id) el.setAttribute("data-prism-id", m.id);
  el.setAttribute("x", fmt(g.x));
  el.setAttribute("y", fmt(g.y));
  const anchor = g.anchor === "start" ? "start" : g.anchor === "end" ? "end" : "middle";
  el.setAttribute("text-anchor", anchor);
  if (g.font_size) el.setAttribute("font-size", fmt(g.font_size));
  if (g.angle) el.setAttribute("transform", `rotate(${fmt(g.angle)} ${fmt(g.x)} ${fmt(g.y)})`);
  _styleAttrs(el, m.style);
  el.textContent = g.content || "";
  _maybeTooltip(el, m, doc);
  parent.appendChild(el);
}

function _renderArc(parent, m, doc) {
  const g = m.arc;
  const el = doc.createElementNS(SVG_NS, "path");
  el.setAttribute("class", "prism-mark-arc");
  if (m.id) el.setAttribute("data-prism-id", m.id);
  el.setAttribute("d", _arcPath(g));
  _styleAttrs(el, m.style);
  _setDatum(el, m);
  _maybeTooltip(el, m, doc);
  parent.appendChild(el);
}

function _arcPath(g) {
  // Mirrors render/svg/marks.go arcPath. Sweep flag = 1 (clockwise
  // in pixel space). Large-arc flag = 1 when angle delta > π.
  const sweepCW  = "1";
  const sweepCCW = "0";
  const largeArc = (g.end_angle - g.start_angle) > Math.PI ? "1" : "0";
  const cosS = Math.cos(g.start_angle);
  const sinS = Math.sin(g.start_angle);
  const cosE = Math.cos(g.end_angle);
  const sinE = Math.sin(g.end_angle);
  const ax = g.cx + g.outer_r * cosS;
  const ay = g.cy + g.outer_r * sinS;
  const bx = g.cx + g.outer_r * cosE;
  const by = g.cy + g.outer_r * sinE;
  if (!g.inner_r || g.inner_r <= 0) {
    return `M${fmt(g.cx)},${fmt(g.cy)}` +
           ` L${fmt(ax)},${fmt(ay)}` +
           ` A${fmt(g.outer_r)},${fmt(g.outer_r)} 0 ${largeArc} ${sweepCW} ${fmt(bx)},${fmt(by)}` +
           ` Z`;
  }
  const cx2 = g.cx + g.inner_r * cosE;
  const cy2 = g.cy + g.inner_r * sinE;
  const dx2 = g.cx + g.inner_r * cosS;
  const dy2 = g.cy + g.inner_r * sinS;
  return `M${fmt(ax)},${fmt(ay)}` +
         ` A${fmt(g.outer_r)},${fmt(g.outer_r)} 0 ${largeArc} ${sweepCW} ${fmt(bx)},${fmt(by)}` +
         ` L${fmt(cx2)},${fmt(cy2)}` +
         ` A${fmt(g.inner_r)},${fmt(g.inner_r)} 0 ${largeArc} ${sweepCCW} ${fmt(dx2)},${fmt(dy2)}` +
         ` Z`;
}

function _renderPath(parent, m, doc) {
  const g = m.path;
  const el = doc.createElementNS(SVG_NS, "path");
  el.setAttribute("class", "prism-mark-path");
  if (m.id) el.setAttribute("data-prism-id", m.id);
  el.setAttribute("d", g.d || "");
  _styleAttrs(el, m.style);
  _setDatum(el, m);
  _maybeTooltip(el, m, doc);
  parent.appendChild(el);
}

function _renderImage(parent, m, doc) {
  const g = m.image;
  const el = doc.createElementNS(SVG_NS, "image");
  el.setAttribute("class", "prism-mark-image");
  if (m.id) el.setAttribute("data-prism-id", m.id);
  el.setAttribute("x", fmt(g.x));
  el.setAttribute("y", fmt(g.y));
  el.setAttribute("width", fmt(g.w));
  el.setAttribute("height", fmt(g.h));
  // SVG2 uses href; SVG1 used xlink:href. Match Go (which emits href).
  el.setAttribute("href", g.href || "");
  _styleAttrs(el, m.style);
  _setDatum(el, m);
  _maybeTooltip(el, m, doc);
  parent.appendChild(el);
  void XHTML_NS;
}
