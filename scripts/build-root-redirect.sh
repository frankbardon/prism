#!/usr/bin/env bash
# Emit the deploy root's index.html on stdout. Redirects /prism/ →
# /prism/latest/ so existing bookmarks land somewhere sensible while
# the versioned tree fills out.

set -euo pipefail

cat <<'HTML'
<!doctype html>
<meta charset="utf-8">
<title>Prism Documentation</title>
<meta http-equiv="refresh" content="0; url=/prism/latest/">
<link rel="canonical" href="/prism/latest/">
<style>body{font-family:system-ui,-apple-system,sans-serif;margin:2rem;color:#445}</style>
<p>Redirecting to <a href="/prism/latest/">the latest Prism documentation</a>.</p>
HTML
