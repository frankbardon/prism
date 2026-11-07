// prism-element.mjs — <prism-chart> + <prism-dataset> +
// <prism-coordinator> + <prism-table> custom elements.
//
// Auto-registers all four elements on import. Host page mounts via:
//
//   <script type="module" src="/static/vendor/prism/prism-element.mjs"></script>
//
//   <prism-dataset name="current" src="/data/q1.pulse"></prism-dataset>
//   <prism-chart id="overview" src="/scenes/brand_score.json"></prism-chart>
//   <prism-table src="/scenes/accounts_table.json"></prism-table>
//
//   <prism-coordinator>
//     <prism-chart id="a" src="/scenes/overview.json"></prism-chart>
//     <prism-chart id="b" src="/scenes/detail.json"></prism-chart>
//   </prism-coordinator>
//
// Charts use shadow DOM (mode: "open") so CSS variables from the
// host page propagate into the SVG `<style>` block (D073). <prism-table>
// follows the same shadow-DOM pattern (see the PrismTable class below,
// near the end of the file, alongside its own doc comment).

import { render, executeSpec, ensureWasmReady } from "./prism.mjs";
import { PrismResolver } from "./prism-resolver.mjs";
import {
  listen,
  setSelection,
  getAllSelections,
} from "./prism-selection.mjs";
import { installTableHandlers } from "./prism-table.mjs";

// ---------------------------------------------------------------- //
// URL-hash state helpers (D079)
// ---------------------------------------------------------------- //

const HASH_PREFIX = "prism-sel:";
const HASH_BUDGET = 1024;
const OVERFLOW_MARKER = "prism-sel:overflow";

/** Encode a stateMap to a URL-safe base64 hash payload. Empty map → "". */
export function serialiseStateMap(stateMap) {
  if (!stateMap || Object.keys(stateMap).length === 0) return "";
  const json = JSON.stringify(stateMap);
  // btoa requires a binary string; encode UTF-8 first.
  const b64 = _toBase64Url(json);
  return HASH_PREFIX + b64;
}

/** Decode a hash payload back to a stateMap. Invalid → null. */
export function deserialiseStateMap(hashPayload) {
  if (!hashPayload || typeof hashPayload !== "string") return null;
  let p = hashPayload;
  if (p.startsWith("#")) p = p.slice(1);
  if (!p.startsWith(HASH_PREFIX)) return null;
  const b64 = p.slice(HASH_PREFIX.length);
  if (!b64 || b64 === "overflow") return null;
  try {
    const json = _fromBase64Url(b64);
    return JSON.parse(json);
  } catch (e) {
    return null;
  }
}

function _toBase64Url(s) {
  // UTF-8 bytes → binary string → btoa → url-safe.
  const bytes = new TextEncoder().encode(s);
  let bin = "";
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
  const b64 = (typeof btoa !== "undefined") ? btoa(bin) : Buffer.from(bin, "binary").toString("base64");
  return b64.replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function _fromBase64Url(s) {
  let b64 = s.replace(/-/g, "+").replace(/_/g, "/");
  while (b64.length % 4) b64 += "=";
  const bin = (typeof atob !== "undefined") ? atob(b64) : Buffer.from(b64, "base64").toString("binary");
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new TextDecoder().decode(bytes);
}

// ---------------------------------------------------------------- //
// <prism-dataset>
// ---------------------------------------------------------------- //

class PrismDataset extends HTMLElement {
  static get observedAttributes() {
    return ["name", "src"];
  }

  connectedCallback() {
    const name = this.getAttribute("name");
    const src  = this.getAttribute("src");
    if (name && src) PrismResolver.register(name, src);
  }

  attributeChangedCallback(name, oldValue, newValue) {
    // On a name change, unregister the old; re-register on the new.
    if (name === "name" && oldValue && oldValue !== newValue) {
      PrismResolver.unregister(oldValue);
    }
    const cur = this.getAttribute("name");
    const src = this.getAttribute("src");
    if (cur && src) PrismResolver.register(cur, src);
  }

  disconnectedCallback() {
    const name = this.getAttribute("name");
    if (name) PrismResolver.unregister(name);
  }
}

// ---------------------------------------------------------------- //
// <prism-chart>
// ---------------------------------------------------------------- //

class PrismChart extends HTMLElement {
  static get observedAttributes() {
    return ["src", "spec", "theme", "selection-mode"];
  }

  constructor() {
    super();
    this._handle = null;
    this._unsubscribers = [];
    this._renderToken = 0;
  }

  connectedCallback() {
    if (!this.shadowRoot) this.attachShadow({ mode: "open" });
    this._render();
  }

  attributeChangedCallback(name, oldVal, newVal) {
    if (oldVal === newVal) return;
    if (this.shadowRoot) this._render();
  }

  disconnectedCallback() {
    this._teardown();
  }

  _teardown() {
    if (this._handle) {
      try { this._handle.destroy(); } catch { /* noop */ }
      this._handle = null;
    }
    for (const dispose of this._unsubscribers) {
      try { dispose(); } catch { /* noop */ }
    }
    this._unsubscribers = [];
  }

  /**
   * _chartKey returns the URL-state lookup key. Element id wins;
   * fallback is pathname + position-in-document (charts without ids
   * still get stable keys within a page).
   */
  _chartKey() {
    if (this.id) return this.id;
    const charts = (this.ownerDocument || document).querySelectorAll("prism-chart");
    const idx = Array.prototype.indexOf.call(charts, this);
    const path = (typeof window !== "undefined" && window.location) ? window.location.pathname : "";
    return `${path}:${idx}`;
  }

  async _render() {
    const token = ++this._renderToken;
    // Hold the previous handle alive across the render so we can
    // animate the swap when the new scene has an animation block.
    // _teardown only runs after we know we're committing to an
    // instant replace (no animation, fetch failure, etc.).
    const previousHandle = this._handle;
    this._handle = null;

    const src  = this.getAttribute("src");
    const spec = this.getAttribute("spec");

    let sceneDoc = null;
    if (src) {
      try {
        sceneDoc = await PrismResolver.fetchJSON(src);
      } catch (err) {
        this._handle = previousHandle;
        this._teardown();
        this._renderError(`Failed to load scene from ${src}: ${err.message}`);
        return;
      }
    } else if (spec) {
      // Two shapes accepted: an inline JSON literal (`spec='{"$schema":...}'`)
      // or a URL pointing at a `.prism.json` file (`spec="/specs/foo.prism.json"`).
      // The latter mirrors the Scene URL path so static hosts can serve raw
      // specs without compiling them up front.
      let parsed;
      const trimmed = spec.trim();
      if (trimmed.startsWith("{")) {
        try { parsed = JSON.parse(trimmed); }
        catch (err) {
          this._handle = previousHandle;
          this._teardown();
          this._renderError(`Spec attribute is not valid JSON: ${err.message}`);
          return;
        }
      } else {
        try { parsed = await PrismResolver.fetchJSON(trimmed); }
        catch (err) {
          this._handle = previousHandle;
          this._teardown();
          this._renderError(`Failed to load spec from ${trimmed}: ${err.message}`);
          return;
        }
      }
      try {
        const datasets = PrismResolver.snapshot();
        const opts = {};
        const themeAttr = this.getAttribute("theme");
        if (themeAttr) opts.theme = themeAttr;
        sceneDoc = await executeSpec(parsed, datasets, opts);
      } catch (err) {
        const code = err.prismCode ? `${err.prismCode}: ` : "";
        this._handle = previousHandle;
        this._teardown();
        this._renderError(`Spec compile failed: ${code}${err.message}`);
        return;
      }
    } else {
      this._handle = previousHandle;
      this._teardown();
      this._renderError("No `src` or `spec` attribute provided.");
      return;
    }

    if (token !== this._renderToken) {
      this._handle = previousHandle;
      this._teardown();
      return;
    }
    if (!this.isConnected) {
      this._handle = previousHandle;
      this._teardown();
      return;
    }

    // Animation path: when the previous render is still alive and the
    // new sceneDoc declares an `animation` block, hand the swap off
    // to SceneHandle.update which runs PrismAnimator. Otherwise fall
    // through to the existing clear + render path.
    const hasAnimation = !!sceneDoc?.grid?.cells?.[0]?.scene?.animation;
    if (previousHandle && hasAnimation) {
      try {
        const updated = await previousHandle.update(sceneDoc);
        if (token !== this._renderToken || !this.isConnected) {
          try { updated.destroy(); } catch {}
          return;
        }
        this._handle = updated;
        // Detach previous selection listeners; new handle wires fresh.
        for (const dispose of this._unsubscribers) {
          try { dispose(); } catch {}
        }
        this._unsubscribers = [];
        this._wireSelectionEvents();
        try { this._restoreState(); } catch {}
        return;
      } catch (err) {
        // Fall through to instant replace on animate failure.
        try { previousHandle.destroy(); } catch {}
      }
    } else if (previousHandle) {
      try { previousHandle.destroy(); } catch {}
      for (const dispose of this._unsubscribers) {
        try { dispose(); } catch {}
      }
      this._unsubscribers = [];
    }

    while (this.shadowRoot.firstChild) {
      this.shadowRoot.removeChild(this.shadowRoot.firstChild);
    }

    try {
      this._handle = await render(sceneDoc, this.shadowRoot);
    } catch (err) {
      this._renderError(`Render failed: ${err.message}`);
      return;
    }
    if (token !== this._renderToken) return; // newer render started while WASM resolved
    if (!this.isConnected) return;

    this._wireSelectionEvents();
    // Seed initial selection state from URL hash / localStorage (D079).
    try { this._restoreState(); } catch { /* defensive */ }
  }

  // _wireSelectionEvents attaches a single `prism:select` re-emitter
  // on the shadow root and remembers the disposer. Called from
  // _render after a fresh mount AND after an animated handle swap.
  _wireSelectionEvents() {
    const dispose = listen(this.shadowRoot, "prism:select", (e) => {
      this.dispatchEvent(new CustomEvent("prism:select", {
        detail: e.detail,
        bubbles: true,
        composed: true,
      }));
      try { this._persistState(); } catch { /* defensive */ }
    });
    this._unsubscribers.push(dispose);
  }

  _restoreState() {
    if (typeof window === "undefined" || !window.location) return;
    if (!this._handle) return;
    const key = this._chartKey();
    const hash = window.location.hash || "";
    let raw = null;
    if (hash === "#" + OVERFLOW_MARKER) {
      try {
        raw = window.localStorage && window.localStorage.getItem(HASH_PREFIX + key);
      } catch { raw = null; }
      if (!raw) return;
      let parsed;
      try { parsed = JSON.parse(raw); } catch { return; }
      _applyStateMapToChart(this._handle, key, parsed);
      return;
    }
    const map = deserialiseStateMap(hash);
    if (!map) return;
    _applyStateMapToChart(this._handle, key, map);
  }

  _persistState() {
    if (typeof window === "undefined" || !window.location) return;
    if (!this._handle) return;
    const key = this._chartKey();
    const mySelections = getAllSelections(this._handle);
    // Merge into existing global state map.
    let global = deserialiseStateMap(window.location.hash) || {};
    if (window.location.hash === "#" + OVERFLOW_MARKER) {
      try {
        const raw = window.localStorage && window.localStorage.getItem(HASH_PREFIX + key);
        if (raw) global = JSON.parse(raw);
      } catch { global = {}; }
    }
    if (!global || typeof global !== "object") global = {};
    if (Object.keys(mySelections).length === 0) {
      delete global[key];
    } else {
      global[key] = mySelections;
    }
    const encoded = serialiseStateMap(global);
    if (encoded.length <= HASH_BUDGET) {
      // Clear any prior localStorage overflow entry — hash is now sufficient.
      try { window.localStorage && window.localStorage.removeItem(HASH_PREFIX + key); } catch {}
      _replaceHash(encoded ? "#" + encoded : "");
    } else {
      try {
        window.localStorage && window.localStorage.setItem(HASH_PREFIX + key, JSON.stringify(global));
      } catch {}
      _replaceHash("#" + OVERFLOW_MARKER);
    }
  }

  _renderError(message) {
    if (!this.shadowRoot) return;
    while (this.shadowRoot.firstChild) {
      this.shadowRoot.removeChild(this.shadowRoot.firstChild);
    }
    const div = (this.ownerDocument || document).createElement("div");
    div.setAttribute("class", "prism-chart-error");
    div.setAttribute("role", "alert");
    div.style.cssText = "font-family: monospace; font-size: 12px; color: #b91c1c; padding: 8px; border: 1px solid #fecaca; background: #fef2f2;";
    div.textContent = `prism-chart: ${message}`;
    this.shadowRoot.appendChild(div);
  }
}

function _applyStateMapToChart(handle, key, stateMap) {
  if (!handle || !stateMap || typeof stateMap !== "object") return;
  const slice = stateMap[key];
  if (!slice || typeof slice !== "object") return;
  for (const [selID, state] of Object.entries(slice)) {
    setSelection(handle, selID, state);
  }
}

function _replaceHash(newHash) {
  if (typeof window === "undefined" || !window.history || !window.history.replaceState) return;
  try {
    const cur = window.location.pathname + window.location.search;
    window.history.replaceState({}, "", cur + newHash);
  } catch { /* defensive */ }
}

// ---------------------------------------------------------------- //
// <prism-coordinator> (D082)
// ---------------------------------------------------------------- //

const _COORDINATED = "__prism_coordinated__";

class PrismCoordinator extends HTMLElement {
  constructor() {
    super();
    this._observer = null;
    this._listener = null;
  }

  connectedCallback() {
    // Single delegated listener — composedPath identifies the source.
    this._listener = (ev) => {
      const detail = ev.detail || {};
      if (detail[_COORDINATED]) return; // loop guard
      const path = (typeof ev.composedPath === "function") ? ev.composedPath() : [];
      const source = path.find(n => n && n.tagName && n.tagName.toLowerCase() === "prism-chart");
      const charts = this.querySelectorAll("prism-chart");
      for (const sibling of charts) {
        if (sibling === source) continue;
        // Only re-dispatch to siblings that declare the matching selection ID.
        if (!sibling._handle) continue;
        const has = (sibling._handle._selections || []).some(s => s.id === detail.id);
        if (!has) continue;
        // Apply state silently first so the visual response matches
        // the new state without spawning a re-dispatch loop, then fire
        // the marker event on the sibling so app-level listeners see
        // the coordinated change.
        try { setSelection(sibling._handle, detail.id, detail.state, { silent: true }); } catch {}
        sibling.dispatchEvent(new CustomEvent("prism:select", {
          detail: Object.assign({}, detail, { [_COORDINATED]: true }),
          bubbles: false,
          composed: true,
        }));
      }
    };
    this.addEventListener("prism:select", this._listener, true);
  }

  disconnectedCallback() {
    if (this._listener) {
      this.removeEventListener("prism:select", this._listener, true);
      this._listener = null;
    }
  }
}

// ---------------------------------------------------------------- //
// <prism-table> (E4-S2)
// ---------------------------------------------------------------- //
//
// Live counterpart to the server/CLI-only table path
// (`prism plot --format html`, `render/html`'s renderTableMarkup):
// resolves a sceneDoc the same two ways <prism-chart> does (`src` →
// PrismResolver.fetchJSON; `spec` → inline JSON or a URL, compiled via
// executeSpec), then renders it through the HTML backend
// (`globalThis.prism.renderHTML`, see docs/src/concepts/browser.md's
// "Render backends: SVG vs HTML" section) instead of the SVG backend
// <prism-chart> uses — the `table` mark has no SVG geometry of its
// own. The returned string is a complete standalone HTML document
// (doctype + html/head/body); parsing it via a throwaway wrapper's
// innerHTML and moving its children into the shadow root keeps the
// `<style>` tag's table CSS (border-collapse, header cursor, page
// controls, `.prism-selected` row highlight — see
// render/html/renderer.go's renderTableDoc) working, and lands the
// `[data-prism-table-root]` wrapper `installTableHandlers` expects to
// find (see prism-table.mjs's own doc comment for the full
// data-prism-* attribute contract render/html/renderer.go emits).
//
// There is no persistent SceneHandle-like object here (renderHTML is
// a synchronous string-in/string-out bridge, not a live DOM handle) —
// installTableHandlers' own listener bookkeeping (a WeakMap keyed by
// the `[data-prism-table-root]` wrapper element, see prism-table.mjs)
// is released for garbage collection once that wrapper is replaced or
// the shadow root it lives in is discarded, so disconnectedCallback
// has nothing to explicitly tear down beyond bumping the render token
// so an in-flight render doesn't mount after the element is gone.
class PrismTable extends HTMLElement {
  static get observedAttributes() {
    return ["src", "spec", "theme"];
  }

  constructor() {
    super();
    this._renderToken = 0;
  }

  connectedCallback() {
    if (!this.shadowRoot) this.attachShadow({ mode: "open" });
    this._render();
  }

  attributeChangedCallback(name, oldVal, newVal) {
    if (oldVal === newVal) return;
    if (this.shadowRoot) this._render();
  }

  disconnectedCallback() {
    // Invalidate any in-flight _render() so it can't mount into a
    // detached shadow root after the element leaves the document.
    this._renderToken++;
  }

  async _render() {
    const token = ++this._renderToken;
    const src = this.getAttribute("src");
    const spec = this.getAttribute("spec");
    const themeAttr = this.getAttribute("theme");

    let sceneDoc = null;
    if (src) {
      try {
        sceneDoc = await PrismResolver.fetchJSON(src);
      } catch (err) {
        this._renderError(`Failed to load scene from ${src}: ${err.message}`);
        return;
      }
    } else if (spec) {
      // Same two shapes <prism-chart> accepts: an inline JSON literal
      // or a URL pointing at a `.prism.json` file.
      let parsed;
      const trimmed = spec.trim();
      if (trimmed.startsWith("{")) {
        try { parsed = JSON.parse(trimmed); }
        catch (err) {
          this._renderError(`Spec attribute is not valid JSON: ${err.message}`);
          return;
        }
      } else {
        try { parsed = await PrismResolver.fetchJSON(trimmed); }
        catch (err) {
          this._renderError(`Failed to load spec from ${trimmed}: ${err.message}`);
          return;
        }
      }
      try {
        const datasets = PrismResolver.snapshot();
        const opts = {};
        if (themeAttr) opts.theme = themeAttr;
        sceneDoc = await executeSpec(parsed, datasets, opts);
      } catch (err) {
        const code = err.prismCode ? `${err.prismCode}: ` : "";
        this._renderError(`Spec compile failed: ${code}${err.message}`);
        return;
      }
    } else {
      this._renderError("No `src` or `spec` attribute provided.");
      return;
    }

    if (token !== this._renderToken || !this.isConnected) return;

    let htmlString;
    try {
      await ensureWasmReady();
      htmlString = globalThis.prism.renderHTML(JSON.stringify(sceneDoc), themeAttr || undefined);
      _throwIfHTMLErrorEnvelope(htmlString, "prism-table");
    } catch (err) {
      this._renderError(`Render failed: ${err.message}`);
      return;
    }

    if (token !== this._renderToken || !this.isConnected) return;

    const doc = this.ownerDocument || document;
    const wrapper = doc.createElement("div");
    wrapper.innerHTML = htmlString;

    while (this.shadowRoot.firstChild) {
      this.shadowRoot.removeChild(this.shadowRoot.firstChild);
    }
    while (wrapper.firstChild) {
      this.shadowRoot.appendChild(wrapper.firstChild);
    }

    try { installTableHandlers(this.shadowRoot); } catch { /* defensive */ }
  }

  _renderError(message) {
    if (!this.shadowRoot) return;
    while (this.shadowRoot.firstChild) {
      this.shadowRoot.removeChild(this.shadowRoot.firstChild);
    }
    const div = (this.ownerDocument || document).createElement("div");
    div.setAttribute("class", "prism-table-error");
    div.setAttribute("role", "alert");
    div.style.cssText = "font-family: monospace; font-size: 12px; color: #b91c1c; padding: 8px; border: 1px solid #fecaca; background: #fef2f2;";
    div.textContent = `prism-table: ${message}`;
    this.shadowRoot.appendChild(div);
  }
}

// _throwIfHTMLErrorEnvelope mirrors prism.mjs's private
// _throwIfErrorEnvelope helper (not exported — renderHTML has no
// prism.mjs-level wrapper the way render()/executeSpec() do, so this
// module calls the raw `globalThis.prism.renderHTML` bridge directly
// and must sniff its error-envelope shape itself). The WASM bridge
// returns either the HTML document string or a JSON `{"ok":false,...}`
// envelope; sniff the literal prefix rather than parsing every
// document body just to check.
function _throwIfHTMLErrorEnvelope(payload, where) {
  if (typeof payload !== "string") return;
  if (!payload.startsWith(`{"ok":false`)) return;
  let parsed;
  try { parsed = JSON.parse(payload); } catch { return; }
  if (parsed && parsed.ok === false) {
    const err = parsed.error || parsed;
    const code = err.code || err.Code || "PRISM_WASM_001";
    const msg = err.message || err.Message || "WASM bridge error";
    const fixups = err.fixups || err.Fixups || [];
    const composed = new Error(`${where}: ${code}: ${msg}`);
    composed.prismCode = code;
    composed.prismFixups = fixups;
    throw composed;
  }
}

// ---------------------------------------------------------------- //
// Registration — idempotent. Re-importing the module does nothing.
// ---------------------------------------------------------------- //

if (typeof customElements !== "undefined") {
  if (!customElements.get("prism-dataset")) {
    customElements.define("prism-dataset", PrismDataset);
  }
  if (!customElements.get("prism-chart")) {
    customElements.define("prism-chart", PrismChart);
  }
  if (!customElements.get("prism-coordinator")) {
    customElements.define("prism-coordinator", PrismCoordinator);
  }
  if (!customElements.get("prism-table")) {
    customElements.define("prism-table", PrismTable);
  }
}

export { PrismChart, PrismDataset, PrismCoordinator, PrismTable };
