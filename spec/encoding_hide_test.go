package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPrismPositionChannelAxisTriState pins the three wire states of
// the axis key: absent, configured, and explicitly null.
func TestPrismPositionChannelAxisTriState(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		wantAxis   bool
		wantHidden bool
	}{
		{"absent", `{"field":"a","type":"nominal"}`, false, false},
		{"object", `{"field":"a","type":"nominal","axis":{"grid":true}}`, true, false},
		{"null", `{"field":"a","type":"nominal","axis":null}`, false, true},
		{"null padded", `{"field":"a","type":"nominal","axis":  null }`, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ch PositionChannel
			if err := json.Unmarshal([]byte(tc.in), &ch); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := ch.Axis != nil; got != tc.wantAxis {
				t.Errorf("Axis populated = %v, want %v", got, tc.wantAxis)
			}
			if ch.AxisHidden != tc.wantHidden {
				t.Errorf("AxisHidden = %v, want %v", ch.AxisHidden, tc.wantHidden)
			}
			if ch.Field != "a" {
				t.Errorf("Field = %q, want %q (field interception must survive)", ch.Field, "a")
			}
		})
	}
}

// TestPrismMarkChannelLegendTriState is the legend counterpart.
func TestPrismMarkChannelLegendTriState(t *testing.T) {
	cases := []struct {
		name         string
		in           string
		wantLegend   bool
		wantHidden   bool
		wantOrientIs string
	}{
		{"absent", `{"field":"a","type":"nominal"}`, false, false, ""},
		{"object", `{"field":"a","type":"nominal","legend":{"orient":"left"}}`, true, false, "left"},
		{"null", `{"field":"a","type":"nominal","legend":null}`, false, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ch MarkChannel
			if err := json.Unmarshal([]byte(tc.in), &ch); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := ch.Legend != nil; got != tc.wantLegend {
				t.Errorf("Legend populated = %v, want %v", got, tc.wantLegend)
			}
			if ch.LegendHidden != tc.wantHidden {
				t.Errorf("LegendHidden = %v, want %v", ch.LegendHidden, tc.wantHidden)
			}
			if tc.wantOrientIs != "" && ch.Legend.Orient != tc.wantOrientIs {
				t.Errorf("Legend.Orient = %q, want %q", ch.Legend.Orient, tc.wantOrientIs)
			}
		})
	}
}

// TestPrismHiddenAxisLegendRoundTrip asserts that neither direction of
// the absent/null distinction collapses through a marshal → unmarshal
// cycle.
func TestPrismHiddenAxisLegendRoundTrip(t *testing.T) {
	t.Run("hidden stays null", func(t *testing.T) {
		var ch PositionChannel
		if err := json.Unmarshal([]byte(`{"field":"a","type":"nominal","axis":null}`), &ch); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		out, err := json.Marshal(ch)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(out), `"axis":null`) {
			t.Fatalf("marshal dropped the explicit null: %s", out)
		}
		var back PositionChannel
		if err := json.Unmarshal(out, &back); err != nil {
			t.Fatalf("re-unmarshal: %v", err)
		}
		if !back.AxisHidden {
			t.Error("AxisHidden lost on round-trip")
		}
	})

	t.Run("absent stays absent", func(t *testing.T) {
		var ch PositionChannel
		if err := json.Unmarshal([]byte(`{"field":"a","type":"nominal"}`), &ch); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		out, err := json.Marshal(ch)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(out), `"axis"`) {
			t.Fatalf("marshal invented an axis key: %s", out)
		}
	})

	t.Run("legend hidden stays null", func(t *testing.T) {
		var ch MarkChannel
		if err := json.Unmarshal([]byte(`{"field":"a","type":"nominal","legend":null}`), &ch); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		out, err := json.Marshal(ch)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(out), `"legend":null`) {
			t.Fatalf("marshal dropped the explicit null: %s", out)
		}
		var back MarkChannel
		if err := json.Unmarshal(out, &back); err != nil {
			t.Fatalf("re-unmarshal: %v", err)
		}
		if !back.LegendHidden {
			t.Error("LegendHidden lost on round-trip")
		}
	})

	t.Run("legend absent stays absent", func(t *testing.T) {
		var ch MarkChannel
		if err := json.Unmarshal([]byte(`{"field":"a","type":"nominal"}`), &ch); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		out, err := json.Marshal(ch)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(out), `"legend"`) {
			t.Fatalf("marshal invented a legend key: %s", out)
		}
	})

	t.Run("configured axis survives", func(t *testing.T) {
		var ch PositionChannel
		if err := json.Unmarshal([]byte(`{"field":"a","type":"nominal","axis":{"grid":true,"title":"T"}}`), &ch); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		out, err := json.Marshal(ch)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var back PositionChannel
		if err := json.Unmarshal(out, &back); err != nil {
			t.Fatalf("re-unmarshal: %v", err)
		}
		if back.Axis == nil || back.Axis.Grid == nil || !*back.Axis.Grid || back.AxisHidden {
			t.Fatalf("configured axis did not survive: %s (%+v)", out, back.Axis)
		}
	})
}

// TestPrismHiddenAxisThroughDecode drives the full strict spec decoder
// so the tri-state survives Decode's DisallowUnknownFields path.
func TestPrismHiddenAxisThroughDecode(t *testing.T) {
	const src = `{
	  "data": {"values": [{"a": "x", "b": 1}]},
	  "mark": {"type": "bar"},
	  "encoding": {
	    "x": {"field": "a", "type": "nominal", "axis": null},
	    "y": {"field": "b", "type": "quantitative"},
	    "color": {"field": "a", "type": "nominal", "legend": null}
	  }
	}`
	s, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !s.Encoding.X.AxisHidden {
		t.Error("x axis not marked hidden")
	}
	if s.Encoding.Y.AxisHidden {
		t.Error("y axis wrongly marked hidden")
	}
	if !s.Encoding.Color.LegendHidden {
		t.Error("color legend not marked hidden")
	}
}

// TestPrismChannelDecodeRejectsMalformedBlock keeps the axis / legend
// interception from swallowing a badly typed block.
func TestPrismChannelDecodeRejectsMalformedBlock(t *testing.T) {
	var ch PositionChannel
	if err := json.Unmarshal([]byte(`{"field":"a","axis":7}`), &ch); err == nil {
		t.Fatal("expected an error for a non-object, non-null axis")
	}
	var mc MarkChannel
	if err := json.Unmarshal([]byte(`{"field":"a","legend":7}`), &mc); err == nil {
		t.Fatal("expected an error for a non-object, non-null legend")
	}
}
