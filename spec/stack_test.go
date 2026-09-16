package spec

import (
	"encoding/json"
	"testing"
)

// decodeStackSpec decodes an inline spec body for the resolver tests.
func decodeStackSpec(t *testing.T, body string) *Spec {
	t.Helper()
	s, err := DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return s
}

// stackSpec wraps an encoding body in a minimal bar spec.
func stackSpec(t *testing.T, mark, encoding string) *Spec {
	t.Helper()
	return decodeStackSpec(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"g": "a", "c": "x", "v": 1}]},
	  "mark": `+mark+`,
	  "encoding": `+encoding+`
	}`)
}

// TestPrismResolveStackImplicit pins the Vega-Lite default: a bar or
// area whose measure channel is aggregated and whose marks are split
// by a grouping channel stacks without the spec saying so.
func TestPrismResolveStackImplicit(t *testing.T) {
	for _, mark := range []string{`"bar"`, `"area"`} {
		s := stackSpec(t, mark, `{
		  "x": {"field": "g", "type": "nominal"},
		  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
		  "color": {"field": "c", "type": "nominal"}
		}`)
		st := ResolveStack(s)
		if st == nil {
			t.Fatalf("mark %s: ResolveStack = nil, want an implicit binding", mark)
		}
		if !st.Implicit {
			t.Errorf("mark %s: Implicit = false, want true", mark)
		}
		if st.Channel != "y" || st.Field != "v" || st.Offset != StackOffsetZero {
			t.Errorf("mark %s: got channel=%q field=%q offset=%q", mark, st.Channel, st.Field, st.Offset)
		}
		if len(st.Groupby) != 1 || st.Groupby[0] != "g" {
			t.Errorf("mark %s: Groupby = %v, want [g]", mark, st.Groupby)
		}
		if len(st.StackBy) != 1 || st.StackBy[0] != "c" {
			t.Errorf("mark %s: StackBy = %v, want [c]", mark, st.StackBy)
		}
		if st.StartAs != "v_start" || st.EndAs != "v_end" {
			t.Errorf("mark %s: as = %q/%q, want v_start/v_end", mark, st.StartAs, st.EndAs)
		}
	}
}

// TestPrismResolveStackDetailGroups pins that `detail` counts as a
// grouping channel for the implicit default, and that StackBy walks
// colour first then detail — the order encode/marks/group.go uses.
func TestPrismResolveStackDetailGroups(t *testing.T) {
	s := stackSpec(t, `"area"`, `{
	  "x": {"field": "g", "type": "nominal"},
	  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
	  "color": {"field": "c", "type": "nominal"},
	  "detail": {"field": "d", "type": "nominal"}
	}`)
	st := ResolveStack(s)
	if st == nil {
		t.Fatal("ResolveStack = nil, want a binding")
	}
	if len(st.StackBy) != 2 || st.StackBy[0] != "c" || st.StackBy[1] != "d" {
		t.Errorf("StackBy = %v, want [c d]", st.StackBy)
	}

	// Detail alone is enough — no colour bound.
	s2 := stackSpec(t, `"area"`, `{
	  "x": {"field": "g", "type": "nominal"},
	  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
	  "detail": {"field": "d", "type": "nominal"}
	}`)
	if st2 := ResolveStack(s2); st2 == nil {
		t.Error("detail-only spec: ResolveStack = nil, want a binding")
	}
}

// TestPrismResolveStackNotImplicit walks every condition that must
// leave a spec unstacked, so no pre-E5-S2 output moves by accident.
func TestPrismResolveStackNotImplicit(t *testing.T) {
	cases := []struct {
		name string
		mark string
		enc  string
	}{
		{"no grouping channel", `"bar"`, `{
		  "x": {"field": "g", "type": "nominal"},
		  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"}}`},
		{"no aggregate", `"bar"`, `{
		  "x": {"field": "g", "type": "nominal"},
		  "y": {"field": "v", "type": "quantitative"},
		  "color": {"field": "c", "type": "nominal"}}`},
		{"both axes aggregated", `"bar"`, `{
		  "x": {"aggregate": "mean", "field": "g", "type": "quantitative"},
		  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
		  "color": {"field": "c", "type": "nominal"}}`},
		{"unstackable mark", `"line"`, `{
		  "x": {"field": "g", "type": "nominal"},
		  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
		  "color": {"field": "c", "type": "nominal"}}`},
		{"span bound", `"bar"`, `{
		  "x": {"field": "g", "type": "nominal"},
		  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
		  "y2": {"field": "v2", "type": "quantitative"},
		  "color": {"field": "c", "type": "nominal"}}`},
		{"log measure scale", `"bar"`, `{
		  "x": {"field": "g", "type": "nominal"},
		  "y": {"aggregate": "sum", "field": "v", "type": "quantitative", "scale": {"type": "log"}},
		  "color": {"field": "c", "type": "nominal"}}`},
		{"other channel unbound", `"bar"`, `{
		  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
		  "color": {"field": "c", "type": "nominal"}}`},
		{"stack on both axes", `"bar"`, `{
		  "x": {"field": "g", "type": "quantitative", "stack": "zero"},
		  "y": {"field": "v", "type": "quantitative", "stack": "zero"},
		  "color": {"field": "c", "type": "nominal"}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if st := ResolveStack(stackSpec(t, c.mark, c.enc)); st != nil {
				t.Errorf("ResolveStack = %+v, want nil", st)
			}
		})
	}
}

// TestPrismResolveStackExplicitDisable pins the three ways an author
// turns stacking off, and that `true` is an alias for "zero".
func TestPrismResolveStackExplicitDisable(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string // "" means no stacking
	}{
		{"null", `null`, ""},
		{"false", `false`, ""},
		{"true", `true`, StackOffsetZero},
		{"zero", `"zero"`, StackOffsetZero},
		{"normalize", `"normalize"`, StackOffsetNormalize},
		{"center reserved", `"center"`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := stackSpec(t, `"bar"`, `{
			  "x": {"field": "g", "type": "nominal"},
			  "y": {"aggregate": "sum", "field": "v", "type": "quantitative", "stack": `+c.raw+`},
			  "color": {"field": "c", "type": "nominal"}
			}`)
			st := ResolveStack(s)
			if c.want == "" {
				if st != nil {
					t.Fatalf("ResolveStack = %+v, want nil", st)
				}
				return
			}
			if st == nil {
				t.Fatalf("ResolveStack = nil, want offset %q", c.want)
			}
			if st.Offset != c.want {
				t.Errorf("Offset = %q, want %q", st.Offset, c.want)
			}
			if st.Implicit {
				t.Error("Implicit = true, want false for an explicit stack")
			}
		})
	}
}

// TestPrismPositionChannelStackRoundTrip pins the tri-state decode: an
// absent key, an explicit null and an explicit false are three distinct
// states, and the null survives a marshal round-trip.
func TestPrismPositionChannelStackRoundTrip(t *testing.T) {
	var absent PositionChannel
	if err := json.Unmarshal([]byte(`{"field": "v", "type": "quantitative"}`), &absent); err != nil {
		t.Fatalf("unmarshal absent: %v", err)
	}
	if absent.Stack != nil || absent.StackNull {
		t.Errorf("absent: Stack=%v StackNull=%v, want nil/false", absent.Stack, absent.StackNull)
	}

	var null PositionChannel
	if err := json.Unmarshal([]byte(`{"field": "v", "stack": null}`), &null); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if null.Stack != nil || !null.StackNull {
		t.Errorf("null: Stack=%v StackNull=%v, want nil/true", null.Stack, null.StackNull)
	}
	out, err := json.Marshal(null)
	if err != nil {
		t.Fatalf("marshal null: %v", err)
	}
	var back PositionChannel
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if !back.StackNull {
		t.Errorf("round-trip dropped the explicit null: %s", out)
	}

	var value PositionChannel
	if err := json.Unmarshal([]byte(`{"field": "v", "stack": "normalize"}`), &value); err != nil {
		t.Fatalf("unmarshal value: %v", err)
	}
	if value.Stack != "normalize" {
		t.Errorf("value: Stack = %v, want \"normalize\"", value.Stack)
	}
}

// TestPrismPositionChannelStackNullKeepsAxis guards the encoding/json
// field-shadowing trap: the aux struct used to re-emit `"stack": null`
// must not suppress a configured `axis` block.
func TestPrismPositionChannelStackNullKeepsAxis(t *testing.T) {
	var ch PositionChannel
	if err := json.Unmarshal([]byte(`{"field": "v", "stack": null, "axis": {"title": "T"}}`), &ch); err != nil {
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
	if !back.StackNull {
		t.Errorf("round-trip dropped the explicit stack null: %s", out)
	}
	if back.Axis == nil || back.Axis.Title != "T" {
		t.Errorf("round-trip dropped the axis block: %s", out)
	}
}

// TestPrismStackTransformDecodes pins the explicit transform variant's
// discriminator registration.
func TestPrismStackTransformDecodes(t *testing.T) {
	s := decodeStackSpec(t, `{
	  "$schema": "urn:prism:schema:v1:spec",
	  "data": {"values": [{"g": "a", "v": 1}]},
	  "transform": [{"stack": "v", "groupby": ["g"], "offset": "normalize", "as": ["lo", "hi"]}],
	  "mark": "bar",
	  "encoding": {
	    "x": {"field": "g", "type": "nominal"},
	    "y": {"field": "lo", "type": "quantitative"},
	    "y2": {"field": "hi", "type": "quantitative"}
	  }
	}`)
	if len(s.Transform) != 1 || s.Transform[0].Stack == nil {
		t.Fatalf("transform did not decode to a stack variant: %+v", s.Transform)
	}
	st := s.Transform[0].Stack
	if st.Stack != "v" || st.Offset != "normalize" {
		t.Errorf("got stack=%q offset=%q", st.Stack, st.Offset)
	}
	if len(st.As) != 2 || st.As[0] != "lo" || st.As[1] != "hi" {
		t.Errorf("As = %v, want [lo hi]", st.As)
	}
	out, err := json.Marshal(s.Transform[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var round Transform
	if err := json.Unmarshal(out, &round); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if round.Stack == nil || round.Stack.Stack != "v" {
		t.Errorf("round-trip lost the stack variant: %s", out)
	}
}
