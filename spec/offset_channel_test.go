package spec_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/frankbardon/prism/spec"
)

// baseSpec wraps an encoding block in the smallest decodable document.
func baseSpec(encoding string) string {
	return `{"data": {"values": [{"a": 1}]}, "mark": {"type": "bar"}, "encoding": ` + encoding + `}`
}

func TestPrismOffsetChannelDecodes(t *testing.T) {
	doc := baseSpec(`{
		"x": {"field": "cat", "type": "nominal"},
		"y": {"field": "val", "type": "quantitative"},
		"x_offset": {"field": "series", "type": "nominal", "sort": "descending", "scale": {"padding_inner": 0.05}}
	}`)
	s, err := spec.Decode(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	off := s.Encoding.XOffset
	if off == nil {
		t.Fatal("encoding.x_offset decoded to nil")
	}
	if off.Field != "series" {
		t.Errorf("field = %q, want %q", off.Field, "series")
	}
	if off.Type != "nominal" {
		t.Errorf("type = %q, want %q", off.Type, "nominal")
	}
	if got, ok := off.Sort.(string); !ok || got != "descending" {
		t.Errorf("sort = %#v, want %q", off.Sort, "descending")
	}
	if off.Scale == nil {
		t.Fatal("scale decoded to nil")
	}
	if off.Scale.PaddingInner == nil || *off.Scale.PaddingInner != 0.05 {
		t.Errorf("scale.padding_inner = %#v, want 0.05", off.Scale.PaddingInner)
	}
	if s.Encoding.YOffset != nil {
		t.Error("y_offset populated by an x_offset-only document")
	}
}

func TestPrismOffsetChannelYVariant(t *testing.T) {
	doc := baseSpec(`{"y_offset": {"field": "series", "type": "nominal"}}`)
	s, err := spec.Decode(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Encoding.YOffset == nil || s.Encoding.YOffset.Field != "series" {
		t.Fatalf("y_offset = %#v, want field \"series\"", s.Encoding.YOffset)
	}
	if s.Encoding.XOffset != nil {
		t.Error("x_offset populated by a y_offset-only document")
	}
}

// TestPrismOffsetChannelRejectsUnknownKey pins the whole point of the
// custom decoder: Go does not propagate spec.Decode's
// DisallowUnknownFields into an UnmarshalJSON, so without
// strictUnmarshal an unknown key inside the channel object would be
// dropped in silence for any caller not running the JSON Schema shape
// stage.
func TestPrismOffsetChannelRejectsUnknownKey(t *testing.T) {
	for _, key := range []string{"x_offset", "y_offset"} {
		doc := baseSpec(`{"` + key + `": {"field": "series", "type": "nominal", "zzz_unknown": 1}}`)
		_, err := spec.Decode(strings.NewReader(doc))
		if err == nil {
			t.Fatalf("%s: an unknown key inside the channel object decoded silently", key)
		}
		if !strings.Contains(err.Error(), "zzz_unknown") {
			t.Errorf("%s: error %q does not name the offending key", key, err)
		}
	}
}

// TestPrismOffsetChannelRejectsWideKeys guards the narrowness that is
// the reason OffsetChannel is not a PositionChannel: a key x / y
// accepts must NOT decode here, or it would be inert on the wire with
// nothing able to detect it.
func TestPrismOffsetChannelRejectsWideKeys(t *testing.T) {
	for _, key := range []string{
		`"axis": {"title": "T"}`,
		`"stack": "zero"`,
		`"aggregate": "sum"`,
		`"title": "T"`,
		`"format": ",.0f"`,
		`"bin": true`,
		`"value": 3`,
		`"key": true`,
	} {
		doc := baseSpec(`{"x_offset": {"field": "series", "type": "nominal", ` + key + `}}`)
		if _, err := spec.Decode(strings.NewReader(doc)); err == nil {
			t.Errorf("x_offset accepted %s; OffsetChannel must stay narrow", key)
		}
	}
}

func TestPrismOffsetChannelRoundTrip(t *testing.T) {
	in := `{"field":"series","type":"nominal","sort":"descending","scale":{"padding_inner":0.05}}`
	var ch spec.OffsetChannel
	if err := json.Unmarshal([]byte(in), &ch); err != nil {
		t.Fatalf("decode: %v", err)
	}
	out, err := json.Marshal(ch)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got, want map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if err := json.Unmarshal([]byte(in), &want); err != nil {
		t.Fatalf("re-decode input: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("round-trip key set = %v, want %v", keysOf(got), keysOf(want))
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("round-trip dropped key %q", k)
		}
	}
}

// TestPrismOffsetChannelOmittedWhenUnbound keeps an unbound channel off
// the wire, so every pre-existing spec round-trips byte-identically.
func TestPrismOffsetChannelOmittedWhenUnbound(t *testing.T) {
	enc := spec.Encoding{X: &spec.PositionChannel{}}
	out, err := json.Marshal(enc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "offset") {
		t.Errorf("unbound offset channels reached the wire: %s", out)
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
