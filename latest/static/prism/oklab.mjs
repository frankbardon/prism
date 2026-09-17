// oklab.mjs — sRGB ↔ OKLab color interpolation for the Prism
// animator. OKLab is perceptually uniform, so interpolating in this
// space produces visibly smoother transitions on saturated hues than
// sRGB lerp. Matrices from Björn Ottosson's reference implementation
// (https://bottosson.github.io/posts/oklab/).
//
// Public API:
//   parseHex(hex) → {r, g, b, a}      // sRGB 0..1, alpha 0..1
//   formatHex({r, g, b, a}) → "#rrggbb" or "#rrggbbaa"
//   lerpHex(a, b, t) → "#rrggbb"      // interpolate two hex strings
//   lerpRgb(a, b, t) → {r, g, b, a}   // interpolate parsed values
//
// All functions are pure; no globals, no module state.

const HEX3 = /^#([0-9a-fA-F]{3})$/;
const HEX4 = /^#([0-9a-fA-F]{4})$/;
const HEX6 = /^#([0-9a-fA-F]{6})$/;
const HEX8 = /^#([0-9a-fA-F]{8})$/;

/**
 * parseHex accepts #rgb, #rgba, #rrggbb, #rrggbbaa.
 * Returns {r, g, b, a} with each channel in 0..1.
 * Throws on unsupported strings.
 */
export function parseHex(hex) {
  if (typeof hex !== "string") {
    throw new TypeError(`parseHex: expected string, got ${typeof hex}`);
  }
  let m;
  if ((m = hex.match(HEX6))) {
    return _split6(m[1], 1);
  }
  if ((m = hex.match(HEX8))) {
    const a = parseInt(m[1].slice(6, 8), 16) / 255;
    return _split6(m[1].slice(0, 6), a);
  }
  if ((m = hex.match(HEX3))) {
    return _split6(_expand3(m[1]), 1);
  }
  if ((m = hex.match(HEX4))) {
    const expanded = _expand3(m[1].slice(0, 3));
    const a = parseInt(m[1].slice(3, 4).repeat(2), 16) / 255;
    return _split6(expanded, a);
  }
  throw new Error(`parseHex: unsupported color "${hex}"`);
}

function _split6(hex6, a) {
  return {
    r: parseInt(hex6.slice(0, 2), 16) / 255,
    g: parseInt(hex6.slice(2, 4), 16) / 255,
    b: parseInt(hex6.slice(4, 6), 16) / 255,
    a,
  };
}

function _expand3(hex3) {
  return hex3.split("").map((c) => c + c).join("");
}

/**
 * formatHex turns {r, g, b, a} back into a hex string. Emits the
 * 8-digit form (with alpha) when a < 1, the 6-digit form otherwise.
 */
export function formatHex({ r, g, b, a = 1 }) {
  const r8 = _byte(r);
  const g8 = _byte(g);
  const b8 = _byte(b);
  const head = `#${_hex2(r8)}${_hex2(g8)}${_hex2(b8)}`;
  if (a >= 1) return head;
  return head + _hex2(_byte(a));
}

function _byte(x) {
  const n = Math.round(x * 255);
  if (n < 0) return 0;
  if (n > 255) return 255;
  return n;
}

function _hex2(n) {
  return n.toString(16).padStart(2, "0");
}

// ---------------------------------------------------------------- //
// sRGB ↔ linear ↔ OKLab
// ---------------------------------------------------------------- //

function _srgbToLinear(c) {
  return c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
}

function _linearToSrgb(c) {
  return c <= 0.0031308 ? c * 12.92 : 1.055 * Math.pow(c, 1 / 2.4) - 0.055;
}

/** Convert {r, g, b, a} (sRGB 0..1) → {L, a, b, alpha} (OKLab). */
export function toOklab({ r, g, b, a = 1 }) {
  const lr = _srgbToLinear(r);
  const lg = _srgbToLinear(g);
  const lb = _srgbToLinear(b);
  const l = 0.4122214708 * lr + 0.5363325363 * lg + 0.0514459929 * lb;
  const m = 0.2119034982 * lr + 0.6806995451 * lg + 0.1073969566 * lb;
  const s = 0.0883024619 * lr + 0.2817188376 * lg + 0.6299787005 * lb;
  const l_ = Math.cbrt(l);
  const m_ = Math.cbrt(m);
  const s_ = Math.cbrt(s);
  return {
    L: 0.2104542553 * l_ + 0.7936177850 * m_ - 0.0040720468 * s_,
    a: 1.9779984951 * l_ - 2.4285922050 * m_ + 0.4505937099 * s_,
    b: 0.0259040371 * l_ + 0.7827717662 * m_ - 0.8086757660 * s_,
    alpha: a,
  };
}

/** Convert {L, a, b, alpha} (OKLab) → {r, g, b, a} (sRGB 0..1). */
export function fromOklab({ L, a, b, alpha = 1 }) {
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b;
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b;
  const s_ = L - 0.0894841775 * a - 1.2914855480 * b;
  const lr = l_ * l_ * l_;
  const lg = m_ * m_ * m_;
  const lb = s_ * s_ * s_;
  const r = +4.0767416621 * lr - 3.3077115913 * lg + 0.2309699292 * lb;
  const g = -1.2684380046 * lr + 2.6097574011 * lg - 0.3413193965 * lb;
  const b2 = -0.0041960863 * lr - 0.7034186147 * lg + 1.7076147010 * lb;
  return {
    r: _clamp01(_linearToSrgb(r)),
    g: _clamp01(_linearToSrgb(g)),
    b: _clamp01(_linearToSrgb(b2)),
    a: alpha,
  };
}

function _clamp01(x) {
  if (x < 0) return 0;
  if (x > 1) return 1;
  return x;
}

// ---------------------------------------------------------------- //
// Interpolation
// ---------------------------------------------------------------- //

/**
 * lerpRgb interpolates between two sRGB colors in OKLab space.
 * t is clamped to [0, 1]. Returns {r, g, b, a} sRGB 0..1.
 */
export function lerpRgb(a, b, t) {
  const tt = t < 0 ? 0 : t > 1 ? 1 : t;
  const lab1 = toOklab(a);
  const lab2 = toOklab(b);
  return fromOklab({
    L: lab1.L + (lab2.L - lab1.L) * tt,
    a: lab1.a + (lab2.a - lab1.a) * tt,
    b: lab1.b + (lab2.b - lab1.b) * tt,
    alpha: lab1.alpha + (lab2.alpha - lab1.alpha) * tt,
  });
}

/**
 * lerpHex is the convenience wrapper that takes / returns hex
 * strings. Invalid inputs surface the parseHex error verbatim.
 */
export function lerpHex(a, b, t) {
  return formatHex(lerpRgb(parseHex(a), parseHex(b), t));
}
