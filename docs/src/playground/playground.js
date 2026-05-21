// playground.js — Prism interactive playground controller.
//
// Wires a JSON editor to the WASM bridge exported by `prism.mjs`,
// debouncing edits into compile → render cycles. Surfaces error
// envelopes from the Go pipeline (PRISM_* codes + fixups) inline.
//
// State persistence: spec, theme, and inspector tab are encoded in
// the URL hash via deflate-raw + base64url. The hash is the source
// of truth on load; the example picker writes to it.

import { executeSpec, ensureWasmReady } from "../static/prism/prism.mjs";

const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => Array.from(document.querySelectorAll(sel));

// ---------------------------------------------------------------- //
// Globals
// ---------------------------------------------------------------- //

const els = {
  editor:        $("#pg-editor"),
  gutter:        $("#pg-gutter"),
  preview:       $("#pg-preview"),
  previewWrap:   $("#pg-preview-wrap"),
  errorBar:      $("#pg-error-bar"),
  status:        $("#pg-status"),
  sidebar:       $("#pg-sidebar"),
  themeSelect:   $("#pg-theme"),
  uiThemeBtn:    $("#pg-ui-theme"),
  formatBtn:     $("#pg-format"),
  shareBtn:      $("#pg-share"),
  resetBtn:      $("#pg-reset"),
  downloadSvg:   $("#pg-dl-svg"),
  downloadScene: $("#pg-dl-scene"),
  inspectorTabs: $("#pg-inspector-tabs"),
  inspectorBody: $("#pg-inspector-body"),
  loading:       $("#pg-loading"),
  loadingText:   $("#pg-loading-text"),
  toast:         $("#pg-toast"),
  versionLabel:  $("#pg-version"),
};

const DEFAULT_EXAMPLE = "bar";
const RENDER_DEBOUNCE_MS = 250;
const STORAGE_KEY = "prism-playground:v1";

const state = {
  specText:       "",
  chartTheme:     "light",
  inspectorTab:   "svg",
  lastScene:      null,
  lastSvg:        "",
  lastError:      null,
  examples:       [],
  manifest:       null,
  activeExample:  null,
  renderToken:    0,
  uiTheme:        "light",
};

let renderTimer = null;

// ---------------------------------------------------------------- //
// Boot
// ---------------------------------------------------------------- //

async function main() {
  applyUiTheme(loadUiThemePref());

  await loadManifest();
  renderSidebar();
  bindUi();
  restoreFromHashOrStorage();

  // Kick the WASM boot in parallel with first paint; the first
  // render call will await it anyway, but starting now hides the
  // load latency behind editor interaction.
  ensureWasmReady().then(() => {
    if (globalThis.prism && typeof globalThis.prism.version === "function") {
      els.versionLabel.textContent = globalThis.prism.version();
    }
    // Geo bundle URL — geoshape / geopoint marks fetch from this path
    // on first encode. Relative so it works under any mdBook nesting
    // (local serve, GH Pages, custom subdomain).
    if (globalThis.prism && globalThis.prism.geo && typeof globalThis.prism.geo.setBundleURL === "function") {
      globalThis.prism.geo.setBundleURL(new URL("../static/prism/geodata/", document.baseURI).href);
    }
    setLoading(false);
    scheduleRender(0);
  }).catch(err => {
    setLoading(false);
    showError({
      code: "PRISM_WASM_001",
      message: `WASM failed to load: ${err.message || err}`,
      fixups: [],
    });
  });
}

// ---------------------------------------------------------------- //
// Manifest & examples
// ---------------------------------------------------------------- //

async function loadManifest() {
  const res = await fetch("examples/manifest.json");
  state.manifest = await res.json();
  state.examples = state.manifest.groups.flatMap(g => g.examples);
}

function renderSidebar() {
  const html = state.manifest.groups.map(g => `
    <h3>${escapeHtml(g.name)}</h3>
    <ul>
      ${g.examples.map(ex => `
        <li data-example-id="${escapeAttr(ex.id)}" title="${escapeAttr(ex.title)}">${escapeHtml(ex.title)}</li>
      `).join("")}
    </ul>
  `).join("");
  els.sidebar.innerHTML = html;
  els.sidebar.addEventListener("click", (ev) => {
    const li = ev.target.closest("li[data-example-id]");
    if (!li) return;
    loadExample(li.dataset.exampleId);
  });
}

async function loadExample(id) {
  const ex = state.examples.find(e => e.id === id);
  if (!ex) return;
  state.activeExample = id;
  highlightActiveExample();
  try {
    const res = await fetch(`examples/${ex.file}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const text = await res.text();
    setSpecText(prettyPrint(text));
    scheduleRender(0);
  } catch (err) {
    showError({
      code: "PRISM_WASM_001",
      message: `Failed to load example "${id}": ${err.message || err}`,
      fixups: [],
    });
  }
}

function highlightActiveExample() {
  $$(".pg-sidebar li").forEach(li => {
    li.classList.toggle("active", li.dataset.exampleId === state.activeExample);
  });
}

// ---------------------------------------------------------------- //
// Editor
// ---------------------------------------------------------------- //

function setSpecText(text) {
  state.specText = text;
  els.editor.value = text;
  updateGutter();
}

function updateGutter() {
  const lines = els.editor.value.split("\n").length;
  let html = "";
  for (let i = 1; i <= lines; i++) html += i + "\n";
  els.gutter.textContent = html;
  els.gutter.scrollTop = els.editor.scrollTop;
}

function prettyPrint(text) {
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}

function bindUi() {
  els.editor.addEventListener("input", () => {
    state.specText = els.editor.value;
    updateGutter();
    state.activeExample = null;
    highlightActiveExample();
    scheduleRender();
  });

  els.editor.addEventListener("scroll", () => {
    els.gutter.scrollTop = els.editor.scrollTop;
  });

  els.editor.addEventListener("keydown", (ev) => {
    // Tab inserts two spaces; Shift-Tab outdents one level.
    if (ev.key === "Tab") {
      ev.preventDefault();
      const ta = els.editor;
      const start = ta.selectionStart;
      const end = ta.selectionEnd;
      const value = ta.value;
      if (!ev.shiftKey) {
        const insert = "  ";
        ta.value = value.slice(0, start) + insert + value.slice(end);
        ta.selectionStart = ta.selectionEnd = start + insert.length;
      } else {
        // Outdent: remove up to 2 spaces at line start.
        const lineStart = value.lastIndexOf("\n", start - 1) + 1;
        const head = value.slice(lineStart, start);
        const trim = head.startsWith("  ") ? 2 : (head.startsWith(" ") ? 1 : 0);
        if (trim > 0) {
          ta.value = value.slice(0, lineStart) + value.slice(lineStart + trim);
          ta.selectionStart = ta.selectionEnd = start - trim;
        }
      }
      state.specText = ta.value;
      updateGutter();
      scheduleRender();
      return;
    }

    // Ctrl/Cmd + S => format (suppress browser save dialog)
    if ((ev.ctrlKey || ev.metaKey) && ev.key.toLowerCase() === "s") {
      ev.preventDefault();
      formatSpec();
      return;
    }
    // Ctrl/Cmd + Enter => force render
    if ((ev.ctrlKey || ev.metaKey) && ev.key === "Enter") {
      ev.preventDefault();
      scheduleRender(0);
    }
  });

  els.themeSelect.addEventListener("change", () => {
    state.chartTheme = els.themeSelect.value;
    els.previewWrap.dataset.chartTheme = state.chartTheme;
    scheduleRender(0);
  });

  els.uiThemeBtn.addEventListener("click", () => {
    const next = state.uiTheme === "light" ? "dark" : "light";
    applyUiTheme(next);
    saveUiThemePref(next);
  });

  els.formatBtn.addEventListener("click", formatSpec);
  els.shareBtn.addEventListener("click", copyShareLink);
  els.resetBtn.addEventListener("click", () => loadExample(state.activeExample || DEFAULT_EXAMPLE));

  els.downloadSvg.addEventListener("click", () => downloadFile("chart.svg", state.lastSvg, "image/svg+xml"));
  els.downloadScene.addEventListener("click", () => {
    if (!state.lastScene) { toast("No scene to download yet."); return; }
    downloadFile("scene.json", JSON.stringify(state.lastScene, null, 2), "application/json");
  });

  els.inspectorTabs.addEventListener("click", (ev) => {
    const btn = ev.target.closest("button[data-tab]");
    if (!btn) return;
    setInspectorTab(btn.dataset.tab);
  });

  // Save spec to URL hash + localStorage on every change (debounced).
  let persistTimer = null;
  els.editor.addEventListener("input", () => {
    clearTimeout(persistTimer);
    persistTimer = setTimeout(persistState, 500);
  });
}

function formatSpec() {
  const pretty = prettyPrint(els.editor.value);
  if (pretty !== els.editor.value) {
    setSpecText(pretty);
    persistState();
    scheduleRender(0);
    toast("Formatted.");
  }
}

// ---------------------------------------------------------------- //
// Render pipeline
// ---------------------------------------------------------------- //

function scheduleRender(delay = RENDER_DEBOUNCE_MS) {
  clearTimeout(renderTimer);
  renderTimer = setTimeout(runRender, delay);
}

async function runRender() {
  const token = ++state.renderToken;
  setStatus("working", "compiling…");

  let spec;
  try {
    spec = JSON.parse(state.specText);
  } catch (err) {
    showError({
      code: "PRISM_SPEC_009",
      message: `Invalid JSON: ${err.message}`,
      fixups: [],
    });
    setStatus("error", "JSON error");
    return;
  }

  // Wait for WASM if first call.
  try {
    await ensureWasmReady();
  } catch (err) {
    showError({
      code: "PRISM_WASM_001",
      message: `WASM not ready: ${err.message || err}`,
      fixups: [],
    });
    setStatus("error", "WASM error");
    return;
  }

  if (token !== state.renderToken) return; // superseded

  // Run the full pipeline via WASM.
  try {
    const opts = { theme: state.chartTheme };
    const scene = await executeSpec(spec, undefined, opts);
    if (token !== state.renderToken) return;
    const svg = globalThis.prism.render(JSON.stringify(scene), state.chartTheme);
    if (typeof svg === "string" && svg.startsWith(`{"ok":false`)) {
      const env = JSON.parse(svg);
      throw envelopeError(env);
    }
    state.lastScene = scene;
    state.lastSvg = svg;
    state.lastError = null;
    mountSvg(svg);
    clearError();
    setStatus("ok", "ok");
    refreshInspector();
  } catch (err) {
    if (token !== state.renderToken) return;
    showError({
      code: err.prismCode || "PRISM_WASM_001",
      message: err.message || String(err),
      fixups: err.prismFixups || [],
      seeAlso: err.prismSeeAlso || [],
    });
    setStatus("error", err.prismCode || "error");
  }
}

function mountSvg(svgString) {
  els.preview.innerHTML = svgString;
  const svg = els.preview.querySelector("svg");
  if (svg) {
    // Strip fixed width/height so the SVG scales responsively; the
    // viewBox carries the aspect ratio. This matches the project's
    // "SVG renderer responsive by default" memory.
    svg.removeAttribute("width");
    svg.removeAttribute("height");
    svg.style.width = "100%";
    svg.style.height = "auto";
  }
}

// ---------------------------------------------------------------- //
// Error display
// ---------------------------------------------------------------- //

function showError({ code, message, fixups, seeAlso }) {
  state.lastError = { code, message, fixups, seeAlso };
  const fixupsHtml = (fixups || []).map(fx => {
    const title = fx.title || fx.Title || "Fixup";
    const body  = fx.template || fx.Template || fx.body || fx.Body || "";
    return `<details class="pg-fixups"><summary>${escapeHtml(title)}</summary><pre>${escapeHtml(body)}</pre></details>`;
  }).join("");
  const seeAlsoHtml = (seeAlso || []).length > 0
    ? `<div class="pg-fixups"><strong>See also:</strong> ${seeAlso.map(s => escapeHtml(s)).join(", ")}</div>`
    : "";

  els.errorBar.className = "pg-error-bar error";
  els.errorBar.innerHTML = `
    <div class="pg-error-head">${escapeHtml(code || "ERROR")}</div>
    <div class="pg-error-msg">${escapeHtml(message || "")}</div>
    ${fixupsHtml}
    ${seeAlsoHtml}
  `;
}

function clearError() {
  els.errorBar.className = "pg-error-bar ok";
  els.errorBar.innerHTML = "✓ rendered";
}

function envelopeError(env) {
  const e = env.error || env;
  const err = new Error(e.Message || e.message || "WASM bridge error");
  err.prismCode = e.Code || e.code || "PRISM_WASM_001";
  err.prismFixups = e.Fixups || e.fixups || [];
  err.prismSeeAlso = e.SeeAlso || e.seeAlso || [];
  return err;
}

// ---------------------------------------------------------------- //
// Inspector tabs (SVG / Scene / Spec)
// ---------------------------------------------------------------- //

function setInspectorTab(name) {
  state.inspectorTab = name;
  $$("#pg-inspector-tabs button").forEach(btn => {
    btn.classList.toggle("active", btn.dataset.tab === name);
  });
  refreshInspector();
  persistState();
}

function refreshInspector() {
  const body = els.inspectorBody;
  let text = "";
  if (state.inspectorTab === "svg") {
    text = state.lastSvg || "";
  } else if (state.inspectorTab === "scene") {
    text = state.lastScene ? JSON.stringify(state.lastScene, null, 2) : "";
  } else if (state.inspectorTab === "spec") {
    text = state.specText || "";
  } else if (state.inspectorTab === "plan") {
    text = describePlan();
  }
  if (!text) {
    body.className = "pg-inspector-body empty";
    body.textContent = state.lastError
      ? "Fix the spec error above to populate this tab."
      : "Compiling…";
  } else {
    body.className = "pg-inspector-body";
    body.textContent = text;
  }
}

function describePlan() {
  if (!globalThis.prism || typeof globalThis.prism.plan !== "function") return "";
  if (!state.specText) return "";
  try {
    JSON.parse(state.specText);
  } catch {
    return "(spec has JSON errors)";
  }
  const out = globalThis.prism.plan(state.specText, "");
  if (typeof out === "string" && out.startsWith(`{"ok":false`)) {
    const env = JSON.parse(out);
    const e = env.error || env;
    return `${e.Code || e.code}: ${e.Message || e.message}`;
  }
  try {
    return JSON.stringify(JSON.parse(out), null, 2);
  } catch {
    return out || "";
  }
}

// ---------------------------------------------------------------- //
// Hash / localStorage persistence + sharing
// ---------------------------------------------------------------- //

async function restoreFromHashOrStorage() {
  // 1) Hash wins (shared link).
  const hashSpec = await decodeHashSpec();
  if (hashSpec) {
    setSpecText(hashSpec.text || "");
    if (hashSpec.theme) {
      state.chartTheme = hashSpec.theme;
      els.themeSelect.value = hashSpec.theme;
      els.previewWrap.dataset.chartTheme = state.chartTheme;
    }
    if (hashSpec.tab) setInspectorTab(hashSpec.tab);
    return;
  }
  // 2) localStorage.
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const saved = JSON.parse(raw);
      if (saved.specText) {
        setSpecText(saved.specText);
        if (saved.chartTheme) {
          state.chartTheme = saved.chartTheme;
          els.themeSelect.value = saved.chartTheme;
          els.previewWrap.dataset.chartTheme = saved.chartTheme;
        }
        if (saved.inspectorTab) setInspectorTab(saved.inspectorTab);
        if (saved.activeExample) {
          state.activeExample = saved.activeExample;
          highlightActiveExample();
        }
        return;
      }
    }
  } catch {}
  // 3) Default example.
  await loadExample(DEFAULT_EXAMPLE);
}

function persistState() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({
      specText:      state.specText,
      chartTheme:    state.chartTheme,
      inspectorTab:  state.inspectorTab,
      activeExample: state.activeExample,
    }));
  } catch {}
}

async function copyShareLink() {
  const payload = {
    text:  state.specText,
    theme: state.chartTheme,
    tab:   state.inspectorTab,
  };
  const hash = await encodeHashSpec(payload);
  const url = `${location.origin}${location.pathname}#s=${hash}`;
  try {
    await navigator.clipboard.writeText(url);
    toast("Share link copied to clipboard.");
  } catch {
    // Fallback: drop URL into the location bar.
    location.hash = `s=${hash}`;
    toast("URL updated — copy from the address bar.");
  }
}

// Hash codec: compress JSON with CompressionStream('deflate-raw'),
// then base64url. Falls back to plain base64url-encoded JSON when
// compression APIs are absent (very old Safari).

async function encodeHashSpec(payload) {
  const json = JSON.stringify(payload);
  if (typeof CompressionStream === "function") {
    const cs = new CompressionStream("deflate-raw");
    const stream = new Blob([json]).stream().pipeThrough(cs);
    const buf = await new Response(stream).arrayBuffer();
    return "c1." + b64UrlEncode(new Uint8Array(buf));
  }
  return "p1." + b64UrlEncode(new TextEncoder().encode(json));
}

async function decodeHashSpec() {
  const hash = location.hash || "";
  const m = hash.match(/^#s=([cp]1\.[A-Za-z0-9_-]+)$/);
  if (!m) return null;
  const [tag, body] = m[1].split(".");
  try {
    const bytes = b64UrlDecode(body);
    let jsonBytes = bytes;
    if (tag === "c1") {
      if (typeof DecompressionStream === "function") {
        const ds = new DecompressionStream("deflate-raw");
        const stream = new Blob([bytes]).stream().pipeThrough(ds);
        jsonBytes = new Uint8Array(await new Response(stream).arrayBuffer());
      } else {
        return null;
      }
    }
    const json = new TextDecoder().decode(jsonBytes);
    return JSON.parse(json);
  } catch (err) {
    console.warn("playground: bad hash payload", err);
    return null;
  }
}

function b64UrlEncode(bytes) {
  let bin = "";
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
  return btoa(bin).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

function b64UrlDecode(s) {
  let str = s.replaceAll("-", "+").replaceAll("_", "/");
  while (str.length % 4) str += "=";
  const bin = atob(str);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return bytes;
}

// ---------------------------------------------------------------- //
// Misc UI helpers
// ---------------------------------------------------------------- //

function setStatus(kind, text) {
  els.status.className = `pg-status ${kind}`;
  els.status.textContent = text;
}

function setLoading(on, text) {
  els.loading.classList.toggle("hidden", !on);
  if (text) els.loadingText.textContent = text;
}

let toastTimer = null;
function toast(text) {
  clearTimeout(toastTimer);
  els.toast.textContent = text;
  els.toast.classList.add("show");
  toastTimer = setTimeout(() => els.toast.classList.remove("show"), 1800);
}

function downloadFile(name, content, mime) {
  if (!content) { toast("Nothing to download yet."); return; }
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function escapeHtml(s) {
  return String(s)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function escapeAttr(s) { return escapeHtml(s); }

// ---------------------------------------------------------------- //
// UI theme (light/dark for the playground chrome itself)
// ---------------------------------------------------------------- //

function applyUiTheme(theme) {
  state.uiTheme = theme;
  document.documentElement.dataset.pgTheme = theme;
  els.uiThemeBtn.textContent = theme === "dark" ? "☀" : "☾";
  els.uiThemeBtn.title = theme === "dark" ? "Switch to light UI" : "Switch to dark UI";
}

function loadUiThemePref() {
  try {
    const saved = localStorage.getItem("prism-playground:ui-theme");
    if (saved === "dark" || saved === "light") return saved;
  } catch {}
  // Match the user's OS preference if available.
  if (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) return "dark";
  return "light";
}

function saveUiThemePref(theme) {
  try { localStorage.setItem("prism-playground:ui-theme", theme); } catch {}
}

main();
