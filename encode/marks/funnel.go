package marks

import (
	"fmt"
	"strconv"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeFunnel emits one PathGeom trapezoid + one TextGeom label
// per stage. Stage order = row order from the upstream table.
// Each stage's top width = value/maxValue × plot.W; bottom width =
// next-stage value / maxValue × plot.W (last stage's bottom = top,
// degenerating to a rectangle). Stages stack vertically with uniform
// height plot.H / N. See D066.
func encodeFunnel(in Inputs) ([]scene.Mark, error) {
	if in.X.Field == "" || in.Y.Field == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"funnel mark requires both x (stage) and y (value) channel bindings.",
			map[string]any{"Field": "<xy>", "Source": "<encoding>", "Available": joinFieldNames(in.Table)},
		)
	}
	stages, err := readField(in.Table, in.X.Field)
	if err != nil {
		return nil, err
	}
	values, err := readField(in.Table, in.Y.Field)
	if err != nil {
		return nil, err
	}
	n := len(values)
	if len(stages) != n {
		return nil, fmt.Errorf("encodeFunnel: column length mismatch (x=%d, y=%d)", len(stages), n)
	}
	if n == 0 {
		return nil, nil
	}

	nums := make([]float64, n)
	maxV := 0.0
	for i, v := range values {
		f, ok := toFloat64(v)
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("funnel value at row %d is not numeric (got %T).", i, v),
				map[string]any{"Field": in.Y.Field, "Source": "<y>", "Available": "numeric"},
			)
		}
		if f < 0 {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("funnel value at row %d is negative (%g); stage counts must be non-negative.", i, f),
				map[string]any{"Field": in.Y.Field, "Source": "<y>", "Available": "non-negative"},
			)
		}
		nums[i] = f
		if f > maxV {
			maxV = f
		}
	}
	if maxV == 0 {
		// Degenerate — all zeros. Produce no marks (avoid div-by-zero).
		return nil, nil
	}

	stageH := in.Layout.H / float64(n)
	cx := in.Layout.X + in.Layout.W/2

	// Color channel resolution (per-stage palette pick when color is bound).
	stageColors := make([]*scene.Color, n)
	if in.Color != nil {
		for i := 0; i < n; i++ {
			cat, _ := stages[i].(string)
			c := lookupCategoryColor(cat, in.Color.Categories, in.Color.Palette)
			stageColors[i] = c
		}
	}

	out := make([]scene.Mark, 0, n*2)
	for i := 0; i < n; i++ {
		topW := nums[i] / maxV * in.Layout.W
		// Last stage: bottom = top (rectangle); otherwise next stage's value.
		var botW float64
		if i+1 < n {
			botW = nums[i+1] / maxV * in.Layout.W
		} else {
			botW = topW
		}
		yTop := in.Layout.Y + float64(i)*stageH
		yBot := yTop + stageH
		tl := [2]float64{cx - topW/2, yTop}
		tr := [2]float64{cx + topW/2, yTop}
		br := [2]float64{cx + botW/2, yBot}
		bl := [2]float64{cx - botW/2, yBot}
		d := fmt.Sprintf("M%s,%s L%s,%s L%s,%s L%s,%s Z",
			fmtFloat(tl[0]), fmtFloat(tl[1]),
			fmtFloat(tr[0]), fmtFloat(tr[1]),
			fmtFloat(br[0]), fmtFloat(br[1]),
			fmtFloat(bl[0]), fmtFloat(bl[1]))
		style := in.Style
		if stageColors[i] != nil {
			style.Fill = stageColors[i]
		}
		stageName := fmt.Sprintf("%v", stages[i])
		out = append(out, scene.Mark{
			Type:  scene.MarkPath,
			ID:    fmt.Sprintf("funnel-stage-%s", stageName),
			Style: style,
			Path:  &scene.PathGeom{D: d},
		})
		// Inline value label, centered on the trapezoid.
		labelStyle := scene.Style{}
		out = append(out, scene.Mark{
			Type:  scene.MarkText,
			ID:    fmt.Sprintf("funnel-label-%s", stageName),
			Style: labelStyle,
			Text: &scene.TextGeom{
				X:        cx,
				Y:        (yTop + yBot) / 2,
				Content:  strconv.FormatFloat(nums[i], 'f', -1, 64),
				Anchor:   scene.AnchorMiddle,
				Baseline: scene.BaselineMiddle,
				FontSize: 12,
			},
		})
	}
	return out, nil
}
