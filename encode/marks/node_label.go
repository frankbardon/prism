package marks

import (
	"github.com/frankbardon/prism/encode/marks/layout"
	"github.com/frankbardon/prism/encode/scene"
)

// Node labelling for the graph mark family (tree, dendrogram,
// network). E4-S5.
//
// Content comes from the `text` encoding channel and is resolved by
// textContents — the same resolver the text mark uses (E4-S4) — so
// `field`, `value` and `format` behave identically here. Labelling is
// opt-in: with no `text` channel the encoders emit no TextGeom at all
// and their output is byte-identical to the pre-E4-S5 scene.
//
// The graph marks are node-oriented but the text channel is
// row-oriented, so the per-row label is keyed to the node named by
// that row's `target` value — the channel the docs call the node
// identity. First row wins for a repeated target. A node that never
// appears as a target (a pure source: the root of an edge-list tree,
// an uncited paper in a citation network) has no row of its own and
// falls back to its id.

// nodeLabelPadding is the gap in pixels between a node glyph's edge
// and the nearest edge of its label.
const nodeLabelPadding = 4.0

// nodeLabelFontSize matches the text mark's own default and the
// theme's --prism-mark-text-font-size token.
const nodeLabelFontSize = 11.0

// nodeLabelAscent / nodeLabelDescent split a label's line box around
// the alphabetic baseline both renderers place at TextGeom.Y —
// roughly two thirds above it, one third below. They are derived from
// scene.LabelLineHeight so the node-label geometry and the axis label
// heuristics stay tied to one estimate.
//
// scene.TextGeom.Baseline is left unset by the graph encoders on
// purpose: neither render/svg nor the vendored JS renderer honours
// it today, so vertical placement is baked into Y instead. Setting
// both would double-apply the moment a renderer starts reading it.
const (
	nodeLabelAscent  = scene.LabelLineHeight * 2 / 3
	nodeLabelDescent = scene.LabelLineHeight / 3
)

// nodeLabelsBound reports whether the spec asked for node labels,
// i.e. bound the `text` channel. False keeps the graph encoders on
// their historical label-free geometry.
func nodeLabelsBound(in Inputs) bool { return in.Text != nil }

// nodeLabelsByID resolves one label per table row through
// textContents and keys them by node id. ids is the already-read
// `target` column; it is passed as textContents' fallback source too,
// so a text channel carrying only `format` (or an empty one) degrades
// to the node id rather than indexing a nil column.
func nodeLabelsByID(in Inputs, ids []any) (map[string]string, error) {
	contents, err := textContents(in, ids, nil, len(ids))
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(ids))
	for i, raw := range ids {
		id := stringifyAny(raw)
		if id == "" {
			continue
		}
		if _, dup := out[id]; dup {
			continue
		}
		out[id] = contents[i]
	}
	return out, nil
}

// nodeLabelFor returns the label to draw for node id: the row-derived
// label when one exists, else the id verbatim.
func nodeLabelFor(labels map[string]string, id string) string {
	if l, ok := labels[id]; ok && l != "" {
		return l
	}
	return id
}

// maxNodeLabelWidth estimates the widest label in g, in pixels, from
// scene.LabelCharWidth. Prism runs no text-measurement pass, so this
// is an approximation shared with the axis label heuristics.
func maxNodeLabelWidth(g *layout.Graph, labels map[string]string) float64 {
	var max float64
	for _, n := range g.Nodes {
		w := float64(len([]rune(nodeLabelFor(labels, n.ID)))) * scene.LabelCharWidth
		if w > max {
			max = w
		}
	}
	return max
}

// insetForLabels shrinks r by the given per-side reservations so the
// labels drawn around the outermost nodes stay inside the rect the
// encoder was handed. Each side is clamped to 40% of its dimension so
// a long label on a small plot degrades to a cramped chart rather
// than an inverted rect.
func insetForLabels(r scene.Rect, left, right, top, bottom float64) scene.Rect {
	left = clampInset(left, r.W)
	right = clampInset(right, r.W)
	top = clampInset(top, r.H)
	bottom = clampInset(bottom, r.H)
	return scene.Rect{
		X: r.X + left,
		Y: r.Y + top,
		W: r.W - left - right,
		H: r.H - top - bottom,
	}
}

func clampInset(v, extent float64) float64 {
	if v < 0 {
		return 0
	}
	if max := extent * 0.4; v > max {
		return max
	}
	return v
}
