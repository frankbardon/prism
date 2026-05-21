// Prism docs version dropdown.
//
// Fetches /prism/versions.json (relative to the docs site root) and
// renders a <select> in the mdBook header. Selecting a version
// navigates to the same page under the new version prefix; the new
// version's index handles missing paths via its own 404.
//
// Designed to no-op when the manifest is missing or the dropdown is
// already wired — safe to load on every page.

(function () {
  if (window.__prismVersionDropdownMounted) return;
  window.__prismVersionDropdownMounted = true;

  function inferSiteRoot() {
    // GitHub Pages mounts under /<repo>/<version>/...
    // Walk back from the current path until we hit /<repo>/.
    var parts = location.pathname.split('/').filter(Boolean);
    if (parts.length === 0) return '/';
    // Common case: /prism/<version>/<page>. Strip everything after
    // the first segment.
    return '/' + parts[0] + '/';
  }

  function inferCurrentVersion() {
    var parts = location.pathname.split('/').filter(Boolean);
    return parts.length >= 2 ? parts[1] : '';
  }

  async function fetchManifest(siteRoot) {
    try {
      var res = await fetch(siteRoot + 'versions.json', { cache: 'no-store' });
      if (!res.ok) return null;
      return await res.json();
    } catch (_) {
      return null;
    }
  }

  function mountSelect(versions, currentVersion) {
    var header = document.querySelector('.menu-bar') || document.querySelector('#menu-bar');
    if (!header) return;
    if (header.querySelector('.prism-version-dropdown')) return;

    var wrapper = document.createElement('div');
    wrapper.className = 'prism-version-dropdown';

    var select = document.createElement('select');
    select.setAttribute('aria-label', 'Documentation version');
    versions.forEach(function (v) {
      var opt = document.createElement('option');
      opt.value = v.id;
      opt.textContent = v.label || v.id;
      opt.dataset.path = v.path;
      if (v.id === currentVersion) opt.selected = true;
      select.appendChild(opt);
    });
    select.addEventListener('change', function () {
      var opt = select.options[select.selectedIndex];
      var targetPath = opt.dataset.path || ('/' + opt.value + '/');
      // Best-effort: preserve the current page path under the new
      // version. mdBook URLs end with .html so we try to map.
      var current = location.pathname;
      var siteRoot = inferSiteRoot();
      var versionRoot = siteRoot + (currentVersion ? currentVersion + '/' : '');
      if (current.indexOf(versionRoot) === 0) {
        var rest = current.slice(versionRoot.length);
        location.assign(targetPath + rest);
      } else {
        location.assign(targetPath);
      }
    });
    wrapper.appendChild(select);
    header.appendChild(wrapper);
  }

  async function init() {
    var siteRoot = inferSiteRoot();
    var manifest = await fetchManifest(siteRoot);
    if (!manifest || !Array.isArray(manifest.versions)) return;
    mountSelect(manifest.versions, inferCurrentVersion());
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
