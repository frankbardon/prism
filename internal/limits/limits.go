// Package limits centralises the env-driven memory ceilings declared
// in DECISIONS.md D007. Every cap has a default constant and a lookup
// helper that consults the matching PRISM_* environment variable.
//
// The lookup helpers parse loudly. A non-empty env var that fails to
// parse, or that resolves to a non-positive value, is rejected by
// returning the default plus ok=false so callers can surface a config
// error rather than silently fall back. The companion Must* helpers
// return only the resolved value for callers that prefer the default-
// only behaviour.
package limits

import (
	"os"
	"strconv"
)

// Default ceilings. Keep these in sync with DECISIONS.md D007 and with
// design/05-dag-executor.md.
const (
	// DefaultTableMaxRows caps any single materialised *table.Table.
	// See PRISM_TABLE_MAX_ROWS.
	DefaultTableMaxRows = 50_000_000

	// DefaultJoinMaxRows caps the product of the two sides of a hash
	// join (left rows × right rows). See PRISM_JOIN_MAX_ROWS.
	DefaultJoinMaxRows = 5_000_000

	// DefaultRenderMaxMarks caps the number of marks the renderer
	// emits before auto-Sample injection. See PRISM_RENDER_MAX_MARKS.
	DefaultRenderMaxMarks = 100_000
)

// Env var names. Exported so callers (CLI help text, error fixups) can
// reference the canonical names without typo risk.
const (
	EnvTableMaxRows   = "PRISM_TABLE_MAX_ROWS"
	EnvJoinMaxRows    = "PRISM_JOIN_MAX_ROWS"
	EnvRenderMaxMarks = "PRISM_RENDER_MAX_MARKS"
)

// TableMaxRows returns the effective cap for any single Table. The
// second return is false when the env var was set but unparseable or
// non-positive; callers may surface this as a PRISM_CONFIG_* error or
// silently fall back to the default value (also returned in that case).
func TableMaxRows() (int, bool) {
	return lookup(EnvTableMaxRows, DefaultTableMaxRows)
}

// MustTableMaxRows returns the resolved cap, discarding the ok flag.
// Equivalent to the first return of TableMaxRows; useful at call sites
// that intentionally tolerate malformed env vars.
func MustTableMaxRows() int {
	v, _ := TableMaxRows()
	return v
}

// JoinMaxRows mirrors TableMaxRows for the join-product ceiling.
func JoinMaxRows() (int, bool) {
	return lookup(EnvJoinMaxRows, DefaultJoinMaxRows)
}

// MustJoinMaxRows mirrors MustTableMaxRows.
func MustJoinMaxRows() int {
	v, _ := JoinMaxRows()
	return v
}

// RenderMaxMarks mirrors TableMaxRows for the render mark ceiling.
func RenderMaxMarks() (int, bool) {
	return lookup(EnvRenderMaxMarks, DefaultRenderMaxMarks)
}

// MustRenderMaxMarks mirrors MustTableMaxRows.
func MustRenderMaxMarks() int {
	v, _ := RenderMaxMarks()
	return v
}

// lookup resolves an integer env var with a default fallback. Returns
// (default, true) when the env var is unset; (parsed, true) when set
// and valid; (default, false) when set but unparseable or non-positive.
func lookup(name string, def int) (int, bool) {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def, false
	}
	return n, true
}
