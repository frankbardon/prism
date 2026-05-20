// main.mjs — Node entrypoint for TestCrossImplSVGParity.
//
// Usage:
//   node main.mjs <fixture-name>
//
// Reads:    testdata/cross_impl/<fixture>/scene.json
// Renders:  via static/vendor/prism/prism.mjs into happy-dom
// Writes:   testdata/cross_impl/<fixture>/js.svg
//
// happy-dom (>=15) provides enough DOM + CustomEvent + shadow-root
// surface for SVG construction + serialisation via outerHTML.
// Pixel parity is out of scope — we diff SVG text bytes (D076).

import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { resolve, dirname } from "node:path";

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO = resolve(HERE, "../../..");

const fixture = process.argv[2];
if (!fixture) {
  console.error("usage: main.mjs <fixture>");
  process.exit(2);
}

const scenePath = resolve(REPO, "testdata", "cross_impl", fixture, "scene.json");
const outPath   = resolve(REPO, "testdata", "cross_impl", fixture, "js.svg");

// Wire happy-dom globals. Done before importing prism.mjs so the
// module's references to document/HTMLElement resolve.
let Window;
try {
  ({ Window } = await import("happy-dom"));
} catch (err) {
  console.error("main.mjs: happy-dom not installed. Run `npm install` inside internal/devtools/cross-impl-runner/.");
  console.error(err.message);
  process.exit(3);
}

const window = new Window({ url: "http://localhost/" });
globalThis.window         = window;
globalThis.document       = window.document;
globalThis.HTMLElement    = window.HTMLElement;
globalThis.CustomEvent    = window.CustomEvent;
globalThis.customElements = window.customElements;
globalThis.fetch          = window.fetch?.bind(window);

const sceneText = await readFile(scenePath, "utf-8");
const scene = JSON.parse(sceneText);

const { render } = await import(resolve(REPO, "static/vendor/prism/prism.mjs"));

const handle = render(scene, document.body);
const svg = document.body.querySelector("svg");
if (!svg) {
  console.error("main.mjs: render produced no <svg> child");
  process.exit(4);
}

// Serialise via outerHTML. happy-dom returns lowercase tag names +
// double-quoted attributes by default, matching the Go writer's
// output. We post-process minimally for parity:
//   - happy-dom doesn't emit the xmlns attribute on the SVG root
//     unless we set it explicitly (we did via setAttribute, so it
//     should be present).
//   - happy-dom collapses self-closing tags differently; we run a
//     small normaliser below.
let out = svg.outerHTML;

// Normalisation pass: collapse `></rect>` to `/>` for void-style
// elements when Go uses the self-closing form. We can iterate this
// further once the first parity run shows the actual diff shape;
// keeping the initial pass minimal for visibility.

await writeFile(outPath, out + "\n", "utf-8");
console.error(`main.mjs: wrote ${outPath} (${out.length} bytes)`);

// Tear down to keep the Node process exit quick.
try { handle?.destroy?.(); } catch {}
try { await window.happyDOM?.close(); } catch {}
