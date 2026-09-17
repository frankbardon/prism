package encode

import (
	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// Progress mark encode-side plumbing (E10-S1).
//
// Two things the mark encoder cannot do for itself live here, because
// both run before it does:
//
//  1. progressMeasureExtras widens the measure channel's domain with
//     mark.total, so the scale ends where the track ends. Without it the
//     domain stops at the data max and a total above that max would put
//     the track's far edge outside the plot — the bug bullet still has
//     with a band bound above its data range.
//  2. progressTrackStyle resolves the track's paint from the active
//     theme. The track is a distinct scene mark, so it takes a distinct
//     Style; since E10-S2 that Style comes from a first-class theme
//     token, theme.Marks["progress_track"] (see marks.Inputs.TrackStyle).

// progressTrackFallbackFill is the track colour of last resort: no
// theme at all, or a theme that sets neither the progress_track token
// nor a grid colour. Same posture as hardcodedDefaultStyle — it exists
// so a nil theme still draws a readable chart, not as the default.
// Every built-in theme states its own value.
const progressTrackFallbackFill = "#e5e7eb"

// progressTrackStyle resolves the Style of a progress mark's unfilled
// track from t.
//
// Resolution order, widest fallback first:
//
//  1. progressTrackFallbackFill — the nil-theme last resort above.
//  2. The theme's grid colour (nested axis block, then the legacy flat
//     field). The track is chrome rather than a second series: it
//     reads as the plot's own ground, so a theme that never heard of
//     progress still tints it in the right family.
//  3. theme.Marks["progress_track"] — the token, folded in through the
//     same applyThemeMarkStyle every other mark's theme style goes
//     through, so the track honours fill / stroke / stroke_width /
//     opacity / gradient / pattern refs exactly as a bar does.
//
// theme.Mark (the global data-mark default) is intentionally NOT folded
// in, which is why this reads t.Marks directly instead of calling
// t.MarkDefault. theme.Mark carries the data fill — light's is
// #4c78a8 — and inheriting it would paint the track the same colour as
// the value bar sitting on it, erasing the reading. The track is
// chrome; the global mark default is not addressed to it.
//
// The spec-side escape hatch is unchanged and still wins: mark_def
// styling applies to the value bar only, so a caller restyling the
// track does it through the theme, which is the point of the token.
func progressTrackStyle(t *theme.Theme) scene.Style {
	style := scene.Style{}
	if c, err := scene.ColorFromHex(progressTrackDerivedFill(t)); err == nil {
		style.Fill = c
	}
	if t == nil {
		return style
	}
	if ms := t.Marks[theme.MarksKeyProgressTrack]; ms != nil {
		applyThemeMarkStyle(&style, ms, t, nil, nil)
	}
	return style
}

// progressTrackDerivedFill returns the pre-token fill a theme implies
// for the track: its grid colour, nested block first, then the legacy
// flat field, then the last-resort constant.
func progressTrackDerivedFill(t *theme.Theme) string {
	if t == nil {
		return progressTrackFallbackFill
	}
	if t.Axis != nil && t.Axis.GridColor != "" {
		return t.Axis.GridColor
	}
	if t.GridColor != "" {
		return t.GridColor
	}
	return progressTrackFallbackFill
}

// progressMeasureIsX reports whether a progress mark's measure axis is
// x (the horizontal reading) rather than y.
//
// It answers, from the spec alone, the question marks.MarkOrientation
// answers from the resolved scales — it has to, because the domain it
// feeds is an input to building those scales. The rule is the same:
// an explicit mark.orient wins, otherwise a discrete y against a
// non-discrete x reads horizontally and everything else reads
// vertically.
func progressMeasureIsX(def *spec.MarkDef, enc *spec.Encoding) bool {
	if def != nil {
		switch def.Orient {
		case string(marksOrientHorizontal):
			return true
		case string(marksOrientVertical):
			return false
		}
	}
	if enc == nil {
		return false
	}
	return channelIsDiscrete(enc.Y) && !channelIsDiscrete(enc.X)
}

// marksOrientHorizontal / marksOrientVertical mirror the orientation
// vocabulary in encode/marks/orient.go. They are declared as local
// constants rather than imported so this file stays free of a
// marks-package dependency it needs for nothing else.
const (
	marksOrientVertical   = "vertical"
	marksOrientHorizontal = "horizontal"
)

// channelIsDiscrete reports whether ch resolves through a band scale —
// the only band-capable scale Prism builds. A nominal / ordinal channel
// type takes the band path by inference; an explicit scale.type of
// "band" takes it directly.
func channelIsDiscrete(ch *spec.PositionChannel) bool {
	if ch == nil || ch.Field == "" {
		return false
	}
	if ch.Scale != nil && ch.Scale.Type != "" {
		return ch.Scale.Type == string(scene.ScaleBand)
	}
	return ch.Type == "nominal" || ch.Type == "ordinal"
}

// progressMeasureExtras returns the extra measure-axis domain values a
// progress mark needs so its track never runs past the plot edge.
//
// A literal mark.total contributes one bound; a field name contributes
// every row's value, because a progress total is read per row (each
// metric may carry its own maximum). An absent total contributes
// nothing — the track then runs to the data-derived domain max, which
// already fits.
func progressMeasureExtras(def *spec.MarkDef, tbl *table.Table) []any {
	if def == nil || tbl == nil {
		return nil
	}
	switch t := def.Total.(type) {
	case nil:
		return nil
	case string:
		if t == "" {
			return nil
		}
		col, ok := tbl.Column(t)
		if !ok {
			return nil
		}
		extras := make([]any, 0, col.Len())
		for i := 0; i < col.Len(); i++ {
			if v, ok := scale.ToFloat(col.ValueAt(i)); ok {
				extras = append(extras, v)
			}
		}
		return extras
	default:
		if v, ok := scale.ToFloat(def.Total); ok {
			return []any{v}
		}
		return nil
	}
}
