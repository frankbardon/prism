package encode

import "testing"

// TestAutoTickFormatNeverEmitsExponent is the regression guard for the
// label that started this: a revenue axis rendered "1e+06".
func TestAutoTickFormatNeverEmitsExponent(t *testing.T) {
	ticks := []float64{0, 500000, 1000000, 1500000, 2000000}
	f := AutoTickFormat(ticks)
	want := []string{"0", "0.5M", "1M", "1.5M", "2M"}
	for i, v := range ticks {
		if got := f.Format(v); got != want[i] {
			t.Errorf("Format(%g) = %q, want %q", v, got, want[i])
		}
	}
}

func TestAutoTickFormatGroupsBelowCompactThreshold(t *testing.T) {
	ticks := []float64{0, 1000, 2000, 3000}
	f := AutoTickFormat(ticks)
	if got := f.Format(3000); got != "3,000" {
		t.Errorf("Format(3000) = %q, want %q", got, "3,000")
	}
	if got := f.Format(0); got != "0" {
		t.Errorf("Format(0) = %q, want %q", got, "0")
	}
}

// TestAutoTickFormatPrecisionFollowsStep asserts a fine step keeps its
// decimals and a coarse one drops them, rather than every axis
// inheriting one global precision.
func TestAutoTickFormatPrecisionFollowsStep(t *testing.T) {
	fine := AutoTickFormat([]float64{2.8, 3.0, 3.2, 3.4, 3.6})
	if got := fine.Format(3.2); got != "3.2" {
		t.Errorf("fine Format(3.2) = %q, want %q", got, "3.2")
	}
	if got := fine.Format(3.0); got != "3" {
		t.Errorf("fine Format(3) = %q, want %q (no trailing zero)", got, "3")
	}
	coarse := AutoTickFormat([]float64{0, 20, 40, 60, 80})
	if got := coarse.Format(40); got != "40" {
		t.Errorf("coarse Format(40) = %q, want %q", got, "40")
	}
}

// TestAutoTickFormatNoNegativeZero pins that a domain crossing zero
// never labels its origin "-0".
func TestAutoTickFormatNoNegativeZero(t *testing.T) {
	f := AutoTickFormat([]float64{-40, -20, 0, 20, 40})
	if got := f.Format(0); got != "0" {
		t.Errorf("Format(0) = %q, want %q", got, "0")
	}
	if got := f.Format(-20); got != "-20" {
		t.Errorf("Format(-20) = %q, want %q", got, "-20")
	}
}

func TestAutoTickFormatEmpty(t *testing.T) {
	if got := AutoTickFormat(nil).Format(7); got != "7" {
		t.Errorf("empty formatter Format(7) = %q, want %q", got, "7")
	}
}

// TestTextMetricsTruncateKeepsSomething asserts truncation never
// reduces a label to a bare ellipsis, however tight the budget.
func TestTextMetricsTruncateKeepsSomething(t *testing.T) {
	m := TextMetrics{FontSize: 11}
	for _, w := range []float64{1, 4, 12, 40} {
		got, cut := m.Truncate("Enterprise financial services", w)
		if !cut {
			t.Fatalf("width %g: expected truncation", w)
		}
		if got == Ellipsis || got == "" {
			t.Errorf("width %g: label reduced to %q", w, got)
		}
	}
}

func TestTextMetricsTruncateLeavesShortLabelsAlone(t *testing.T) {
	m := TextMetrics{FontSize: 11}
	got, cut := m.Truncate("Q1", 100)
	if cut || got != "Q1" {
		t.Errorf("Truncate(Q1, 100) = %q, %v; want unchanged", got, cut)
	}
}
