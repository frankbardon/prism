package encode

import (
	"testing"
	"time"
)

// TestTimeTicksMonthSpan ensures a 6-month domain yields month-level
// ticks formatted "2006-01".
func TestTimeTicksMonthSpan(t *testing.T) {
	mn := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mx := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	s := &TimeScale{Linear: &LinearScale{
		DomainMin: float64(mn.UnixMilli()),
		DomainMax: float64(mx.UnixMilli()),
		RangeMin:  0, RangeMax: 600,
	}}
	ticks := TimeTicks(s, 6)
	if len(ticks) < 4 {
		t.Fatalf("expected ~6 ticks, got %d", len(ticks))
	}
	if ticks[0].Label != "2026-01" {
		t.Errorf("first label = %q, want 2026-01", ticks[0].Label)
	}
}

// TestTimeTicksDaySpan ensures a 5-day domain yields day-level ticks
// formatted "2006-01-02".
func TestTimeTicksDaySpan(t *testing.T) {
	mn := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	mx := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
	s := &TimeScale{Linear: &LinearScale{
		DomainMin: float64(mn.UnixMilli()),
		DomainMax: float64(mx.UnixMilli()),
		RangeMin:  0, RangeMax: 600,
	}}
	ticks := TimeTicks(s, 5)
	if len(ticks) < 3 {
		t.Fatalf("expected day ticks, got %d", len(ticks))
	}
	if ticks[0].Label != "2026-05-01" {
		t.Errorf("first label = %q, want 2026-05-01", ticks[0].Label)
	}
}

// TestTimeTicksHourSpan ensures a 6-hour domain yields hour ticks.
func TestTimeTicksHourSpan(t *testing.T) {
	mn := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	mx := time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)
	s := &TimeScale{Linear: &LinearScale{
		DomainMin: float64(mn.UnixMilli()),
		DomainMax: float64(mx.UnixMilli()),
		RangeMin:  0, RangeMax: 600,
	}}
	ticks := TimeTicks(s, 6)
	if len(ticks) < 3 {
		t.Fatalf("expected hour ticks, got %d", len(ticks))
	}
	if ticks[0].Label != "09:00" {
		t.Errorf("first label = %q, want 09:00", ticks[0].Label)
	}
}
