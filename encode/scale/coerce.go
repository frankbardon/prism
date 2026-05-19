package scale

import (
	"math"
	"time"
)

// ToFloat coerces common Go numeric types to float64. Returns
// (0, false) for NaN / Inf / non-numeric inputs. Exported so the
// encoder + per-scale apply paths share one definition.
func ToFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, !math.IsNaN(x) && !math.IsInf(x, 0)
	case float32:
		f := float64(x)
		return f, !math.IsNaN(f) && !math.IsInf(f, 0)
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// ToEpochMs converts a time-shaped value (time.Time, ISO-8601 string,
// numeric epoch ms) to float64 epoch ms.
func ToEpochMs(v any) (float64, bool) {
	switch x := v.(type) {
	case time.Time:
		return float64(x.UnixMilli()), true
	case string:
		if t, err := time.Parse(time.RFC3339, x); err == nil {
			return float64(t.UnixMilli()), true
		}
		if t, err := time.Parse("2006-01-02", x); err == nil {
			return float64(t.UnixMilli()), true
		}
		return 0, false
	}
	return ToFloat(v)
}
