package limits

import (
	"testing"
)

func TestTableMaxRowsDefault(t *testing.T) {
	t.Setenv(EnvTableMaxRows, "")
	v, ok := TableMaxRows()
	if !ok {
		t.Fatalf("expected ok=true for unset env, got ok=false")
	}
	if v != DefaultTableMaxRows {
		t.Fatalf("expected default %d, got %d", DefaultTableMaxRows, v)
	}
}

func TestTableMaxRowsOverride(t *testing.T) {
	t.Setenv(EnvTableMaxRows, "1234")
	v, ok := TableMaxRows()
	if !ok {
		t.Fatalf("expected ok=true for valid override, got ok=false")
	}
	if v != 1234 {
		t.Fatalf("expected 1234, got %d", v)
	}
}

func TestTableMaxRowsMalformedFallsBackWithOkFalse(t *testing.T) {
	t.Setenv(EnvTableMaxRows, "not-a-number")
	v, ok := TableMaxRows()
	if ok {
		t.Fatalf("expected ok=false for malformed env, got ok=true")
	}
	if v != DefaultTableMaxRows {
		t.Fatalf("expected default %d on malformed, got %d", DefaultTableMaxRows, v)
	}
}

func TestTableMaxRowsZeroOrNegativeRejected(t *testing.T) {
	for _, s := range []string{"0", "-1", "-1000"} {
		t.Run(s, func(t *testing.T) {
			t.Setenv(EnvTableMaxRows, s)
			v, ok := TableMaxRows()
			if ok {
				t.Fatalf("expected ok=false for %q, got ok=true", s)
			}
			if v != DefaultTableMaxRows {
				t.Fatalf("expected default %d on %q, got %d", DefaultTableMaxRows, s, v)
			}
		})
	}
}

func TestJoinAndRenderHelpersRoundTrip(t *testing.T) {
	t.Setenv(EnvJoinMaxRows, "42")
	if v, ok := JoinMaxRows(); !ok || v != 42 {
		t.Fatalf("JoinMaxRows: got (%d, %v); want (42, true)", v, ok)
	}
	if v := MustJoinMaxRows(); v != 42 {
		t.Fatalf("MustJoinMaxRows: got %d; want 42", v)
	}

	t.Setenv(EnvRenderMaxMarks, "99")
	if v, ok := RenderMaxMarks(); !ok || v != 99 {
		t.Fatalf("RenderMaxMarks: got (%d, %v); want (99, true)", v, ok)
	}
	if v := MustRenderMaxMarks(); v != 99 {
		t.Fatalf("MustRenderMaxMarks: got %d; want 99", v)
	}
}

func TestMustTableMaxRowsTolerates(t *testing.T) {
	t.Setenv(EnvTableMaxRows, "garbage")
	if v := MustTableMaxRows(); v != DefaultTableMaxRows {
		t.Fatalf("MustTableMaxRows: got %d; want default %d", v, DefaultTableMaxRows)
	}
}
