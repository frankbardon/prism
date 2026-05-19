// Package svg implements render.Renderer for SVG output. The
// renderer walks a SceneDoc top-down, emitting per-mark elements
// (rect, line, area=path, circle, line=rule) inside the canonical
// <svg> skeleton from design/07-rendering.md.
//
// Output goes through encoding/xml for attribute escaping;
// coordinates pin to RenderPrecision (3 decimals) via the writer's
// FormatFloat helper. Goldens compare byte-equal against committed
// testdata/svgs/*.svg files.
package svg

import (
	"bytes"
	"encoding/xml"
	"strings"

	"github.com/frankbardon/prism/render"
)

// Writer wraps bytes.Buffer with precision-pinned float formatting
// + escaped attribute writing. Used by every per-mark and per-axis
// helper so output stays byte-stable across Go versions.
//
// We don't use xml.Encoder directly because its Token-based API is
// inconvenient for the heavily-nested SVG structure we emit; we
// reach for the underlying buf and use xml.EscapeText / our own
// FormatFloat to handle the safety bits.
type Writer struct {
	buf *bytes.Buffer
}

// NewWriter returns a Writer backed by an internal bytes.Buffer.
// Call Bytes() at the end to get the final output.
func NewWriter() *Writer {
	return &Writer{buf: &bytes.Buffer{}}
}

// Bytes returns the accumulated output.
func (w *Writer) Bytes() []byte { return w.buf.Bytes() }

// String returns the accumulated output as a string.
func (w *Writer) String() string { return w.buf.String() }

// Raw writes raw bytes (no escaping). Use for control characters
// like `<`, `>`, `/` and structural skeletons.
func (w *Writer) Raw(s string) { w.buf.WriteString(s) }

// Text writes XML-escaped text content (anything between tags).
func (w *Writer) Text(s string) {
	_ = xml.EscapeText(w.buf, []byte(s))
}

// Attr writes ` name="value"` with the value XML-escaped for
// attribute context (handles quotes, ampersands, etc.).
func (w *Writer) Attr(name, value string) {
	w.buf.WriteByte(' ')
	w.buf.WriteString(name)
	w.buf.WriteString(`="`)
	escapeAttr(w.buf, value)
	w.buf.WriteByte('"')
}

// AttrFloat writes ` name="3.142"` with the value pinned to
// RenderPrecision via FormatFloat.
func (w *Writer) AttrFloat(name string, v float64) {
	w.buf.WriteByte(' ')
	w.buf.WriteString(name)
	w.buf.WriteString(`="`)
	w.buf.WriteString(render.FormatFloat(v))
	w.buf.WriteByte('"')
}

// OpenTag writes `<name`. Caller follows with Attr / AttrFloat then
// CloseTag or SelfClose.
func (w *Writer) OpenTag(name string) {
	w.buf.WriteByte('<')
	w.buf.WriteString(name)
}

// OpenAttr writes ` name="`. Caller follows with Raw calls (which
// must be pre-escaped) and a final CloseAttr. Used for SVG path d=
// and polyline points= attributes where building the value via
// repeated Attr calls would require an intermediate string buffer.
func (w *Writer) OpenAttr(name string) {
	w.buf.WriteByte(' ')
	w.buf.WriteString(name)
	w.buf.WriteString(`="`)
}

// CloseAttr writes `"` to terminate the OpenAttr value.
func (w *Writer) CloseAttr() {
	w.buf.WriteByte('"')
}

// CloseTagOpen writes `>` (closes the opening tag; children follow).
func (w *Writer) CloseTagOpen() {
	w.buf.WriteByte('>')
}

// SelfClose writes ` />` to end a void element.
func (w *Writer) SelfClose() {
	w.buf.WriteString("/>")
}

// EndTag writes `</name>`.
func (w *Writer) EndTag(name string) {
	w.buf.WriteString("</")
	w.buf.WriteString(name)
	w.buf.WriteByte('>')
}

// Newline writes a single '\n'. Used between top-level sections for
// human-readable goldens.
func (w *Writer) Newline() { w.buf.WriteByte('\n') }

// Indent writes n spaces. Used for indenting nested elements in
// goldens.
func (w *Writer) Indent(n int) {
	for i := 0; i < n; i++ {
		w.buf.WriteByte(' ')
	}
}

// escapeAttr writes value with attribute-context XML escaping
// (handles &, <, >, ", '). encoding/xml.EscapeText escapes for
// element content; this variant escapes the additional characters
// attributes require.
func escapeAttr(buf *bytes.Buffer, value string) {
	for _, r := range value {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '"':
			buf.WriteString("&quot;")
		case '\'':
			buf.WriteString("&#39;")
		default:
			buf.WriteRune(r)
		}
	}
}

// JoinAttr writes a space-separated list inside an attribute value.
// Convenience for `class="a b c"` and `stroke-dasharray="5 5"`.
func (w *Writer) JoinAttr(name string, parts []string) {
	w.Attr(name, strings.Join(parts, " "))
}
