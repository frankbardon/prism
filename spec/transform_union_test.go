package spec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTransformDiscriminator(t *testing.T) {
	cases := map[string]struct {
		json   string
		check  func(*testing.T, *Transform)
		errSub string
	}{
		"filter": {
			json: `{"filter":"x > 0"}`,
			check: func(t *testing.T, tr *Transform) {
				if tr.Filter == nil || tr.Filter.Filter != "x > 0" {
					t.Errorf("filter not parsed: %+v", tr)
				}
			},
		},
		"aggregate": {
			json: `{"aggregate":[{"op":"mean","field":"score","as":"score_mean"}]}`,
			check: func(t *testing.T, tr *Transform) {
				if tr.Aggregate == nil || len(tr.Aggregate.Aggregate) != 1 {
					t.Errorf("aggregate not parsed: %+v", tr)
				}
			},
		},
		"missing-discriminator": {
			json:   `{"data":"x"}`,
			errSub: "missing discriminator",
		},
		"multiple-discriminators": {
			json:   `{"filter":"a","calculate":"b","as":"c"}`,
			errSub: "multiple discriminator",
		},
		"unknown-field-in-variant": {
			json:   `{"filter":"x","wat":1}`,
			errSub: `unknown field "wat"`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var tr Transform
			err := json.Unmarshal([]byte(tc.json), &tr)
			if tc.errSub != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("expected error containing %q, got %v", tc.errSub, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.check(t, &tr)
		})
	}
}

func TestMarkDiscriminator(t *testing.T) {
	t.Run("shorthand", func(t *testing.T) {
		var m Mark
		if err := json.Unmarshal([]byte(`"bar"`), &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if m.Shorthand != "bar" {
			t.Errorf("shorthand not set: %+v", m)
		}
		if m.Def != nil {
			t.Errorf("def should be nil for shorthand")
		}
	})
	t.Run("def", func(t *testing.T) {
		var m Mark
		if err := json.Unmarshal([]byte(`{"type":"line","stroke":"red"}`), &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if m.Def == nil || m.Def.Type != "line" || m.Def.Stroke != "red" {
			t.Errorf("def not parsed: %+v", m.Def)
		}
	})
	t.Run("unknown-field", func(t *testing.T) {
		var m Mark
		err := json.Unmarshal([]byte(`{"type":"bar","wat":1}`), &m)
		if err == nil || !strings.Contains(err.Error(), `unknown field "wat"`) {
			t.Fatalf("expected unknown-field error, got %v", err)
		}
	})
}
