package svg_test

import (
	"strings"
	"testing"
)

// TestPrismRenderEmitsLayerDataAttr asserts every rendered <g
// class="prism-layer"> carries data-prism-layer per D077. Drives the
// browser-side hit-test scope (D077).
func TestPrismRenderEmitsLayerDataAttr(t *testing.T) {
	got, err := renderFixture(t, "bar_basic.json")
	if err != nil {
		t.Fatalf("renderFixture: %v", err)
	}
	out := string(got)
	if !strings.Contains(out, `class="prism-layer"`) {
		t.Fatalf("missing prism-layer class:\n%s", truncate(got, 800))
	}
	// data-prism-layer should appear on the prism-layer group; check
	// the exact substring including the value.
	if !strings.Contains(out, `class="prism-layer" data-layer-id="layer-0" data-prism-layer="layer-0"`) {
		t.Errorf("expected data-prism-layer=\"layer-0\" on prism-layer group\ngot:\n%s", truncate(got, 800))
	}
}

// TestPrismRenderEmitsDatumRowAttr asserts every per-row mark in
// bar_basic carries data-prism-datum-row matching its row index.
func TestPrismRenderEmitsDatumRowAttr(t *testing.T) {
	got, err := renderFixture(t, "bar_basic.json")
	if err != nil {
		t.Fatalf("renderFixture: %v", err)
	}
	out := string(got)
	for i := 0; i < 3; i++ {
		marker := `data-prism-datum-row="` + itoa(i) + `"`
		if !strings.Contains(out, marker) {
			t.Errorf("missing %s in rendered SVG:\n%s", marker, truncate(got, 800))
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
