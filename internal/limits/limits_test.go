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

// TestQueryWorkersDefaultIsSentinel pins the contract that an unset env
// var yields the sentinel 0 (callers substitute NumCPU) rather than the
// hardcoded NumCPU itself. This keeps the limits package free of
// runtime/ knowledge.
func TestQueryWorkersDefaultIsSentinel(t *testing.T) {
	t.Setenv(EnvQueryWorkers, "")
	v, ok := QueryWorkers()
	if !ok {
		t.Fatalf("expected ok=true for unset env, got ok=false")
	}
	if v != DefaultQueryWorkers {
		t.Fatalf("expected sentinel %d, got %d", DefaultQueryWorkers, v)
	}
}

func TestQueryWorkersHonoursOverride(t *testing.T) {
	t.Setenv(EnvQueryWorkers, "4")
	v, ok := QueryWorkers()
	if !ok || v != 4 {
		t.Fatalf("got (%d, %v); want (4, true)", v, ok)
	}
}

func TestQueryWorkersRejectsNegativeButAllowsZero(t *testing.T) {
	t.Setenv(EnvQueryWorkers, "0")
	if v, ok := QueryWorkers(); !ok || v != 0 {
		t.Fatalf("zero rejected: got (%d, %v); want (0, true)", v, ok)
	}
	t.Setenv(EnvQueryWorkers, "-1")
	if v, ok := QueryWorkers(); ok || v != DefaultQueryWorkers {
		t.Fatalf("negative accepted: got (%d, %v); want (default, false)", v, ok)
	}
	t.Setenv(EnvQueryWorkers, "abc")
	if v, ok := QueryWorkers(); ok || v != DefaultQueryWorkers {
		t.Fatalf("garbage accepted: got (%d, %v); want (default, false)", v, ok)
	}
}

func TestTableCacheSizeDefaultAndOverride(t *testing.T) {
	t.Setenv(EnvTableCacheSize, "")
	if v, ok := TableCacheSize(); !ok || v != DefaultTableCacheSize {
		t.Fatalf("default: got (%d, %v); want (%d, true)", v, ok, DefaultTableCacheSize)
	}
	t.Setenv(EnvTableCacheSize, "512")
	if v, ok := TableCacheSize(); !ok || v != 512 {
		t.Fatalf("override: got (%d, %v); want (512, true)", v, ok)
	}
	t.Setenv(EnvTableCacheSize, "0")
	if v, ok := TableCacheSize(); ok || v != DefaultTableCacheSize {
		t.Fatalf("zero accepted: got (%d, %v); want (default, false)", v, ok)
	}
}
