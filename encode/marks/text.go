package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/format"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeText emits one TextGeom per table row. At least one position
// channel (x or y) must be bound; the unbound axis centres the label
// in the plot region. spec.Mark.Def carries font/angle/anchor.
//
// Label content resolves in this order:
//
//  1. encoding.text with a `field` — the bound column's value for the
//     row, rendered through the channel's `format` specifier (a
//     d3-format subset, see encode/format) when one is set.
//  2. encoding.text with only a `value` — that literal on every row,
//     likewise formatted when `format` is set.
//  3. No text channel (or an empty one) — the y-field value verbatim,
//     falling back to the x-field value when y is unbound. This is
//     the historical behaviour and is preserved byte-for-byte.
func encodeText(in Inputs) ([]scene.Mark, error) {
	xBound := in.X.Field != "" && in.X.Scale != nil
	yBound := in.Y.Field != "" && in.Y.Scale != nil
	if !xBound && !yBound {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"text mark requires at least one of x or y to be bound.",
			map[string]any{"Field": "<text>", "Source": "<encoding>", "Available": "x|y"},
		)
	}

	var xs, ys []any
	var err error
	if xBound {
		if xs, err = readField(in.Table, in.X.Field); err != nil {
			return nil, err
		}
	}
	if yBound {
		if ys, err = readField(in.Table, in.Y.Field); err != nil {
			return nil, err
		}
	}
	if xBound && yBound && len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeText: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}
	rowCount := len(xs)
	if !xBound {
		rowCount = len(ys)
	}

	contents, err := textContents(in, xs, ys, rowCount)
	if err != nil {
		return nil, err
	}

	anchor := scene.AnchorMiddle
	baseline := scene.BaselineMiddle
	angle := 0.0
	fontSize := 11.0
	// dx / dy (E4-S1) offset the glyph from its anchor. They ride on
	// the geometry rather than the style because they are positional,
	// and the renderer applies them after Angle — see scene.TextGeom.
	dx, dy := 0.0, 0.0
	if in.Mark != nil {
		switch in.Mark.Align {
		case "left":
			anchor = scene.AnchorStart
		case "right":
			anchor = scene.AnchorEnd
		}
		switch in.Mark.Baseline {
		case "top":
			baseline = scene.BaselineTop
		case "bottom":
			baseline = scene.BaselineBottom
		}
		if in.Mark.Angle != nil {
			angle = *in.Mark.Angle
		}
		if in.Mark.FontSize != nil {
			fontSize = *in.Mark.FontSize
		}
		if in.Mark.Dx != nil {
			dx = *in.Mark.Dx
		}
		if in.Mark.Dy != nil {
			dy = *in.Mark.Dy
		}
	}

	marks := make([]scene.Mark, 0, rowCount)
	for i := 0; i < rowCount; i++ {
		x := in.Layout.CenterX()
		if xBound {
			if x, err = PointPixel(in.X, xs[i]); err != nil {
				return nil, err
			}
		}
		y := in.Layout.CenterY()
		if yBound {
			if y, err = PointPixel(in.Y, ys[i]); err != nil {
				return nil, err
			}
		}
		marks = append(marks, scene.Mark{
			Type:  scene.MarkText,
			ID:    fmt.Sprintf("text-%d", i),
			Style: in.Style,
			Text: &scene.TextGeom{
				X:        x,
				Y:        y,
				Content:  contents[i],
				Anchor:   anchor,
				Baseline: baseline,
				Angle:    angle,
				FontSize: fontSize,
				Dx:       dx,
				Dy:       dy,
			},
		})
	}
	return marks, nil
}

// textContents resolves the per-row label string for a text mark.
// xs / ys are the already-read position columns (nil when the channel
// is unbound) and back the no-text-channel fallback.
func textContents(in Inputs, xs, ys []any, rowCount int) ([]string, error) {
	out := make([]string, rowCount)

	// Formatter shared by every branch. An unparseable specifier
	// degrades to "%v" here — PRISM_SPEC_011 rejects it at validate
	// time, so encode never needs to be the one to complain.
	var formatter *format.Spec
	if in.Text != nil && in.Text.Format != "" {
		if sp, err := format.Parse(in.Text.Format); err == nil {
			formatter = sp
		}
	}
	render := func(v any) string {
		if formatter != nil {
			return formatter.Apply(v)
		}
		return fmt.Sprintf("%v", v)
	}

	switch {
	case in.Text != nil && in.Text.Field != "":
		labels, err := readField(in.Table, in.Text.Field)
		if err != nil {
			return nil, err
		}
		if len(labels) < rowCount {
			return nil, fmt.Errorf("encodeText: text column %q has %d rows, expected %d", in.Text.Field, len(labels), rowCount)
		}
		for i := 0; i < rowCount; i++ {
			out[i] = render(labels[i])
		}
	case in.Text != nil && in.Text.Value != nil:
		literal := render(in.Text.Value)
		for i := 0; i < rowCount; i++ {
			out[i] = literal
		}
	default:
		// Historical fallback: the y value (x when y is unbound). With
		// no text channel at all there is no formatter either, so the
		// output is byte-identical to the pre-text-channel encoder; a
		// text channel carrying only `format` still formats it.
		src := ys
		if src == nil {
			src = xs
		}
		for i := 0; i < rowCount; i++ {
			out[i] = render(src[i])
		}
	}
	return out, nil
}
