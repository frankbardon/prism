package html_test

import (
	"bytes"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/frankbardon/prism/custommark"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// E5-S1: two HTMLCustomRenderer example implementations demonstrating
// the `custom` mark mechanism documented in
// docs/src/cookbook/custom-marks.md — a quote/pull-quote card and a
// stat/metric card. Both are example code, not part of any public
// package: they exist to be tested here (escaping proof + gallery
// fixture regeneration) and mirrored in the cookbook's prose.
//
// Neither implements SVGCustomRenderer — a pull-quote or metric tile
// is inherently HTML-shaped (blockquote/footer, a bordered div with a
// large number) with no natural non-browser SVG equivalent, unlike
// the cookbook's "badge" example, which intentionally implements both
// interfaces to demonstrate that dual-method contract. Under the
// `svg` backend these fall back to the <foreignObject> wrapping path
// documented in the cookbook's dual-method table.

// quoteCardRenderer renders the first row as a pull-quote block: an
// italic quote plus an attribution line (author, optionally with a
// role/title). Recognized fields: "quote", "author", "role" (optional).
type quoteCardRenderer struct{}

func (quoteCardRenderer) RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	if len(rows) == 0 {
		return "", nil
	}
	quote, _ := rows[0]["quote"].(string)
	author, _ := rows[0]["author"].(string)
	role, _ := rows[0]["role"].(string)

	// Row data is untrusted — html.EscapeString neutralizes &<>"' in
	// every interpolated field. This package does not sanitize custom
	// mark output; escaping is this renderer's job, per the cookbook's
	// security contract.
	safeQuote := html.EscapeString(quote)
	attribution := html.EscapeString(author)
	if role != "" {
		attribution = fmt.Sprintf("%s, %s", html.EscapeString(author), html.EscapeString(role))
	}

	return fmt.Sprintf(
		`<blockquote style="margin:0;max-width:%.0fpx;padding:20px 24px;`+
			`border-left:4px solid %s;background:%s;font:italic 16px/1.5 %s;color:%s">`+
			`<p style="margin:0 0 12px 0">&#8220;%s&#8221;</p>`+
			`<footer style="font-style:normal;font-weight:600;font-size:13px;color:%s">&mdash; %s</footer>`+
			`</blockquote>`,
		box.W, tokens.AxisColor, tokens.GridColor, tokens.FontSans, tokens.TextColor,
		safeQuote, tokens.AxisColor, attribution,
	), nil
}

// statCardRenderer renders the first row as a dashboard-style
// single-metric tile: a label, a large value, and an optional
// delta/trend indicator. Recognized fields: "label", "value",
// "delta" (optional numeric percent — sign determines up/down color).
type statCardRenderer struct{}

func (statCardRenderer) RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	if len(rows) == 0 {
		return "", nil
	}
	label, _ := rows[0]["label"].(string)
	safeLabel := html.EscapeString(label)
	safeValue := html.EscapeString(formatStatValue(rows[0]["value"]))

	var deltaHTML string
	if raw, ok := rows[0]["delta"]; ok {
		if delta, ok := toFloat(raw); ok {
			arrow, color := "▲", "#16a34a" // up-triangle, green
			if delta < 0 {
				arrow, color = "▼", "#dc2626" // down-triangle, red
			}
			// %.1f%% is our own formatting, not row data, so it needs
			// no escaping — but we still route it through
			// EscapeString for uniformity/defense-in-depth since the
			// sign/magnitude ultimately derives from row data.
			deltaHTML = fmt.Sprintf(
				`<div style="margin-top:6px;font-size:13px;font-weight:600;color:%s">%s %s</div>`,
				color, arrow, html.EscapeString(fmt.Sprintf("%.1f%%", delta)),
			)
		}
	}

	return fmt.Sprintf(
		`<div style="max-width:%.0fpx;padding:16px 20px;border:1px solid %s;border-radius:8px;`+
			`font-family:%s">`+
			`<div style="font-size:12px;text-transform:uppercase;letter-spacing:.05em;color:%s">%s</div>`+
			`<div style="margin-top:4px;font-size:28px;font-weight:700;color:%s">%s</div>%s`+
			`</div>`,
		box.W, tokens.GridColor, tokens.FontSans,
		tokens.AxisColor, safeLabel, tokens.TextColor, safeValue, deltaHTML,
	), nil
}

// formatStatValue renders a JSON-decoded value (string, float64, or
// int) as display text for the stat card's headline number. Integer
// values get thousands separators for dashboard-tile readability;
// non-integers and strings pass through unadorned.
func formatStatValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == math.Trunc(t) {
			return withThousands(strconv.FormatFloat(t, 'f', 0, 64))
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return withThousands(strconv.Itoa(t))
	default:
		return fmt.Sprintf("%v", t)
	}
}

// withThousands inserts ',' thousands separators into a base-10
// integer string (sign-aware). Written by hand rather than pulled in
// via golang.org/x/text/message to avoid adding a dependency for one
// cookbook example.
func withThousands(s string) string {
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	n := len(s)
	if n <= 3 {
		if neg {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	first := n % 3
	if first == 0 {
		first = 3
	}
	b.WriteString(s[:first])
	for i := first; i < n; i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	out := b.String()
	if neg {
		return "-" + out
	}
	return out
}

// toFloat extracts a numeric delta from a JSON-decoded value.
func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	default:
		return 0, false
	}
}

// quoteCardFixtureSpec / statCardFixtureSpec build the specs used both
// by the escaping proofs below and by the gallery fixture
// regeneration test — a realistic single-row payload for each card.

func quoteCardFixtureSpec(rendererName string) []byte {
	return []byte(fmt.Sprintf(`{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{
    "quote": "Design is not just what it looks like and feels like. Design is how it works.",
    "author": "Steve Jobs",
    "role": "Co-founder, Apple & NeXT"
  }]},
  "mark": {"type": "custom", "renderer": %q},
  "encoding": {}
}`, rendererName))
}

func statCardFixtureSpec(rendererName string) []byte {
	return []byte(fmt.Sprintf(`{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{
    "label": "Monthly Active Users",
    "value": 128400,
    "delta": 4.2
  }]},
  "mark": {"type": "custom", "renderer": %q},
  "encoding": {}
}`, rendererName))
}

// TestQuoteCardRendererEscapesRowData proves the quote card escapes
// HTML-special characters (<, &, ") found in row data — the security
// contract every custom-mark renderer must uphold per the cookbook.
func TestQuoteCardRendererEscapesRowData(t *testing.T) {
	const name = "test-quote-card-escape"
	custommark.ResetForTest(t)
	if err := custommark.Register(name, quoteCardRenderer{}); err != nil {
		t.Fatalf("custommark.Register: %v", err)
	}

	spec := []byte(fmt.Sprintf(`{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{
    "quote": "<script>alert(1)</script> & \"quoted\"",
    "author": "A & B <Corp>",
    "role": "R&D \"lead\""
  }]},
  "mark": {"type": "custom", "renderer": %q},
  "encoding": {}
}`, name))

	got, err := renderCustomFixture(t, spec)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Fatalf("quote card did not escape a raw <script> tag from row data:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("expected escaped script tag in quote text, got:\n%s", out)
	}
	if !strings.Contains(out, "A &amp; B &lt;Corp&gt;") {
		t.Errorf("expected escaped ampersand/angle-brackets in author, got:\n%s", out)
	}
	if !strings.Contains(out, "&#34;quoted&#34;") {
		t.Errorf("expected escaped double-quote in quote text, got:\n%s", out)
	}
	if !strings.Contains(out, "R&amp;D &#34;lead&#34;") {
		t.Errorf("expected escaped ampersand/double-quote in role, got:\n%s", out)
	}
}

// TestStatCardRendererEscapesRowData proves the stat card escapes
// HTML-special characters (<, &, ") found in row data, including when
// they appear in a string-typed "value" field.
func TestStatCardRendererEscapesRowData(t *testing.T) {
	const name = "test-stat-card-escape"
	custommark.ResetForTest(t)
	if err := custommark.Register(name, statCardRenderer{}); err != nil {
		t.Fatalf("custommark.Register: %v", err)
	}

	spec := []byte(fmt.Sprintf(`{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"values": [{
    "label": "<b>Signups</b> & \"Trials\"",
    "value": "42 <units>",
    "delta": -3.5
  }]},
  "mark": {"type": "custom", "renderer": %q},
  "encoding": {}
}`, name))

	got, err := renderCustomFixture(t, spec)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	out := string(got)

	if strings.Contains(out, "<b>Signups</b>") {
		t.Fatalf("stat card did not escape raw markup in the label field:\n%s", out)
	}
	if !strings.Contains(out, "&lt;b&gt;Signups&lt;/b&gt; &amp; &#34;Trials&#34;") {
		t.Errorf("expected escaped label, got:\n%s", out)
	}
	if strings.Contains(out, "42 <units>") {
		t.Fatalf("stat card did not escape raw markup in the value field:\n%s", out)
	}
	if !strings.Contains(out, "42 &lt;units&gt;") {
		t.Errorf("expected escaped value, got:\n%s", out)
	}
	if !strings.Contains(out, "▼") || !strings.Contains(out, "#dc2626") {
		t.Errorf("expected a negative (down/red) delta indicator for delta=-3.5, got:\n%s", out)
	}
}

// TestGalleryCustomMarkCardsGoldensStable regenerates the two
// committed gallery fixtures for E5-S1 (quote card, stat card) under
// docs/src/gallery/custom-marks/. Unlike every other gallery category,
// these have no compilable *.prism.json companion: the shared `prism`
// CLI binary has no renderer registered under any name (registration
// is a Go-level call, `custommark.Register`, that only this
// in-process pipeline can make), so
// cmd/prism/gallery_test.go's *.prism.json walk cannot — and does not
// — pick these up. This test is the regeneration mechanism instead;
// its non-update path also acts as a golden/drift check on every
// `go test ./render/html/...` run. See docs/src/cookbook/custom-marks.md
// for the full worked examples and docs/src/gallery/index.md's
// "Custom marks" section for why no *.prism.json ships alongside these.
//
// Run `UPDATE_GOLDENS=1 go test ./render/html/... -run
// TestGalleryCustomMarkCardsGoldensStable` to regenerate.
func TestGalleryCustomMarkCardsGoldensStable(t *testing.T) {
	update := os.Getenv("UPDATE_GOLDENS") == "1"
	cases := []struct {
		name     string
		renderer string
		impl     custommark.CustomRenderer
		specFn   func(string) []byte
	}{
		{name: "quote_card", renderer: "gallery-quote-card", impl: quoteCardRenderer{}, specFn: quoteCardFixtureSpec},
		{name: "stat_card", renderer: "gallery-stat-card", impl: statCardRenderer{}, specFn: statCardFixtureSpec},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			custommark.ResetForTest(t)
			if err := custommark.Register(tc.renderer, tc.impl); err != nil {
				t.Fatalf("custommark.Register: %v", err)
			}

			got, err := renderCustomFixture(t, tc.specFn(tc.renderer))
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			galleryPath := filepath.Join(repoRoot(t), "docs", "src", "gallery", "custom-marks", tc.name+".html")
			if update {
				if err := os.MkdirAll(filepath.Dir(galleryPath), 0o755); err != nil {
					t.Fatalf("mkdir gallery dir: %v", err)
				}
				if err := os.WriteFile(galleryPath, got, 0o644); err != nil {
					t.Fatalf("write gallery fixture %s: %v", galleryPath, err)
				}
				t.Logf("wrote gallery fixture %s (%d bytes)", galleryPath, len(got))
				return
			}
			want, err := os.ReadFile(galleryPath)
			if err != nil {
				t.Fatalf("read gallery fixture (%s): %v.\nRun `UPDATE_GOLDENS=1 go test ./render/html/... -run TestGalleryCustomMarkCardsGoldensStable` to create.", galleryPath, err)
			}
			if !bytes.Equal(want, got) {
				t.Errorf("gallery fixture does not match committed %s.\n--- committed ---\n%s\n--- got ---\n%s",
					galleryPath, truncate(want, 1200), truncate(got, 1200))
			}
		})
	}
}
