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
//
// When a span channel is bound (E9-S3) the rule stops spanning the
// plot and becomes an interval segment instead — see encodeRuleSpan.
func encodeRule(in Inputs) ([]scene.Mark, error) {
	if spanBound(in.X2) || spanBound(in.Y2) {
		return encodeRuleSpan(in)
	}
	style, err := ruleStyles(in)
	if err != nil {
		return nil, err
	}
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
			if skipRow(in, i) {
				continue
			}
			y, err := PointPixel(in.Y, v)
			if err != nil {
				return nil, err
			}
			marks = append(marks, scene.Mark{
				Type:  scene.MarkRule,
				ID:    fmt.Sprintf("rule-%d", i),
				Style: style(i),
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
			if skipRow(in, i) {
				continue
			}
			x, err := PointPixel(in.X, v)
			if err != nil {
				return nil, err
			}
			marks = append(marks, scene.Mark{
				Type:  scene.MarkRule,
				ID:    fmt.Sprintf("rule-%d", i),
				Style: style(i),
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
		if skipRow(in, i) {
			continue
		}
		x, err := PointPixel(in.X, xs[i])
		if err != nil {
			return nil, err
		}
		y, err := PointPixel(in.Y, ys[i])
		if err != nil {
			return nil, err
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("rule-%d", i),
			Style: style(i),
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

// encodeRuleSpan emits one interval segment per row, running from
// (x, y) to (x2, y2). An unbound span channel holds its endpoint at
// the base channel's pixel, so `x`/`x2` plus `y` draws a horizontal
// interval (an error bar's whisker), `y`/`y2` plus `x` draws a
// vertical one, and binding all four draws the connecting diagonal.
//
// Endpoints keep their authored order rather than being sorted, so a
// descending interval stays a descending segment.
//
// Both base channels must be bound: a segment with only one
// coordinate has no position to sit at, and silently inventing one is
// exactly the no-op this path exists to remove.
func encodeRuleSpan(in Inputs) ([]scene.Mark, error) {
	if in.X.Field == "" || in.X.Scale == nil || in.Y.Field == "" || in.Y.Scale == nil {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"An interval rule (x2 / y2 bound) requires both x and y to be bound.",
			map[string]any{"Field": "<rule>", "Source": "<encoding>", "Available": "x, y"},
		)
	}
	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeRuleSpan: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}
	style, err := ruleStyles(in)
	if err != nil {
		return nil, err
	}
	var x2s, y2s []any
	if spanBound(in.X2) {
		if x2s, err = readField(in.Table, in.X2.Field); err != nil {
			return nil, err
		}
	}
	if spanBound(in.Y2) {
		if y2s, err = readField(in.Table, in.Y2.Field); err != nil {
			return nil, err
		}
	}

	marks := make([]scene.Mark, 0, len(xs))
	for i := range xs {
		if skipRow(in, i) {
			continue
		}
		x1, err := PointPixel(in.X, xs[i])
		if err != nil {
			return nil, err
		}
		y1, err := PointPixel(in.Y, ys[i])
		if err != nil {
			return nil, err
		}
		x2, y2 := x1, y1
		if i < len(x2s) {
			if x2, err = PointPixel(in.X2, x2s[i]); err != nil {
				return nil, err
			}
		}
		if i < len(y2s) {
			if y2, err = PointPixel(in.Y2, y2s[i]); err != nil {
				return nil, err
			}
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkRule,
			ID:    fmt.Sprintf("rule-%d", i),
			Style: style(i),
			Rule: &scene.RuleGeom{
				X1: x1,
				Y1: y1,
				X2: x2,
				Y2: y2,
			},
		})
	}
	return marks, nil
}

// ruleStyles returns the style of row i's rule. With a categorical
// color channel bound, a rule strokes in its row's category colour —
// the colour a point or bar of the same category fills with, paired
// for auto-dark through the same registry — so an interval drawn as a
// rule matches the dot or bar it brackets. Unbound, every rule keeps
// in.Style.
func ruleStyles(in Inputs) (func(i int) scene.Style, error) {
	if in.Color == nil || in.Color.Field == "" {
		return func(int) scene.Style { return in.Style }, nil
	}
	cats, err := readField(in.Table, in.Color.Field)
	if err != nil {
		return nil, err
	}
	return func(i int) scene.Style {
		style := in.Style
		if i >= len(cats) {
			return style
		}
		cat, _ := cats[i].(string)
		if c, v := resolveCategoryColor(in, cat); c != nil || v != "" {
			style.Stroke, style.StrokeRef, style.StrokeVar = c, "", v
		}
		return style
	}, nil
}
