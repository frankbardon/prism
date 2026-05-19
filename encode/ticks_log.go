package encode

import (
	"math"
	"strconv"

	"github.com/frankbardon/prism/encode/scene"
)

// LogTicks returns the canonical tick set for a LogScale: integer
// powers of base between log_b(min) and log_b(max) as major ticks,
// with the mantissa values (2..base-1) at each major as minor ticks.
//
// Major tick labels render in compact form (e.g. "1k" for 1e3 when
// base=10 and value >= 1000). Minor ticks carry empty labels so the
// renderer skips text emission.
func LogTicks(s *LogScale) []scene.Tick {
	base := s.Base
	if base == 0 {
		base = 10
	}
	if s.DomainMin <= 0 || s.DomainMax <= 0 || s.DomainMax <= s.DomainMin {
		return nil
	}
	logBase := math.Log(base)
	pMin := int(math.Floor(math.Log(s.DomainMin) / logBase))
	pMax := int(math.Ceil(math.Log(s.DomainMax) / logBase))

	out := []scene.Tick{}
	for p := pMin; p <= pMax; p++ {
		major := math.Pow(base, float64(p))
		if major >= s.DomainMin && major <= s.DomainMax {
			pix, err := s.Apply(major)
			if err == nil {
				out = append(out, scene.Tick{
					Value: major,
					Pixel: pix,
					Label: formatLogTick(major),
				})
			}
		}
		// Minor ticks at mantissa positions (2..base-1) between
		// major and base * major.
		intBase := int(base)
		for m := 2; m < intBase; m++ {
			v := float64(m) * major
			if v < s.DomainMin || v > s.DomainMax {
				continue
			}
			pix, err := s.Apply(v)
			if err == nil {
				out = append(out, scene.Tick{
					Value: v,
					Pixel: pix,
					Label: "",
					Minor: true,
				})
			}
		}
	}
	return out
}

// formatLogTick renders a log tick value in a compact form. Powers
// of 10 use scientific (1e3) above 9999 or below 0.001; otherwise the
// natural decimal form.
func formatLogTick(v float64) string {
	abs := math.Abs(v)
	if abs >= 10000 || (abs > 0 && abs < 0.001) {
		return strconv.FormatFloat(v, 'e', 0, 64)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}
