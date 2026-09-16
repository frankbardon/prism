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
//     Style; its colour is theme-derived rather than a constant in the
//     marks package (see marks.Inputs.TrackStyle). E10-S2 replaces the
//     derivation below with a first-class theme token.

// progressTrackFallbackFill is the track colour used only when there is
// no theme at all to derive one from — the same last-resort posture as
// hardcodedDefaultStyle. Any loaded theme, built-in or custom, supplies
// its own value through the grid colour.
const progressTrackFallbackFill = "#e5e7eb"

// progressTrackStyle resolves the Style of a progress mark's unfilled
// track from t.
//
// The track is chrome, not data: it reads as the axis's own ground
// rather than as a second series, so it takes the theme's grid colour —
// the token every built-in theme already tunes for exactly that role
// (light #e5e7eb, dark #374151, print #cccccc). The nested axis block
// wins over the legacy flat field, matching how the rest of the
// encoder reads axis tokens.
func progressTrackStyle(t *theme.Theme) scene.Style {
	hex := progressTrackFallbackFill
	if t != nil {
		if t.Axis != nil && t.Axis.GridColor != "" {
			hex = t.Axis.GridColor
		} else if t.GridColor != "" {
			hex = t.GridColor
		}
	}
	style := scene.Style{}
	if c, err := scene.ColorFromHex(hex); err == nil {
		style.Fill = c
	}
	return style
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
