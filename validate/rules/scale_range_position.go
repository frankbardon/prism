package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ScaleRangePosition implements PRISM_SPEC_044: `scale.range` is
// honoured on color channels only, and declaring it on a position
// channel is an error rather than a silent no-op.
//
// The reason is structural. A position scale's range is not the
// author's to choose: encode/layout.go computes the plot rect, and
// the axis builder, the gridlines and every mark encoder all measure
// against that same rect. A spec-supplied range would move the marks
// without moving the chrome, so the chart would render with axes that
// disagree with the data they label — the worst failure mode a
// visualization library has, because it looks fine.
//
// Position extent is reachable through the supported keys instead:
// `scale.domain` / `zero` / `nice` bound what the axis covers, and
// `width` / `height` / `padding` size the rect the scale fills.
type ScaleRangePosition struct{}

// Code returns PRISM_SPEC_044.
func (ScaleRangePosition) Code() string { return "PRISM_SPEC_044" }

// Check walks every position channel, on the root spec and on every
// composition child, and flags a declared scale.range.
func (ScaleRangePosition) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	walkScaleDomainSpecs(s, func(sub *spec.Spec) {
		out = append(out, checkScaleRangePosition(sub.Encoding)...)
	})
	return out
}

func checkScaleRangePosition(enc *spec.Encoding) []*errors.AppError {
	if enc == nil {
		return nil
	}
	var out []*errors.AppError
	check := func(channel string, ch *spec.PositionChannel) {
		if ch == nil || ch.Scale == nil || ch.Scale.Range == nil {
			return
		}
		out = append(out, errors.New("PRISM_SPEC_044",
			fmt.Sprintf("Channel %q declares scale.range, which Prism honours on color channels only.", channel),
			map[string]any{"Channel": channel},
		))
	}
	check("x", enc.X)
	check("y", enc.Y)
	check("x2", enc.X2)
	check("y2", enc.Y2)
	check("theta", enc.Theta)
	check("radius", enc.Radius)
	return out
}
