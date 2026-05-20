// dataset-registry-dedupe.mjs — TestPrismDatasetRegistryDedupe.
//
// Asserts that PrismResolver.fetch memoises by URL: multiple
// consumers requesting the same `src` share one round-trip
// (D074). PHASE.md: "two Pulse fetches (not six) because shared
// datasets".
//
// Test plan:
//   - Mock globalThis.fetch with a counter.
//   - Call PrismResolver.fetch(srcA) three times — assert counter == 1.
//   - Call PrismResolver.fetch(srcB) twice — assert counter == 2.
//   - Verify register/unregister bookkeeping.
//
// Exits 0 on pass, 1 with error to stderr on fail.

import { fileURLToPath } from "node:url";
import { resolve, dirname } from "node:path";

const HERE = dirname(fileURLToPath(import.meta.url));
const REPO = resolve(HERE, "../../..");

function fail(msg) {
  console.error(`FAIL: ${msg}`);
  process.exit(1);
}

// happy-dom for fetch + ArrayBuffer + TextEncoder availability,
// though we override fetch.
let Window;
try {
  ({ Window } = await import("happy-dom"));
} catch (e) {
  fail("happy-dom not installed");
}
const window = new Window({ url: "http://localhost/" });
globalThis.window = window;

let counter = 0;
const lastURLs = [];
globalThis.fetch = async (url) => {
  counter++;
  lastURLs.push(url);
  return {
    ok: true,
    status: 200,
    arrayBuffer: async () => new TextEncoder().encode(`{"url":"${url}"}`).buffer,
    text:        async () => `{"url":"${url}"}`,
    json:        async () => ({ url }),
  };
};

const { PrismResolver } = await import(resolve(REPO, "static/vendor/prism/prism-resolver.mjs"));

// Fresh state.
PrismResolver._resetForTests();
if (Object.keys(PrismResolver.snapshot()).length !== 0) {
  fail("snapshot not empty after _resetForTests");
}

const srcA = "/data/cohort-a.pulse";
const srcB = "/data/cohort-b.pulse";

PrismResolver.register("current", srcA);
PrismResolver.register("bench",   srcA);
PrismResolver.register("prior",   srcB);

const snap = PrismResolver.snapshot();
if (Object.keys(snap).length !== 3) {
  fail(`snapshot size = ${Object.keys(snap).length}, want 3`);
}
if (snap.current !== srcA || snap.bench !== srcA || snap.prior !== srcB) {
  fail(`snapshot bindings wrong: ${JSON.stringify(snap)}`);
}

// Fetch srcA three times — should dedupe to 1 call.
const a1 = await PrismResolver.fetch(srcA);
const a2 = await PrismResolver.fetch(srcA);
const a3 = await PrismResolver.fetch(srcA);
if (counter !== 1) {
  fail(`after 3× fetch(srcA), counter = ${counter}, want 1 (URL dedupe)`);
}
if (!(a1 instanceof ArrayBuffer)) {
  fail("fetch(srcA) did not return an ArrayBuffer");
}
// Identity-equal because the Promise is memoised.
if (a1 !== a2 || a2 !== a3) {
  fail("three fetch(srcA) calls returned non-identical results — Promise not memoised");
}

// Fetch srcB — counter increments to 2.
const b1 = await PrismResolver.fetch(srcB);
if (counter !== 2) {
  fail(`after fetch(srcB), counter = ${counter}, want 2`);
}
if (b1 === a1) {
  fail("fetch(srcA) and fetch(srcB) returned the same ArrayBuffer — incorrect dedupe");
}

// unregister current → still bound to srcA via 'bench'.
PrismResolver.unregister("current");
if (PrismResolver.resolve("current") !== undefined) {
  fail("unregister('current') did not remove the binding");
}
if (PrismResolver.resolve("bench") !== srcA) {
  fail("unregister('current') accidentally removed 'bench'");
}

// Same URL, fresh fetch attempt — still deduped.
await PrismResolver.fetch(srcA);
if (counter !== 2) {
  fail(`after un-register + re-fetch(srcA), counter = ${counter}, want 2 (cache not purged on unregister; intentional)`);
}

console.error(`PASS: 3× fetch(srcA) + 2× fetch(srcB) + unregister round-trip → ${counter} fetch calls (urls=${JSON.stringify(lastURLs)})`);
try { await window.happyDOM?.close(); } catch {}
process.exit(0);
