// table-interactivity.mjs — TestPrismTableInteractivity (E1-S5).
//
// Exercises static/vendor/prism/prism-table.mjs against the exact
// data-prism-* attribute contract render/html/renderer.go emits (see
// its renderTableMarkup doc comment). No WASM required — a table mark
// never renders through prism.wasm's SVG-only bridge, so this harness
// builds the fixture markup by hand rather than invoking the wasm
// pipeline, mirroring animator-tween.mjs's "no WASM required" shape.
//
// Assertions:
//   1. Header click sorts rows ascending by the column's underlying
//      field VALUE (a typed JSON number from data-prism-sort-value),
//      not by lexicographic string comparison of rendered text — a
//      naive text sort of "5" wired ahead of the row worth 80 alone
//      would fail this.
//   2. A second click on the same header reverses to descending.
//   3. The "trend" column (data-prism-sort-value carrying a JSON
//      array, standing in for a sparkline sub-mark whose visible
//      content is an <svg> with no meaningful textContent to sort
//      by) sorts correctly by its array value — proving the sort
//      reads the attribute, not the cell's rendered markup.
//   4. Pagination: page_size=2 over 5 rows shows only page 1's rows
//      initially, Next/Prev re-slice client-side (no re-render call),
//      and the page indicator + button disabled states track the
//      current page.
//   5. A row click dispatches `prism:select` on the wrapper with a
//      selection.Event-shaped detail (scene_id/selection_id/kind/
//      marks/data_rows/spec_path) keyed off data-prism-datum-row.
//
// Driven by internal/devtools/table_interactivity_test.go.

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
  fail("happy-dom not installed: " + e.message);
}

const window = new Window({ url: "http://localhost/" });
globalThis.window = window;
globalThis.document = window.document;
globalThis.HTMLElement = window.HTMLElement;
globalThis.CustomEvent = window.CustomEvent;
globalThis.MouseEvent = window.MouseEvent;

const {
  installTableHandlers,
  compareSortValues,
  sortValueOf,
  buildRowSelectionEvent,
} = await import(resolve(REPO, "static/vendor/prism/prism-table.mjs"));

// ---------------------------------------------------------------- //
// Fixture: 5 rows, page_size=2, one text column ("name"), one
// numeric column ("revenue") whose values would sort WRONG under a
// naive string comparison ("120" < "5" < "80" lexicographically, but
// 5 < 80 < 120 numerically), and one array column ("trend") standing
// in for a sparkline sub-mark.
// ---------------------------------------------------------------- //

const rows = [
  { id: 0, name: "Acme", revenue: 120, trend: [9, 9, 9] },
  { id: 1, name: "Globex", revenue: 5, trend: [1, 1, 1] },
  { id: 2, name: "Initech", revenue: 80, trend: [5, 5, 5] },
  { id: 3, name: "Umbrella", revenue: 200, trend: [7, 7, 7] },
  { id: 4, name: "Soylent", revenue: 40, trend: [3, 3, 3] },
];

function esc(s) {
  return String(s).replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;");
}

function rowHTML(r) {
  return `<tr data-prism-datum-row="${r.id}">
    <td data-prism-sort-value="${esc(JSON.stringify(r.name))}">${esc(r.name)}</td>
    <td data-prism-sort-value="${esc(JSON.stringify(r.revenue))}">${r.revenue}</td>
    <td data-prism-sort-value="${esc(JSON.stringify(r.trend))}"><svg></svg></td>
  </tr>`;
}

document.body.innerHTML = `
<div class="prism-html-table-wrap" data-prism-table-root>
<table class="prism-html-table" data-prism-scene="scene-0" data-prism-layer="layer-0" data-prism-page-size="2">
<thead><tr>
  <th data-prism-field="name">Account</th>
  <th data-prism-field="revenue">revenue</th>
  <th data-prism-field="trend">Trend</th>
</tr></thead>
<tbody>
${rows.map(rowHTML).join("\n")}
</tbody>
</table>
<div class="prism-html-table-pagination" data-prism-table-pagination>
  <button type="button" data-prism-page-prev>Prev</button>
  <span data-prism-page-indicator>Page 1 of 3</span>
  <button type="button" data-prism-page-next>Next</button>
</div>
</div>
`;

const wrap = document.querySelector("[data-prism-table-root]");
const table = wrap.querySelector("table");
const tbody = table.querySelector("tbody");
const pagination = wrap.querySelector("[data-prism-table-pagination]");
const headers = Array.from(table.querySelectorAll("th"));

installTableHandlers(document);

function visibleNames() {
  return Array.from(tbody.querySelectorAll("tr"))
    .filter((tr) => tr.style.display !== "none")
    .map((tr) => tr.querySelector("td").textContent);
}
function allNamesInOrder() {
  return Array.from(tbody.querySelectorAll("tr")).map((tr) => tr.querySelector("td").textContent);
}

// --- 1/2. Sort by numeric "revenue" column ------------------------------

const revenueHeader = headers.find((h) => h.getAttribute("data-prism-field") === "revenue");
revenueHeader.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));

let order = allNamesInOrder();
const wantAsc = ["Globex", "Soylent", "Initech", "Acme", "Umbrella"]; // 5,40,80,120,200
if (JSON.stringify(order) !== JSON.stringify(wantAsc)) {
  fail(`ascending revenue sort order = ${JSON.stringify(order)}, want ${JSON.stringify(wantAsc)}`);
}
if (revenueHeader.getAttribute("aria-sort") !== "ascending") {
  fail(`expected aria-sort=ascending on revenue header, got ${revenueHeader.getAttribute("aria-sort")}`);
}

revenueHeader.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
order = allNamesInOrder();
const wantDesc = [...wantAsc].reverse();
if (JSON.stringify(order) !== JSON.stringify(wantDesc)) {
  fail(`descending revenue sort order = ${JSON.stringify(order)}, want ${JSON.stringify(wantDesc)}`);
}
if (revenueHeader.getAttribute("aria-sort") !== "descending") {
  fail(`expected aria-sort=descending on revenue header, got ${revenueHeader.getAttribute("aria-sort")}`);
}

// --- 3. Sort by the array-valued "trend" column (sparkline stand-in) ---

const trendHeader = headers.find((h) => h.getAttribute("data-prism-field") === "trend");
trendHeader.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
order = allNamesInOrder();
// trend values: Acme=[9,9,9] Globex=[1,1,1] Initech=[5,5,5] Umbrella=[7,7,7] Soylent=[3,3,3]
const wantTrendAsc = ["Globex", "Soylent", "Initech", "Umbrella", "Acme"];
if (JSON.stringify(order) !== JSON.stringify(wantTrendAsc)) {
  fail(`ascending trend (array) sort order = ${JSON.stringify(order)}, want ${JSON.stringify(wantTrendAsc)} — sort must read data-prism-sort-value, not rendered <svg> markup`);
}

// Direct comparator + reader unit coverage (exported for this reason).
if (compareSortValues(1, 2) >= 0) fail("compareSortValues(1,2) should be negative");
if (compareSortValues([1, 2], [1, 3]) >= 0) fail("compareSortValues([1,2],[1,3]) should be negative");
if (compareSortValues("a", "b") >= 0) fail('compareSortValues("a","b") should be negative');
const firstTd = tbody.querySelector("tr td");
if (sortValueOf(firstTd) === null) fail("sortValueOf returned null for a td with data-prism-sort-value");

// --- 4. Pagination ------------------------------------------------------
// Sorting resets to page 1; verify only 2 rows visible post-sort.

let visible = visibleNames();
if (visible.length !== 2) fail(`expected 2 visible rows on page 1 after sort, got ${visible.length}: ${JSON.stringify(visible)}`);
if (visible.join(",") !== wantTrendAsc.slice(0, 2).join(",")) {
  fail(`page 1 rows = ${JSON.stringify(visible)}, want first two of ${JSON.stringify(wantTrendAsc)}`);
}
const indicator = pagination.querySelector("[data-prism-page-indicator]");
if (indicator.textContent !== "Page 1 of 3") fail(`indicator = ${indicator.textContent}, want "Page 1 of 3"`);
const prevBtn = pagination.querySelector("[data-prism-page-prev]");
const nextBtn = pagination.querySelector("[data-prism-page-next]");
if (!prevBtn.disabled) fail("expected Prev disabled on page 1");
if (nextBtn.disabled) fail("expected Next enabled on page 1");

nextBtn.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
visible = visibleNames();
if (visible.length !== 2) fail(`expected 2 visible rows on page 2, got ${visible.length}`);
if (visible.join(",") !== wantTrendAsc.slice(2, 4).join(",")) {
  fail(`page 2 rows = ${JSON.stringify(visible)}, want ${JSON.stringify(wantTrendAsc.slice(2, 4))}`);
}
if (indicator.textContent !== "Page 2 of 3") fail(`indicator = ${indicator.textContent}, want "Page 2 of 3"`);
if (prevBtn.disabled) fail("expected Prev enabled on page 2");
if (nextBtn.disabled) fail("expected Next enabled on page 2");

nextBtn.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
visible = visibleNames();
if (visible.length !== 1) fail(`expected 1 visible row on page 3 (5 rows / page_size 2), got ${visible.length}`);
if (indicator.textContent !== "Page 3 of 3") fail(`indicator = ${indicator.textContent}, want "Page 3 of 3"`);
if (!nextBtn.disabled) fail("expected Next disabled on last page");

nextBtn.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
if (indicator.textContent !== "Page 3 of 3") fail("Next should be a no-op once disabled/on last page");

prevBtn.dispatchEvent(new window.MouseEvent("click", { bubbles: true }));
if (indicator.textContent !== "Page 2 of 3") fail(`Prev from page 3 → indicator = ${indicator.textContent}, want "Page 2 of 3"`);

// --- 5. Row-click selection ---------------------------------------------

let captured = null;
wrap.addEventListener("prism:select", (ev) => { captured = ev.detail; });

const clickedRow = tbody.querySelector("tr");
const clickedRowID = Number(clickedRow.getAttribute("data-prism-datum-row"));
clickedRow.querySelector("td").dispatchEvent(new window.MouseEvent("click", { bubbles: true }));

if (!captured) fail("prism:select did not fire after row click");
if (captured.kind !== "point") fail(`detail.kind = ${captured.kind}, want point`);
if (captured.scene_id !== "scene-0") fail(`detail.scene_id = ${captured.scene_id}, want scene-0`);
if (captured.selection_id !== "table") fail(`detail.selection_id = ${captured.selection_id}, want table (default)`);
if (!Array.isArray(captured.marks) || captured.marks.length !== 1) fail("detail.marks should have exactly one entry");
if (captured.marks[0].instance_key !== `layer-0:${clickedRowID}`) {
  fail(`detail.marks[0].instance_key = ${captured.marks[0].instance_key}, want layer-0:${clickedRowID}`);
}
if (!Array.isArray(captured.data_rows) || captured.data_rows.length !== 1 || captured.data_rows[0].row_index !== clickedRowID) {
  fail(`detail.data_rows = ${JSON.stringify(captured.data_rows)}, want one entry with row_index ${clickedRowID}`);
}
if (captured.spec_path !== "/selection/table") fail(`detail.spec_path = ${captured.spec_path}, want /selection/table`);

// buildRowSelectionEvent should be independently callable (used above
// only indirectly; call it directly here for coverage of the export).
const direct = buildRowSelectionEvent(table, clickedRow);
if (!direct || direct.kind !== "point") fail("buildRowSelectionEvent direct call did not return a point event");

console.error("PASS: header-click sort (text/numeric/array columns), pagination slicing, and row-click selection all behave per contract");
try { await window.happyDOM?.close(); } catch { /* defensive */ }
process.exit(0);
