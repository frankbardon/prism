package spec

import "testing"

// TestPrismSpecRepeatRefDecode pins the polymorphic `field` decoding
// covering both the bare-string form and the {"repeat": <axis>}
// substitution form. The substitution walker
// (plan/build/composite.go) rewrites FieldRef into Field per cell;
// the spec layer just hands both shapes back to the caller.
func TestPrismSpecRepeatRefDecode(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantField   string
		wantRefAxis string
	}{
		{
			name:      "bare string field",
			body:      `{"$schema": "urn:prism:schema:v1:spec", "mark": "bar", "encoding": {"y": {"field": "score", "type": "quantitative"}}}`,
			wantField: "score",
		},
		{
			name:        "repeat row substitution",
			body:        `{"$schema": "urn:prism:schema:v1:spec", "mark": "bar", "encoding": {"y": {"field": {"repeat": "row"}, "type": "quantitative"}}}`,
			wantRefAxis: "row",
		},
		{
			name:        "repeat column substitution",
			body:        `{"$schema": "urn:prism:schema:v1:spec", "mark": "bar", "encoding": {"y": {"field": {"repeat": "column"}, "type": "quantitative"}}}`,
			wantRefAxis: "column",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s, err := DecodeBytes([]byte(tc.body))
			if err != nil {
				t.Fatalf("DecodeBytes: %v", err)
			}
			if s.Encoding == nil || s.Encoding.Y == nil {
				t.Fatal("missing y encoding")
			}
			y := s.Encoding.Y
			if y.Field != tc.wantField {
				t.Errorf("Field = %q, want %q", y.Field, tc.wantField)
			}
			if tc.wantRefAxis == "" {
				if y.FieldRef != nil {
					t.Errorf("FieldRef = %+v, want nil", y.FieldRef)
				}
				return
			}
			if y.FieldRef == nil {
				t.Fatalf("FieldRef = nil, want axis %q", tc.wantRefAxis)
			}
			if y.FieldRef.Axis != tc.wantRefAxis {
				t.Errorf("FieldRef.Axis = %q, want %q", y.FieldRef.Axis, tc.wantRefAxis)
			}
		})
	}
}

// TestPrismSpecRepeatRefDecodeOnColorChannel pins MarkChannel
// substitution support so a repeated color field works too.
func TestPrismSpecRepeatRefDecodeOnColorChannel(t *testing.T) {
	body := `{
		"$schema": "urn:prism:schema:v1:spec",
		"mark": "point",
		"encoding": {
			"x":     {"field": "day",   "type": "temporal"},
			"y":     {"field": "score", "type": "quantitative"},
			"color": {"field": {"repeat": "row"}, "type": "nominal"}
		}
	}`
	s, err := DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	if s.Encoding == nil || s.Encoding.Color == nil {
		t.Fatal("missing color encoding")
	}
	c := s.Encoding.Color
	if c.Field != "" {
		t.Errorf("Field = %q, want empty (substitution form)", c.Field)
	}
	if c.FieldRef == nil || c.FieldRef.Axis != "row" {
		t.Errorf("FieldRef = %+v, want {Axis:row}", c.FieldRef)
	}
}
