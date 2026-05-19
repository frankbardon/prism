package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// encodeText emits one TextGeom per table row. Required encodings:
// position (x and/or y) + spec.Mark.Def carrying font/angle/anchor.
// Label content reads from the encoding's `text` channel field; when
// no text channel is bound, encoder falls back to the y-field value.
func encodeText(in Inputs) ([]scene.Mark, error) {
	xs, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	ys, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	if len(xs) != len(ys) {
		return nil, fmt.Errorf("encodeText: column length mismatch (x=%d, y=%d)", len(xs), len(ys))
	}

	anchor := scene.AnchorMiddle
	baseline := scene.BaselineMiddle
	angle := 0.0
	fontSize := 11.0
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
			Type:  scene.MarkText,
			ID:    fmt.Sprintf("text-%d", i),
			Style: in.Style,
			Text: &scene.TextGeom{
				X:        x,
				Y:        y,
				Content:  fmt.Sprintf("%v", ys[i]),
				Anchor:   anchor,
				Baseline: baseline,
				Angle:    angle,
				FontSize: fontSize,
			},
		})
	}
	return marks, nil
}
