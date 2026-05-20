// prism-element.mjs — <prism-chart> + <prism-dataset> custom elements.
//
// Auto-registers both elements on import. Host page mounts via:
//
//   <script type="module" src="/static/vendor/prism/prism-element.mjs"></script>
//
//   <prism-dataset name="current" src="/data/q1.pulse"></prism-dataset>
//   <prism-chart src="/scenes/brand_score.json"></prism-chart>
//
// Both use shadow DOM (mode: "open") so CSS variables from the
// host page propagate into the SVG `<style>` block (D073).

import { render } from "./prism.mjs";
import { PrismResolver } from "./prism-resolver.mjs";
import { listen } from "./prism-selection.mjs";

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
    // Track in-flight render so attributeChangedCallback can cancel
    // stale work when the user rapidly toggles attributes.
    this._renderToken = 0;
  }

  connectedCallback() {
    if (!this.shadowRoot) this.attachShadow({ mode: "open" });
    this._render();
  }

  attributeChangedCallback(name, oldVal, newVal) {
    if (oldVal === newVal) return;
    // Re-render on any tracked attribute change (D073: theme switch
    // triggers a re-render rather than a client-side recompute).
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

  async _render() {
    const token = ++this._renderToken;
    this._teardown();

    const src  = this.getAttribute("src");
    const spec = this.getAttribute("spec");

    let sceneDoc = null;
    if (src) {
      try {
        sceneDoc = await PrismResolver.fetchJSON(src);
      } catch (err) {
        this._renderError(`Failed to load scene from ${src}: ${err.message}`);
        return;
      }
    } else if (spec) {
      // v1: client-side spec compile requires a server endpoint
      // (P14 ships /prism/scene). Render a placeholder note.
      this._renderError("Client-side spec compile requires the server endpoint (lands in P14). Use the `src` attribute to point at a precompiled Scene JSON.");
      return;
    } else {
      this._renderError("No `src` or `spec` attribute provided.");
      return;
    }

    if (token !== this._renderToken) return; // stale work
    if (!this.isConnected) return;            // detached mid-flight

    // Clear shadow root before re-rendering so attribute toggles
    // don't stack DOM trees.
    while (this.shadowRoot.firstChild) {
      this.shadowRoot.removeChild(this.shadowRoot.firstChild);
    }

    try {
      this._handle = render(sceneDoc, this.shadowRoot);
    } catch (err) {
      this._renderError(`Render failed: ${err.message}`);
      return;
    }

    // Forward selection events from the SceneHandle's root onto the
    // host element. Use composed:true so listeners outside the shadow
    // boundary receive the event (matches D073).
    const dispose = listen(this.shadowRoot, "prism:select", (e) => {
      this.dispatchEvent(new CustomEvent("prism:select", {
        detail: e.detail,
        bubbles: true,
        composed: true,
      }));
    });
    this._unsubscribers.push(dispose);
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
}

export { PrismChart, PrismDataset };
