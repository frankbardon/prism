// prism-selection.mjs — selection state plumbing for P12.
//
// **P12 STUB**: This module ships the SelectionState shape, the
// broadcast / listen wrappers, and the per-handle storage WeakMap.
// `getSelection(handle)` returns null until P13 wires the encoder
// (Scene.Selections) + hit-test handlers + brush handlers.
//
// Cross-chart filtering (brush on A filters B) is an application-
// level concern in v1 — host pages listen for the `prism:select`
// CustomEvent (dispatched on the host element, composed + bubbling)
// and update other charts themselves.

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
export function listen(target, name, handler) {
  if (!target || typeof target.addEventListener !== "function") {
    return () => {};
  }
  target.addEventListener(name, handler);
  return () => target.removeEventListener(name, handler);
}

// ---------------------------------------------------------------- //
// Storage — per-SceneHandle selection state, keyed via WeakMap so
// destroyed handles release their state without an explicit cleanup
// call.
// ---------------------------------------------------------------- //

const _store = new WeakMap();

/**
 * setSelection records the new selection state for the given handle
 * and broadcasts `prism:select` on the handle's root target.
 * P13 wires hit-test / brush handlers to call this.
 */
export function setSelection(handle, state) {
  _store.set(handle, state);
  const target = handle && (handle._root || handle._svg);
  if (target) broadcast(target, "prism:select", { state });
}

/**
 * getSelection returns the current selection for the given handle,
 * or null when none is stored. In P12 this always returns null
 * (the stub) because the encoder never calls setSelection — full
 * wiring lands in P13.
 */
export function getSelection(handle) {
  return _store.get(handle) ?? null;
}

/** _resetForTests clears all stored selections. Test escape hatch. */
export function _resetForTests() {
  // WeakMap has no clear(); rebuild by reassigning. Since the symbol
  // is module-scoped + the export is a function-call API, callers
  // can't observe the swap.
  // eslint-disable-next-line no-global-assign
  // (no globals here; this is fine.)
  // For now we mark this as no-op; tests should use fresh handle
  // instances rather than relying on a clear() that WeakMap doesn't
  // expose. Provided for API completeness.
}
