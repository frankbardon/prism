// prism-resolver.mjs — page-level dataset registry.
//
// The `<prism-dataset>` element populates this registry on
// connectedCallback so `<prism-chart>` elements can reference
// datasets by name. The fetch helper dedupes by URL: two charts
// requesting the same `src` share one in-flight Promise → one
// network round-trip, N consumers (D074).
//
// Cache is per-page (module singleton); reset on navigation. The
// browser's HTTP cache handles cross-page dedupe via Cache-Control
// headers, which is outside Prism's concern.

const _registry = new Map(); // name → src
const _fetches  = new Map(); // src → Promise<ArrayBuffer>

export const PrismResolver = {
  /** register binds a dataset name to a src URL. Called by
   *  <prism-dataset> on connectedCallback. */
  register(name, src) {
    if (!name) return;
    _registry.set(name, src);
  },

  /** unregister removes the binding. Called by <prism-dataset>
   *  on disconnectedCallback. */
  unregister(name) {
    if (!name) return;
    _registry.delete(name);
  },

  /** resolve returns the src URL bound to `name`, or undefined. */
  resolve(name) {
    return _registry.get(name);
  },

  /** snapshot returns the current registry as a plain object.
   *  Used by the spec-compile path to ship dataset references to
   *  the server (P14). */
  snapshot() {
    return Object.fromEntries(_registry);
  },

  /** fetch returns a Promise<ArrayBuffer> for the given src URL,
   *  memoising by URL so multiple consumers share one round-trip.
   *  Mirrors design/08-browser.md "Three live charts; two Pulse
   *  fetches (not six)". */
  fetch(src) {
    if (!src) return Promise.reject(new TypeError("PrismResolver.fetch: empty src"));
    if (!_fetches.has(src)) {
      const p = fetch(src).then(r => {
        if (!r.ok) {
          throw new Error(`PrismResolver.fetch: ${src} → HTTP ${r.status}`);
        }
        return r.arrayBuffer();
      });
      _fetches.set(src, p);
    }
    return _fetches.get(src);
  },

  /** fetchJSON is a convenience for Scene JSON loads, where the
   *  payload is text → JSON. Same dedupe semantics; the JSON
   *  decode happens once per shared promise so consumers get
   *  identical objects (cheaper than redecoding per-consumer). */
  fetchJSON(src) {
    return this.fetch(src).then(buf => {
      const text = new TextDecoder("utf-8").decode(buf);
      return JSON.parse(text);
    });
  },

  /** _resetForTests clears both maps. Test escape hatch. */
  _resetForTests() {
    _registry.clear();
    _fetches.clear();
  },
};
