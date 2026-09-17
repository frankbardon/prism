package spec

import "testing"

// offsetSpec wraps an encoding body in a minimal bar spec, mirroring
// stackSpec in stack_test.go.
func offsetSpec(t *testing.T, encoding string) *Spec {
	t.Helper()
	return stackSpec(t, `"bar"`, encoding)
}

// TestPrismResolveOffsetUnbound pins the no-op: every spec that
// predates the offset channel resolves to nil, which is what keeps
// the planner and the encoder byte-identical for it.
func TestPrismResolveOffsetUnbound(t *testing.T) {
	if got := ResolveOffset(nil); got != nil {
		t.Fatalf("ResolveOffset(nil) = %#v, want nil", got)
	}
	s := offsetSpec(t, `{
	  "x": {"field": "g", "type": "nominal"},
	  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"}
	}`)
	if got := ResolveOffset(s.Encoding); got != nil {
		t.Fatalf("ResolveOffset = %#v, want nil for an encoding with no offset", got)
	}
}

// TestPrismResolveOffsetAxes covers both spellings of the channel.
func TestPrismResolveOffsetAxes(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"x_offset", "x"},
		{"y_offset", "y"},
	}
	for _, tc := range cases {
		s := offsetSpec(t, `{
		  "x": {"field": "g", "type": "nominal"},
		  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
		  "`+tc.key+`": {"field": "c", "type": "nominal"}
		}`)
		off := ResolveOffset(s.Encoding)
		if off == nil {
			t.Fatalf("%s: ResolveOffset = nil, want a binding", tc.key)
		}
		if off.Channel != tc.want {
			t.Errorf("%s: Channel = %q, want %q", tc.key, off.Channel, tc.want)
		}
		if off.Field != "c" {
			t.Errorf("%s: Field = %q, want \"c\"", tc.key, off.Field)
		}
		if off.Offset == nil || off.Offset.Type != "nominal" {
			t.Errorf("%s: Offset = %#v, want the source channel block", tc.key, off.Offset)
		}
	}
}

// TestPrismResolveOffsetNoField keeps an offset object that names no
// column from binding: there is nothing to subdivide the band by, so
// it must read as unbound rather than as a zero-field binding.
func TestPrismResolveOffsetNoField(t *testing.T) {
	s := offsetSpec(t, `{
	  "x": {"field": "g", "type": "nominal"},
	  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
	  "x_offset": {"type": "nominal"}
	}`)
	if got := ResolveOffset(s.Encoding); got != nil {
		t.Fatalf("ResolveOffset = %#v, want nil for a fieldless offset", got)
	}
}

// TestPrismResolveOffsetBothAxes pins the documented answer to the
// incoherent spec: a mark cannot dodge along two axes, so the
// resolver refuses rather than picking a winner. E2-S2 rejects the
// spec outright with PRISM_SPEC_064; until then the requirement here
// is only that the resolver stays total.
func TestPrismResolveOffsetBothAxes(t *testing.T) {
	s := offsetSpec(t, `{
	  "x": {"field": "g", "type": "nominal"},
	  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
	  "x_offset": {"field": "c", "type": "nominal"},
	  "y_offset": {"field": "c", "type": "nominal"}
	}`)
	if got := ResolveOffset(s.Encoding); got != nil {
		t.Fatalf("ResolveOffset = %#v, want nil when both axes bind an offset", got)
	}
}

// TestPrismResolveStackYieldsToOffset is the half of E1-S2 that stops
// a grouped bar from also stacking. The two encodings differ by one
// key, so the test also pins that removing the offset restores the
// implicit binding unchanged.
func TestPrismResolveStackYieldsToOffset(t *testing.T) {
	for _, key := range []string{"x_offset", "y_offset"} {
		withOffset := offsetSpec(t, `{
		  "x": {"field": "g", "type": "nominal"},
		  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
		  "color": {"field": "c", "type": "nominal"},
		  "`+key+`": {"field": "c", "type": "nominal"}
		}`)
		if st := ResolveStack(withOffset); st != nil {
			t.Errorf("%s: ResolveStack = %#v, want nil — offset supersedes the implicit stack", key, st)
		}
	}

	without := offsetSpec(t, `{
	  "x": {"field": "g", "type": "nominal"},
	  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
	  "color": {"field": "c", "type": "nominal"}
	}`)
	st := ResolveStack(without)
	if st == nil {
		t.Fatalf("ResolveStack = nil without an offset, want the implicit binding")
	}
	if !st.Implicit || st.Channel != "y" || st.Field != "v" || st.Offset != StackOffsetZero {
		t.Errorf("implicit binding changed: %#v", st)
	}
}

// TestPrismResolveStackExplicitSurvivesOffset guards the boundary the
// story draws: an explicit `stack` alongside a bound offset is a
// contradiction validate rejects with PRISM_SPEC_065, so ResolveStack
// must still hand back a binding. Swallowing it here would turn a
// reported error into a silent behaviour change.
func TestPrismResolveStackExplicitSurvivesOffset(t *testing.T) {
	for _, key := range []string{"x_offset", "y_offset"} {
		s := offsetSpec(t, `{
		  "x": {"field": "g", "type": "nominal"},
		  "y": {"aggregate": "sum", "field": "v", "type": "quantitative", "stack": "normalize"},
		  "color": {"field": "c", "type": "nominal"},
		  "`+key+`": {"field": "c", "type": "nominal"}
		}`)
		st := ResolveStack(s)
		if st == nil {
			t.Fatalf("%s: ResolveStack = nil, want the explicit binding preserved", key)
		}
		if st.Implicit {
			t.Errorf("%s: Implicit = true, want false for an explicit stack", key)
		}
		if st.Channel != "y" || st.Offset != StackOffsetNormalize {
			t.Errorf("%s: got channel=%q offset=%q, want y/normalize", key, st.Channel, st.Offset)
		}
	}
}

// TestPrismResolveStackOffsetOnNonStackingShape checks the
// suppression is scoped to the implicit path and does not reach specs
// that were never going to stack: without a grouping channel there is
// no implicit binding to suppress, with or without an offset.
func TestPrismResolveStackOffsetOnNonStackingShape(t *testing.T) {
	s := offsetSpec(t, `{
	  "x": {"field": "g", "type": "nominal"},
	  "y": {"aggregate": "sum", "field": "v", "type": "quantitative"},
	  "x_offset": {"field": "c", "type": "nominal"}
	}`)
	if st := ResolveStack(s); st != nil {
		t.Fatalf("ResolveStack = %#v, want nil with no grouping channel bound", st)
	}
}
