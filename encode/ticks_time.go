package encode

import (
	"time"

	"github.com/frankbardon/prism/encode/scene"
)

// TimeTicks produces calendar-aligned ticks for a time scale. The
// domain is interpreted as epoch milliseconds; ticks align to a
// natural calendar boundary chosen by the span:
//
//	span > 2 years   → year ticks ("2006")
//	span > 2 months  → month ticks ("2006-01")
//	span > 2 days    → day ticks ("2006-01-02")
//	span > 2 hours   → hour ticks ("15:04")
//	span > 2 minutes → minute ticks ("15:04")
//	span > 2 seconds → second ticks ("15:04:05")
//	else             → millisecond ticks ("15:04:05.000")
//
// Pure stdlib; no external date library. Returned ticks carry the
// epoch-ms float64 as Value (so callers can re-apply the scale) and a
// pre-formatted Label.
func TimeTicks(s *TimeScale, count int) []scene.Tick {
	if s == nil || s.Linear == nil {
		return nil
	}
	mn := time.UnixMilli(int64(s.Linear.DomainMin))
	mx := time.UnixMilli(int64(s.Linear.DomainMax))
	if !mx.After(mn) {
		return nil
	}
	span := mx.Sub(mn)
	level, fmtLayout := pickTimeLevel(span)
	starts := walkTimeTicks(mn, mx, level)
	if len(starts) > count*3 && count > 0 {
		// Down-sample uniformly when the natural granularity is too fine.
		stride := len(starts) / count
		if stride < 1 {
			stride = 1
		}
		filtered := starts[:0]
		for i := 0; i < len(starts); i += stride {
			filtered = append(filtered, starts[i])
		}
		starts = filtered
	}
	out := make([]scene.Tick, 0, len(starts))
	for _, t := range starts {
		epoch := float64(t.UnixMilli())
		pix, err := s.Linear.Apply(epoch)
		if err != nil {
			continue
		}
		out = append(out, scene.Tick{
			Value: epoch,
			Pixel: pix,
			Label: t.UTC().Format(fmtLayout),
		})
	}
	return out
}

// timeLevel discriminates the calendar bucket for tick alignment.
type timeLevel int

const (
	levelMillisecond timeLevel = iota
	levelSecond
	levelMinute
	levelHour
	levelDay
	levelMonth
	levelYear
)

// pickTimeLevel returns the calendar bucket + Go-format layout for a
// span. Layouts use Go's reference-time mnemonics, not strftime.
func pickTimeLevel(span time.Duration) (timeLevel, string) {
	switch {
	case span > 2*365*24*time.Hour:
		return levelYear, "2006"
	case span > 2*30*24*time.Hour:
		return levelMonth, "2006-01"
	case span > 48*time.Hour:
		return levelDay, "2006-01-02"
	case span > 2*time.Hour:
		return levelHour, "15:04"
	case span > 2*time.Minute:
		return levelMinute, "15:04"
	case span > 2*time.Second:
		return levelSecond, "15:04:05"
	}
	return levelMillisecond, "15:04:05.000"
}

// walkTimeTicks returns calendar-aligned timestamps between mn and mx
// at the chosen granularity. The first tick is the smallest aligned
// boundary >= mn; subsequent ticks step by the level's natural unit.
func walkTimeTicks(mn, mx time.Time, level timeLevel) []time.Time {
	var out []time.Time
	t := alignUp(mn.UTC(), level)
	for !t.After(mx) {
		out = append(out, t)
		t = step(t, level)
		if len(out) > 2000 {
			// Safety bound — prevents runaway loops on bad domains.
			break
		}
	}
	return out
}

// alignUp returns the smallest calendar boundary at level >= t.
func alignUp(t time.Time, level timeLevel) time.Time {
	switch level {
	case levelYear:
		y := t.Year()
		boundary := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
		if boundary.Before(t) {
			boundary = time.Date(y+1, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		return boundary
	case levelMonth:
		y, m, _ := t.Date()
		boundary := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
		if boundary.Before(t) {
			boundary = boundary.AddDate(0, 1, 0)
		}
		return boundary
	case levelDay:
		y, m, d := t.Date()
		boundary := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
		if boundary.Before(t) {
			boundary = boundary.AddDate(0, 0, 1)
		}
		return boundary
	case levelHour:
		y, m, d := t.Date()
		boundary := time.Date(y, m, d, t.Hour(), 0, 0, 0, time.UTC)
		if boundary.Before(t) {
			boundary = boundary.Add(time.Hour)
		}
		return boundary
	case levelMinute:
		y, m, d := t.Date()
		boundary := time.Date(y, m, d, t.Hour(), t.Minute(), 0, 0, time.UTC)
		if boundary.Before(t) {
			boundary = boundary.Add(time.Minute)
		}
		return boundary
	case levelSecond:
		y, m, d := t.Date()
		boundary := time.Date(y, m, d, t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
		if boundary.Before(t) {
			boundary = boundary.Add(time.Second)
		}
		return boundary
	}
	return t
}

// step advances t by one unit at the level's natural granularity.
func step(t time.Time, level timeLevel) time.Time {
	switch level {
	case levelYear:
		return t.AddDate(1, 0, 0)
	case levelMonth:
		return t.AddDate(0, 1, 0)
	case levelDay:
		return t.AddDate(0, 0, 1)
	case levelHour:
		return t.Add(time.Hour)
	case levelMinute:
		return t.Add(time.Minute)
	case levelSecond:
		return t.Add(time.Second)
	}
	return t.Add(time.Millisecond)
}
