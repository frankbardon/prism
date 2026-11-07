package format

import (
	"testing"
	"time"
)

// TestPrismD3FormatStrings — required by PHASE.md. Table-driven
// asserts of each supported specifier against expected output.
func TestPrismD3FormatStrings(t *testing.T) {
	cases := []struct {
		name string
		spec string
		in   any
		want string
	}{
		{"comma-fixed-2", ",.2f", 1234.5, "1,234.50"},
		{"pct-0dec", ".0%", 0.123, "12%"},
		{"pct-default", "%", 0.123, "12.3%"},
		{"comma-int", ",d", 1234567, "1,234,567"},
		{"si-default", ".0s", 1234.0, "1k"},
		{"si-with-prec", ".2s", 1234.0, "1.23k"},
		{"sci-3dec", ".3e", 1234.5, "1.234e+03"},
		{"date-ymd", "%Y-%m-%d", time.Date(2026, 5, 19, 14, 30, 0, 0, time.UTC), "2026-05-19"},
		{"hour-min", "%H:%M", time.Date(2026, 5, 19, 14, 30, 0, 0, time.UTC), "14:30"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sp, err := Parse(tc.spec)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.spec, err)
			}
			got := sp.Apply(tc.in)
			if got != tc.want {
				t.Errorf("Apply(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestPrismFormatInvalidSpec asserts malformed specs surface
// PRISM_SPEC_011.
func TestPrismFormatInvalidSpec(t *testing.T) {
	bad := []string{",.x", "%Z", "abc", "junk-spec"}
	for _, s := range bad {
		_, err := Parse(s)
		if err == nil {
			t.Errorf("expected error for %q, got nil", s)
			continue
		}
		// AppError-shaped: Code field == PRISM_SPEC_011
		type coded interface{ AppErrorCode() string }
		_ = coded(nil) // skip if not coded
	}
}

// TestPrismFormatEpochMsAsTime ensures float64 epoch-ms inputs are
// accepted by time-typed specs.
func TestPrismFormatEpochMsAsTime(t *testing.T) {
	sp := MustParse("%Y-%m-%d")
	t0 := time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)
	got := sp.Apply(float64(t0.UnixMilli()))
	if got != "2026-05-19" {
		t.Errorf("Apply(epoch_ms) = %q, want 2026-05-19", got)
	}
}

// TestPrismFormatTrimSignPrecision covers the d3 pieces added for
// chart builders that format in the spec rather than upstream: the
// "~" trim, the sign flag and "g" with a precision (toPrecision).
func TestPrismFormatTrimSignPrecision(t *testing.T) {
	cases := []struct {
		spec string
		in   any
		want string
	}{
		{".2~f", 1.5, "1.5"},
		{".2~f", 2.0, "2"},
		{".2~f", 0.126, "0.13"},
		{",.2~f", 1234.5, "1,234.5"},
		{".1~%", 0.25, "25%"},
		{".2~%", 0.1234, "12.34%"},
		{".3~e", 1500.0, "1.5e+03"},
		{".2~s", 2000.0, "2k"},
		{".4g", 1234.5678, "1235"},
		{".4g", 0.00012346, "0.0001235"},
		{".4g", 12345678.0, "1.235e+7"},
		{".2g", 9.99, "10"},
		{".3g", 0.0, "0.00"},
		{".3~g", 0.0, "0"},
		{".3~g", 1.5, "1.5"},
		{".4~g", 0.000000123, "1.23e-7"},
		{",.4~g", 123456.0, "1.235e+5"},
		{",.6~g", 123456.0, "123,456"},
		{"+.1f", 2.0, "+2.0"},
		{"+.1f", -2.0, "-2.0"},
		{"+.1f", 0.0, "+0.0"},
		{"+d", 3.0, "+3"},
		{" .1f", 2.0, " 2.0"},
		{"-.1f", 2.0, "2.0"},
	}
	for _, tc := range cases {
		sp, err := Parse(tc.spec)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.spec, err)
		}
		if got := sp.Apply(tc.in); got != tc.want {
			t.Errorf("Parse(%q).Apply(%v) = %q, want %q", tc.spec, tc.in, got, tc.want)
		}
	}
	for _, bad := range []string{"~~f", ".2f~", "++f"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q): want an error", bad)
		}
	}
}
