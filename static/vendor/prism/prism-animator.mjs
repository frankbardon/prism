// prism-animator.mjs — client-side tween between successive Prism
// scenes. The animator does not render — it expects two already-
// rendered SVGs (the live one in the DOM and a freshly produced
// replacement) and walks the marks by their `data-prism-mark-key`
// attribute, interpolating numeric attrs in place each rAF tick.
//
// Public surface:
//   class PrismAnimator
//     constructor(prevSvg, nextSvg, animation, opts?)
//     start({ onDone } = {})
//     cancel()
//     partition() → { enter, update, exit }   // exposed for tests
//
// Easing functions and structuralCompatible() helper are exported so
// the web component can do a quick fallback check without
// instantiating the animator.
//
// SVG/PDF renderers ignore animation hints — this module is the only
// consumer of `scene.animation` and `mark.key`.

// ---------------------------------------------------------------- //
// Easing
// ---------------------------------------------------------------- //

export const EASINGS = {
  linear:        (t) => t,
  cubic_in:      (t) => t * t * t,
  cubic_out:     (t) => 1 - Math.pow(1 - t, 3),
  cubic_in_out:  (t) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2),
  quad_in:       (t) => t * t,
  quad_out:      (t) => 1 - (1 - t) * (1 - t),
  quad_in_out:   (t) => (t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2),
  sine_in:       (t) => 1 - Math.cos((t * Math.PI) / 2),
  sine_out:      (t) => Math.sin((t * Math.PI) / 2),
  sine_in_out:   (t) => -(Math.cos(Math.PI * t) - 1) / 2,
  expo_in:       (t) => (t === 0 ? 0 : Math.pow(2, 10 * t - 10)),
  expo_out:      (t) => (t === 1 ? 1 : 1 - Math.pow(2, -10 * t)),
  expo_in_out:   (t) => {
    if (t === 0) return 0;
    if (t === 1) return 1;
    return t < 0.5
      ? Math.pow(2, 20 * t - 10) / 2
      : (2 - Math.pow(2, -20 * t + 10)) / 2;
  },
};

/** Resolve an easing name to a function. Unknown → cubic_in_out. */
export function easingFn(name) {
  return EASINGS[name] || EASINGS.cubic_in_out;
}

// ---------------------------------------------------------------- //
// Numeric attribute tween table — per SVG element name
// ---------------------------------------------------------------- //

// Numeric attrs we know how to tween, keyed by SVG element tag.
const NUMERIC_ATTRS = {
  rect:     ["x", "y", "width", "height", "rx", "ry", "opacity", "fill-opacity", "stroke-opacity"],
  circle:   ["cx", "cy", "r", "opacity", "fill-opacity", "stroke-opacity"],
  line:     ["x1", "y1", "x2", "y2", "stroke-width", "opacity"],
  polyline: ["stroke-width", "opacity"],
  ellipse:  ["cx", "cy", "rx", "ry", "opacity"],
  text:     ["x", "y", "font-size", "opacity"],
  image:    ["x", "y", "width", "height", "opacity"],
  path:     ["opacity", "fill-opacity", "stroke-opacity"],
};

// Color attrs that route through OKLab interpolation.
const COLOR_ATTRS = ["fill", "stroke"];

// ---------------------------------------------------------------- //
// Public helpers
// ---------------------------------------------------------------- //

/**
 * structurallyCompatible compares two SceneDocs cheaply — same layer
 * count, same mark-family per layer, same axis count per scene. Used
 * by the host element to short-circuit to an instant swap when the
 * animator can't do anything sensible.
 */
export function structurallyCompatible(prev, next) {
  if (!prev || !next) return false;
  const prevCells = prev?.grid?.cells || [];
  const nextCells = next?.grid?.cells || [];
  if (prevCells.length !== nextCells.length) return false;
  for (let i = 0; i < prevCells.length; i++) {
    const a = prevCells[i].scene;
    const b = nextCells[i].scene;
    if (!a || !b) return false;
    const al = a.layers || [];
    const bl = b.layers || [];
    if (al.length !== bl.length) return false;
    for (let j = 0; j < al.length; j++) {
      if ((al[j].mark || "") !== (bl[j].mark || "")) return false;
    }
    const aa = a.axes || [];
    const ba = b.axes || [];
    if (aa.length !== ba.length) return false;
  }
  return true;
}

/**
 * prefersReducedMotion returns true when the OS / browser asks for
 * reduced motion. Works in both real browsers and happy-dom; falls
 * back to false outside any media-query environment.
 */
export function prefersReducedMotion() {
  if (typeof globalThis.matchMedia !== "function") return false;
  try {
    return globalThis.matchMedia("(prefers-reduced-motion: reduce)").matches === true;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------- //
// PrismAnimator
// ---------------------------------------------------------------- //

const KEY_SELECTOR = "[data-prism-mark-key]";

export class PrismAnimator {
  /**
   * @param {SVGElement} prevSvg  the live SVG that will receive
   *   interpolated attr writes each frame.
   * @param {SVGElement} nextSvg  the freshly-rendered SVG whose final
   *   attr values are the targets. Not mounted; only read.
   * @param {object} animation  `scene.animation` block.
   * @param {object} [opts]
   * @param {Function} [opts.rAF]    custom rAF (test override).
   * @param {Function} [opts.cAF]    custom cancelAnimationFrame.
   * @param {Function} [opts.now]    custom clock (test override).
   * @param {boolean}  [opts.applyExitOpacity]  fade exit marks (default true).
   */
  constructor(prevSvg, nextSvg, animation, opts = {}) {
    this._prev = prevSvg;
    this._next = nextSvg;
    this._anim = _normaliseAnimation(animation);
    this._rAF = opts.rAF || _defaultRAF();
    this._cAF = opts.cAF || _defaultCAF();
    this._now = opts.now || _defaultNow();
    this._applyExit = opts.applyExitOpacity !== false;
    this._token = 0;
    this._cancelled = false;
    this._startTime = 0;
    this._tweens = null;
    this._done = null;
  }

  /**
   * partition splits marks into enter / update / exit sets keyed by
   * `data-prism-mark-key`. Exposed for tests and for callers that
   * want to react to specific transitions.
   */
  partition() {
    const prevMarks = _indexByKey(this._prev);
    const nextMarks = _indexByKey(this._next);
    const enter = [];
    const update = [];
    const exit  = [];
    for (const [key, nextEl] of nextMarks) {
      if (prevMarks.has(key)) {
        update.push({ key, prev: prevMarks.get(key), next: nextEl });
      } else {
        enter.push({ key, next: nextEl });
      }
    }
    for (const [key, prevEl] of prevMarks) {
      if (!nextMarks.has(key)) {
        exit.push({ key, prev: prevEl });
      }
    }
    return { enter, update, exit };
  }

  /**
   * start kicks off the rAF loop. Returns a Promise that resolves
   * once the tween completes (or is cancelled). `onDone` runs in
   * the same tick the tween ends.
   */
  start({ onDone } = {}) {
    if (this._cancelled) return Promise.resolve(false);

    // Pre-compute the per-mark tween program so the rAF loop is hot.
    const { update, exit } = this.partition();
    this._tweens = [];

    for (const u of update) {
      this._tweens.push(..._buildAttrTweens(u.prev, u.next));
    }
    if (this._applyExit && this._anim.exit !== "none") {
      for (const e of exit) {
        this._tweens.push({
          el: e.prev,
          attr: "opacity",
          kind: "number",
          from: _readNumeric(e.prev, "opacity", 1),
          to: 0,
        });
      }
    }

    this._startTime = this._now();
    const total = this._anim.durationMs;
    if (total <= 0) {
      // Snap immediately.
      this._applyAll(1);
      if (onDone) onDone(false);
      return Promise.resolve(true);
    }

    return new Promise((resolve) => {
      const ease = easingFn(this._anim.easing);
      const stagger = this._anim.staggerMs;
      const tweenCount = this._tweens.length;
      const tick = () => {
        if (this._cancelled) {
          resolve(false);
          return;
        }
        const elapsed = this._now() - this._startTime;
        let allDone = true;
        for (let i = 0; i < this._tweens.length; i++) {
          const t = this._tweens[i];
          const startAt = stagger > 0 ? (i / Math.max(tweenCount, 1)) * stagger : 0;
          const raw = (elapsed - startAt) / total;
          if (raw < 0) {
            _applyTween(t, 0);
            allDone = false;
            continue;
          }
          if (raw >= 1) {
            _applyTween(t, 1);
            continue;
          }
          _applyTween(t, ease(raw));
          allDone = false;
        }
        if (allDone) {
          if (onDone) onDone(true);
          resolve(true);
          return;
        }
        this._token = this._rAF(tick);
      };
      this._token = this._rAF(tick);
    });
  }

  cancel() {
    this._cancelled = true;
    if (this._token && this._cAF) this._cAF(this._token);
    this._token = 0;
    if (this._tweens) this._applyAll(1);
  }

  _applyAll(t) {
    if (!this._tweens) return;
    for (const tw of this._tweens) _applyTween(tw, t);
  }
}

// ---------------------------------------------------------------- //
// Internals
// ---------------------------------------------------------------- //

function _normaliseAnimation(a) {
  const out = {
    durationMs: 400,
    easing: "cubic_in_out",
    staggerMs: 0,
    enter: "fade",
    exit: "fade",
  };
  if (a && typeof a === "object") {
    if (Number.isFinite(a.duration_ms)) out.durationMs = Math.max(0, Math.min(5000, a.duration_ms | 0));
    if (typeof a.easing === "string" && a.easing) out.easing = a.easing;
    if (Number.isFinite(a.stagger_ms)) out.staggerMs = Math.max(0, Math.min(1000, a.stagger_ms | 0));
    if (a.enter === "none" || a.enter === "fade") out.enter = a.enter;
    if (a.exit === "none" || a.exit === "fade") out.exit = a.exit;
  }
  return out;
}

function _indexByKey(svg) {
  const out = new Map();
  if (!svg || typeof svg.querySelectorAll !== "function") return out;
  const nodes = svg.querySelectorAll(KEY_SELECTOR);
  for (const el of nodes) {
    const key = el.getAttribute("data-prism-mark-key");
    if (!key || out.has(key)) continue;
    out.set(key, el);
  }
  return out;
}

function _readNumeric(el, attr, fallback) {
  const raw = el.getAttribute(attr);
  if (raw == null || raw === "") return fallback;
  const n = parseFloat(raw);
  return Number.isFinite(n) ? n : fallback;
}

function _isHexColor(s) {
  return typeof s === "string" && s.length > 0 && s[0] === "#";
}

function _buildAttrTweens(prevEl, nextEl) {
  const tag = (prevEl.tagName || "").toLowerCase();
  const out = [];

  const nums = NUMERIC_ATTRS[tag] || ["opacity"];
  for (const attr of nums) {
    if (!_hasAttr(nextEl, attr) && !_hasAttr(prevEl, attr)) continue;
    const from = _readNumeric(prevEl, attr, 0);
    const to   = _readNumeric(nextEl, attr, from);
    if (from === to) continue;
    out.push({ el: prevEl, attr, kind: "number", from, to });
  }

  for (const attr of COLOR_ATTRS) {
    const fromRaw = prevEl.getAttribute(attr);
    const toRaw   = nextEl.getAttribute(attr);
    if (!_isHexColor(fromRaw) || !_isHexColor(toRaw) || fromRaw === toRaw) continue;
    out.push({ el: prevEl, attr, kind: "color", from: fromRaw, to: toRaw });
  }

  return out;
}

function _hasAttr(el, name) {
  return el && typeof el.hasAttribute === "function" && el.hasAttribute(name);
}

function _applyTween(t, progress) {
  if (t.kind === "number") {
    const v = t.from + (t.to - t.from) * progress;
    t.el.setAttribute(t.attr, _format(v));
    return;
  }
  if (t.kind === "color") {
    // Lazy-import oklab to keep this module Node-resolvable even when
    // oklab is mocked away in unit tests.
    const lerp = _oklabLerpHex();
    if (lerp) t.el.setAttribute(t.attr, lerp(t.from, t.to, progress));
    return;
  }
}

let _oklabLerp = null;
function _oklabLerpHex() {
  return _oklabLerp;
}

/**
 * registerOklab is the lazy hook used by prism.mjs to inject the
 * OKLab lerp without forcing this module to import a path that may
 * be mocked under happy-dom. Tests call it directly; production code
 * sees a static import side-effect.
 */
export function registerOklab(lerpHex) {
  _oklabLerp = lerpHex;
}

function _format(n) {
  if (!Number.isFinite(n)) return "0";
  // Match render.FormatFloat: 3 decimals, trim trailing zeros + dot.
  let s = n.toFixed(3);
  if (s.indexOf(".") >= 0) s = s.replace(/0+$/, "").replace(/\.$/, "");
  return s;
}

function _defaultRAF() {
  if (typeof globalThis.requestAnimationFrame === "function") {
    return (cb) => globalThis.requestAnimationFrame(cb);
  }
  return (cb) => setTimeout(() => cb(_defaultNow()()), 16);
}

function _defaultCAF() {
  if (typeof globalThis.cancelAnimationFrame === "function") {
    return (id) => globalThis.cancelAnimationFrame(id);
  }
  return (id) => clearTimeout(id);
}

function _defaultNow() {
  if (typeof performance !== "undefined" && typeof performance.now === "function") {
    return () => performance.now();
  }
  return () => Date.now();
}

// ---------------------------------------------------------------- //
// Side-effect: register the production OKLab lerp.
// ---------------------------------------------------------------- //

import { lerpHex as _lerpHex } from "./oklab.mjs";
registerOklab(_lerpHex);
