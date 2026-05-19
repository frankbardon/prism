package scale

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// TimeScale wraps a LinearScale over epoch-ms. Apply accepts
// time.Time, ISO-8601 strings, and numeric epoch ms. Calendar-aware
// tick generation lives in encode/ticks_time.go.
type TimeScale struct {
	Linear *LinearScale
}

// Apply implements Scale.
func (s *TimeScale) Apply(value any) (float64, error) {
	v, ok := ToEpochMs(value)
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("TimeScale.Apply: value %v (type %T) is not a recognised time form (want ISO-8601 string, time.Time, or numeric epoch ms).", value, value),
			map[string]any{"Field": "<time>", "Source": "<scale>", "Available": "iso-8601 | time.Time | float epoch_ms"},
		)
	}
	return s.Linear.Apply(v)
}

// Domain implements Scale.
func (s *TimeScale) Domain() []any { return s.Linear.Domain() }

// Range implements Scale.
func (s *TimeScale) Range() [2]float64 { return s.Linear.Range() }

// Type implements Scale.
func (s *TimeScale) Type() scene.ScaleType { return scene.ScaleTime }
