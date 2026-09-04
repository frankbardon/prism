// prism-table.mjs — client-side table interactivity (E1-S5): single-
// column click-to-sort, client-side pagination, row-click selection.
//
// Operates directly on the DOM markup render/html/renderer.go emits
// for a table mark (see that file's renderTableMarkup doc comment for
// the full data-prism-* attribute contract). There is no WASM round
// trip here — a table mark never renders through prism.wasm's SVG-only
// bridge (cmd/prismwasm/main.go hardcodes svg.New()); the HTML the
// backend emits is meant to be served/mounted as-is (e.g. `prism plot
// --format html`, or a host page's own fetch), and this module wires
// interactivity onto it after it lands in the DOM.
//
// Public exports:
//   - installTableHandlers(root)      — wire every table under root
//   - sortValueOf(td)                 — read + JSON-parse a <td>'s
//                                        data-prism-sort-value
//   - compareSortValues(a, b)         — the sort comparator
//   - buildRowSelectionEvent(table, tr) — selection.Event-shaped
//                                          payload for a row click
//
// All exports stay synchronous, mirroring prism-selection.mjs.

const _store = new WeakMap();

function _wrapState(wrap) {
  let st = _store.get(wrap);
  if (!st) {
    st = { disposers: [], sort: null, page: 1 };
    _store.set(wrap, st);
  }
  return st;
}

/**
 * installTableHandlers wires header-click sort, pagination
 * prev/next, and row-click selection for every
 * `[data-prism-table-root]` wrapper found under `root` (an Element
 * or Document; `root` itself is included when it matches). Idempotent
 * — a second call on the same wrapper tears down its prior listeners
 * first, so re-running after replacing markup is safe.
 *
 * @param {Element|Document} root
 */
export function installTableHandlers(root) {
  if (!root || typeof root.querySelectorAll !== "function") return;
  const wraps = [];
  if (typeof root.matches === "function" && root.matches("[data-prism-table-root]")) {
    wraps.push(root);
  }
  for (const el of root.querySelectorAll("[data-prism-table-root]")) wraps.push(el);

  for (const wrap of wraps) {
    const table = wrap.querySelector("table.prism-html-table");
    if (!table) continue;
    _installOne(wrap, table);
  }
}

function _installOne(wrap, table) {
  const state = _wrapState(wrap);
  for (const d of state.disposers) {
    try { d(); } catch { /* defensive */ }
  }
  state.disposers = [];
  state.sort = null;
  state.page = 1;

  const thead = table.tHead || table.querySelector("thead");
  const tbody = table.tBodies ? table.tBodies[0] : table.querySelector("tbody");
  if (!thead || !tbody) return;

  // --- Column sort -------------------------------------------------
  const headerRow = thead.rows ? thead.rows[0] : thead.querySelector("tr");
  const headers = headerRow ? Array.from(headerRow.children).filter(el => el.tagName === "TH") : [];
  for (let i = 0; i < headers.length; i++) {
    const th = headers[i];
    const field = th.getAttribute("data-prism-field");
    if (!field) continue;
    const onClick = () => _sortByColumn(wrap, table, tbody, headers, i, field, state);
    th.addEventListener("click", onClick);
    state.disposers.push(() => th.removeEventListener("click", onClick));
  }

  // --- Pagination ----------------------------------------------------
  const pageSize = Number(table.getAttribute("data-prism-page-size")) || 0;
  const pagination = wrap.querySelector("[data-prism-table-pagination]");
  if (pagination && pageSize > 0) {
    const prevBtn = pagination.querySelector("[data-prism-page-prev]");
    const nextBtn = pagination.querySelector("[data-prism-page-next]");
    if (prevBtn) {
      const onPrev = () => _gotoPage(wrap, table, tbody, pagination, pageSize, state.page - 1, state);
      prevBtn.addEventListener("click", onPrev);
      state.disposers.push(() => prevBtn.removeEventListener("click", onPrev));
    }
    if (nextBtn) {
      const onNext = () => _gotoPage(wrap, table, tbody, pagination, pageSize, state.page + 1, state);
      nextBtn.addEventListener("click", onNext);
      state.disposers.push(() => nextBtn.removeEventListener("click", onNext));
    }
  }
  // Apply the initial slice — the server pre-rendered every row (so
  // the client has the full set to page through with no additional
  // round trip); the first page's worth is all that should be
  // visible until Next is clicked.
  if (pageSize > 0) _applyPage(tbody, pageSize, 1, pagination);

  // --- Row-click selection --------------------------------------------
  const onRowClick = (ev) => {
    const tr = _closestRow(ev.target, tbody);
    if (!tr) return;
    const detail = buildRowSelectionEvent(table, tr);
    if (!detail) return;
    const target = wrap;
    const event = new CustomEvent("prism:select", {
      detail,
      bubbles: true,
      composed: true,
      cancelable: false,
    });
    target.dispatchEvent(event);
  };
  tbody.addEventListener("click", onRowClick);
  state.disposers.push(() => tbody.removeEventListener("click", onRowClick));
}

function _closestRow(target, tbody) {
  let node = target;
  while (node && node !== tbody && node.nodeType === 1) {
    if (node.hasAttribute && node.hasAttribute("data-prism-datum-row")) return node;
    node = node.parentNode;
  }
  return null;
}

// ---------------------------------------------------------------- //
// Sort
// ---------------------------------------------------------------- //

/**
 * sortValueOf reads a <td>'s data-prism-sort-value attribute and
 * JSON-parses it back to the underlying field value (string, number,
 * boolean, null, or an array — e.g. a sparkline column's raw numeric
 * series). Returns null when the attribute is absent or unparseable.
 */
export function sortValueOf(td) {
  if (!td || typeof td.getAttribute !== "function") return null;
  const raw = td.getAttribute("data-prism-sort-value");
  if (raw === null) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

/**
 * compareSortValues orders two underlying field values:
 *   - both numbers  → numeric compare
 *   - both strings  → lexicographic compare
 *   - both arrays   → element-wise compare (first differing element
 *     wins; a shorter array that is a prefix of the other sorts
 *     first). This is what makes a sub-mark column (e.g. a sparkline,
 *     whose field value is a numeric series with no single scalar to
 *     reduce it to) sort meaningfully by its actual data instead of
 *     falling back to comparing rendered <svg> markup.
 *   - null/undefined sorts before any defined value
 *   - mixed types fall back to a string compare so the sort is always
 *     total (never throws, never leaves order undefined)
 */
export function compareSortValues(a, b) {
  const aNil = a === null || a === undefined;
  const bNil = b === null || b === undefined;
  if (aNil && bNil) return 0;
  if (aNil) return -1;
  if (bNil) return 1;

  if (Array.isArray(a) && Array.isArray(b)) {
    const n = Math.min(a.length, b.length);
    for (let i = 0; i < n; i++) {
      const c = compareSortValues(a[i], b[i]);
      if (c !== 0) return c;
    }
    return a.length - b.length;
  }
  if (typeof a === "number" && typeof b === "number") {
    return a - b;
  }
  if (typeof a === "string" && typeof b === "string") {
    return a < b ? -1 : a > b ? 1 : 0;
  }
  if (typeof a === "boolean" && typeof b === "boolean") {
    return (a === b) ? 0 : (a ? 1 : -1);
  }
  const sa = typeof a === "object" ? JSON.stringify(a) : String(a);
  const sb = typeof b === "object" ? JSON.stringify(b) : String(b);
  return sa < sb ? -1 : sa > sb ? 1 : 0;
}

function _sortByColumn(wrap, table, tbody, headers, colIndex, field, state) {
  const dir = (state.sort && state.sort.field === field && state.sort.dir === "asc") ? "desc" : "asc";
  state.sort = { field, dir };

  for (const th of headers) {
    if (th.getAttribute("data-prism-field") === field) {
      th.setAttribute("aria-sort", dir === "asc" ? "ascending" : "descending");
    } else {
      th.removeAttribute("aria-sort");
    }
  }

  const rows = Array.from(tbody.rows ? tbody.rows : tbody.querySelectorAll("tr"));
  const sign = dir === "asc" ? 1 : -1;
  rows.sort((r1, r2) => {
    const td1 = r1.children[colIndex];
    const td2 = r2.children[colIndex];
    return sign * compareSortValues(sortValueOf(td1), sortValueOf(td2));
  });

  const frag = tbody.ownerDocument.createDocumentFragment();
  for (const r of rows) frag.appendChild(r);
  tbody.appendChild(frag);

  // Sorting reshuffles row order — restart pagination at page 1 so
  // the visible page always reflects the current sort.
  const pageSize = Number(table.getAttribute("data-prism-page-size")) || 0;
  if (pageSize > 0) {
    state.page = 1;
    const pagination = wrap.querySelector("[data-prism-table-pagination]");
    _applyPage(tbody, pageSize, 1, pagination);
    state.page = 1;
  }
}

// ---------------------------------------------------------------- //
// Pagination
// ---------------------------------------------------------------- //

function _gotoPage(wrap, table, tbody, pagination, pageSize, page, state) {
  const rows = tbody.rows ? tbody.rows : tbody.querySelectorAll("tr");
  const totalPages = Math.max(1, Math.ceil(rows.length / pageSize));
  const clamped = Math.max(1, Math.min(totalPages, page));
  state.page = clamped;
  _applyPage(tbody, pageSize, clamped, pagination);
}

function _applyPage(tbody, pageSize, page, pagination) {
  const rows = Array.from(tbody.rows ? tbody.rows : tbody.querySelectorAll("tr"));
  const totalPages = Math.max(1, Math.ceil(rows.length / pageSize));
  const start = (page - 1) * pageSize;
  const end = start + pageSize;
  for (let i = 0; i < rows.length; i++) {
    rows[i].style.display = (i >= start && i < end) ? "" : "none";
  }
  if (!pagination) return;
  const indicator = pagination.querySelector("[data-prism-page-indicator]");
  if (indicator) indicator.textContent = `Page ${page} of ${totalPages}`;
  const prevBtn = pagination.querySelector("[data-prism-page-prev]");
  const nextBtn = pagination.querySelector("[data-prism-page-next]");
  if (prevBtn) prevBtn.disabled = page <= 1;
  if (nextBtn) nextBtn.disabled = page >= totalPages;
}

// ---------------------------------------------------------------- //
// Row-click selection event
// ---------------------------------------------------------------- //

/**
 * buildRowSelectionEvent assembles the structured selection.Event
 * payload (mirrors selection/event.go and prism-selection.mjs's
 * buildSelectionEvent) for a table row click. `kind` is always
 * "point" — a table row click is a single-datum pick, never an
 * interval/lasso. `selection_id` is read from the table's own
 * `data-prism-selection-id` attribute when present (a future story
 * may wire an explicit spec.Selection to a table mark), otherwise
 * defaults to "table" so every table-row click event carries a
 * stable, non-empty ID.
 */
export function buildRowSelectionEvent(table, tr) {
  if (!table || !tr) return null;
  const rowRaw = tr.getAttribute("data-prism-datum-row");
  const rowID = Number(rowRaw);
  if (!Number.isFinite(rowID)) return null;

  const sceneID = table.getAttribute("data-prism-scene") || "";
  const layerID = table.getAttribute("data-prism-layer") || "";
  const selectionID = table.getAttribute("data-prism-selection-id") || "table";
  const instanceKey = `${layerID}:${rowID}`;

  return {
    scene_id: sceneID,
    selection_id: selectionID,
    kind: "point",
    timestamp: Date.now(),
    marks: [{ mark_index: 0, instance_key: instanceKey }],
    data_rows: [{ dataset_name: "", row_index: rowID }],
    data_extent: undefined,
    pixel_extent: undefined,
    spec_path: `/selection/${selectionID}`,
    // Back-compat keys, matching prism-selection.mjs's event shape:
    id: selectionID,
    state: { points: [{ layer_id: layerID, row_id: rowID }], range: null },
  };
}

/**
 * _resetForTests clears stored per-wrapper state (disposers, sort,
 * page). Test escape hatch; production code should not call this.
 */
export function _resetForTests(wrap) {
  if (wrap) _store.delete(wrap);
}
