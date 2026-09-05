package spec

import (
	"encoding/json"
	"testing"
)

// TestCustomMarkDefRoundTrip asserts a custom mark_def carrying
// renderer decodes without loss and re-encodes to an equivalent
// shape. renderer is a plain string key resolved against the
// prism.RegisterCustomMark registry at encode/render time — the spec
// JSON never carries executable code.
func TestCustomMarkDefRoundTrip(t *testing.T) {
	const in = `{"type": "custom", "renderer": "my-viz"}`

	var m Mark
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Def == nil {
		t.Fatalf("expected mark def, got shorthand %q", m.Shorthand)
	}
	def := m.Def
	if def.Type != "custom" {
		t.Errorf("type = %q, want custom", def.Type)
	}
	if def.Renderer != "my-viz" {
		t.Errorf("renderer = %q, want my-viz", def.Renderer)
	}

	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var rt Mark
	if err := json.Unmarshal(out, &rt); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if rt.Def == nil || rt.Def.Renderer != "my-viz" {
		t.Fatalf("round-trip renderer lost or wrong: %+v", rt.Def)
	}
}

// TestCustomMarkDefRendererUnset asserts an unset renderer decodes as
// the empty string (no renderer registered/resolvable), matching the
// zero-value convention for the other plain-string MarkDef fields
// (Fill, Stroke, ...).
func TestCustomMarkDefRendererUnset(t *testing.T) {
	var m Mark
	if err := json.Unmarshal([]byte(`{"type": "custom"}`), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Def == nil {
		t.Fatalf("expected mark def, got shorthand %q", m.Shorthand)
	}
	if m.Def.Renderer != "" {
		t.Errorf("renderer = %q, want empty (unset)", m.Def.Renderer)
	}
}

// TestCustomMarkDefRejectsNonStringRenderer asserts decode rejects a
// non-string renderer value outright — renderer is always a plain
// string key, never executable code or a structured value, per
// Prism's no-expression-language invariant.
func TestCustomMarkDefRejectsNonStringRenderer(t *testing.T) {
	for _, in := range []string{
		`{"type": "custom", "renderer": 42}`,
		`{"type": "custom", "renderer": true}`,
		`{"type": "custom", "renderer": {"name": "my-viz"}}`,
		`{"type": "custom", "renderer": ["my-viz"]}`,
	} {
		var m Mark
		if err := json.Unmarshal([]byte(in), &m); err == nil {
			t.Errorf("Unmarshal(%s): expected error, got nil (def=%+v)", in, m.Def)
		}
	}
}
