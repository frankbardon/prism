package encode

import (
	"github.com/frankbardon/prism/encode/marks"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/theme"
)

// boxplotMedianStyle resolves the paint for the median line a boxplot
// draws across its box.
//
// The median is the one boxplot element drawn ON the filled box rather
// than outside it, so it is the one element the box's own colour cannot
// serve: a median stroked in the box's fill is exactly as unreadable as
// a median with no stroke at all. Every built-in theme already answers
// this question for `arc`, whose wedge separators face the same
// problem, so the shipped defaults mirror each theme's own arc stroke
// rather than inventing a second convention.
//
// The lookup is t.Marks[MarksKeyBoxplotMedian] DIRECTLY, never
// MarkDefault: MarkDefault folds the global theme.Mark block in first,
// and theme.Mark.Fill is the data-mark colour — the very value that
// makes the median vanish. Same rule, and the same reason, as
// progressTrackStyle.
//
// The fallback when a theme names no median style is the whisker
// treatment (stroke taken from the box fill). That is not ideal, but it
// is strictly better than the pre-fix state of no stroke at all, and a
// theme that wants a readable median states one.
func boxplotMedianStyle(t *theme.Theme, base scene.Style) scene.Style {
	style := marks.StrokeStyleFor(base)
	if t == nil {
		return style
	}
	ms := t.Marks[theme.MarksKeyBoxplotMedian]
	if ms == nil {
		return style
	}
	// A themed median replaces the derived stroke outright; clear the
	// derived paint first so applyThemeMarkStyle's stroke lands
	// instead of competing with it.
	style.Stroke, style.StrokeRef, style.StrokeVar = nil, "", ""
	applyThemeMarkStyle(&style, ms, t, nil, nil)
	if style.Stroke == nil && style.StrokeRef == "" && style.StrokeVar == "" {
		return marks.StrokeStyleFor(base)
	}
	return style
}
