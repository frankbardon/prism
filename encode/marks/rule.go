package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeRule emits one scene.Mark per table row with a RuleGeom that
// spans the plot horizontally (when y is bound) or vertically (when
// x is bound). P05 supports both orientations but the typical
// fixture binds only y (horizontal threshold lines).
func encodeRule(in Inputs) ([]scene.Mark, error) {
	yBound := in.Y.Field != ""
	xBound := in.X.Field != ""
	if !yBound && !xBound {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"rule mark requires at least one of x or y to be bound.",
			map[string]any{"Field": "<rule>", "Source": "<encoding>", "Available": "x|y"},
		)
	}

	if yBound && !xBound {
		// Horizontal rule per row, spanning plot width.
		ys, err := readField(in.Table, in.Y.Field)
		if err != nil {
			return nil, err
		}
		marks := make([]scene.Mark, 0, len(ys))
		for i, v := range ys {
			y, err := in.Y.Scale.Apply(v)
			if err != nil {
				return nil, err
			}
			marks = append(marks, scene.Mark{
				Type:  scene.MarkRule,
				ID:    fmt.Sprintf("rule-%d", i),
				Style: in.Style,
				Rule: &scene.RuleGeom{
					X1: in.Layout.X,
					Y1: y,
					X2: in.Layout.Right(),
					Y2: y,
				},
			})
		}
		return marks, nil
	}

	if xBound && !yBound {
		// Vertical rule per row, spanning plot height.
		xs, err := readField(in.Table, in.X.Field)
		if err != nil {
			return nil, err
		}
		marks := make([]scene.Mark, 0, len(xs))
		for i, v := range xs {
			x, err := in.X.Scale.Apply(v)
			if err != nil {
				return nil, err
			}
			marks = append(marks, scene.Mark{
				Type:  scene.MarkRule,
				ID:    fmt.Sprintf("rule-%d", i),
				Style: in.Style,
				Rule: &scene.RuleGeom{
					X1: x,
					Y1: in.Layout.Y,
					X2: x,
					Y2: in.Layout.Bottom(),
				},
			})
		}
		return marks, nil
	}

	// Both bound: tiny horizontal line at each (x, y). Uncommon in
	// v1; bar / point cover the typical cases. Just emit a single
	// 1-px-wide horizontal rule at each row.
	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeRule: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}
	marks := make([]scene.Mark, 0, len(xs))
	for i := range xs {
		x, err := in.X.Scale.Apply(xs[i])
		if err != nil {
			return nil, err
		}
		y, err := in.Y.Scale.Apply(ys[i])
		if err != nil {
			return nil, err
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("rule-%d", i),
			Style: in.Style,
			Rule: &scene.RuleGeom{
				X1: x - 0.5,
				Y1: y,
				X2: x + 0.5,
				Y2: y,
			},
		})
	}
	return marks, nil
}
