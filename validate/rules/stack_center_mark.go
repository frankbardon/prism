package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// StackCenterMark implements PRISM_SPEC_053: the centred stack offset
// is an area-mark shape.
//
// `stack: "center"` floats every stack's baseline so the band is
// symmetric about zero — the streamgraph. That layout reads because a
// ribbon carries its own two edges and the eye follows thickness. A
// bar does not: it is baseline-anchored geometry, and centring it
// detaches every column from the axis it is measured against, leaving
// a row of floating rectangles whose tick labels are offsets from a
// synthetic midline rather than values. The chart is not wrong so much
// as unreadable, so Prism refuses it rather than drawing it.
//
// `zero` and `normalize` are unaffected on bars — only the centred
// offset is rejected, and only on a mark that anchors to a baseline.
type StackCenterMark struct{}

// Code returns PRISM_SPEC_053.
func (StackCenterMark) Code() string { return "PRISM_SPEC_053" }

// Check walks every leaf spec and rejects a centred stack declared on
// a mark that cannot float its baseline.
func (StackCenterMark) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	walk(s, func(leaf *spec.Spec) {
		if leaf.Mark == nil || leaf.Encoding == nil {
			return
		}
		mark := leaf.Mark.TypeName()
		if centeringMark(mark) {
			return
		}
		for _, ch := range []struct {
			name string
			pos  *spec.PositionChannel
		}{{"x", leaf.Encoding.X}, {"y", leaf.Encoding.Y}} {
			if ch.pos == nil {
				continue
			}
			if v, ok := ch.pos.Stack.(string); !ok || v != spec.StackOffsetCenter {
				continue
			}
			out = append(out, errors.New("PRISM_SPEC_053",
				fmt.Sprintf("Channel %q declares stack \"center\", which mark type %q cannot draw.", ch.name, mark),
				map[string]any{"Channel": ch.name, "Mark": mark, "Offset": spec.StackOffsetCenter},
			))
		}
	})
	return out
}

// centeringMark reports whether markType can float its stack baseline.
// Only area can: it draws both of its edges from the data, so moving
// the pair off zero moves the whole ribbon intact.
func centeringMark(markType string) bool {
	return markType == "area"
}
