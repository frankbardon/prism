package marks

import (
	"github.com/frankbardon/prism/encode/marks/layout"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeNetwork renders an undirected / directed node-link diagram
// via the Fruchterman-Reingold force layout from
// encode/marks/layout. The layout is deterministic for a fixed seed
// (default 42) so SVG goldens stay byte-stable across runs.
//
// Channel bindings:
//   - source: from-node id field
//   - target: to-node id field
//   - value:  optional edge weight (drives link stroke width)
//   - text:   optional per-node label (E4-S5)
//
// Mark-def options: node_shape, node_size, iterations, link_distance,
// charge, seed.
//
// Output decomposes into LineGeom (one per edge) + Point/Rect geoms
// (one per unique node) + one TextGeom per node when the text channel
// is bound. The SVG renderer handles all three primitives without
// changes.
//
// Labels are opt-in on the text channel: with none bound the output
// is byte-identical to the pre-E4-S5 scene. A network row is an
// *edge*, not a node, so the row label binds to that row's `target`
// node (first row wins); a node that only ever appears as a source
// has no row of its own and falls back to its id. There is no
// leaf / internal distinction in a force layout and no growth
// direction to follow, so every label sits directly under its node,
// centred.
func encodeNetwork(in Inputs) ([]scene.Mark, error) {
	if in.Source.Field == "" || in.Target.Field == "" {
		return nil, prismerrors.New(
			"PRISM_SPEC_028",
			"network mark requires source + target channel bindings.",
			map[string]any{"Mark": "network"},
		)
	}
	fromVals, err := readField(in.Table, in.Source.Field)
	if err != nil {
		return nil, err
	}
	toVals, err := readField(in.Table, in.Target.Field)
	if err != nil {
		return nil, err
	}

	g := layout.NewGraph()
	for i := 0; i < len(fromVals); i++ {
		from := stringifyAny(fromVals[i])
		to := stringifyAny(toVals[i])
		if from == "" || to == "" {
			continue
		}
		g.AddEdge(layout.Edge{From: from, To: to})
	}

	labelsOn := nodeLabelsBound(in)
	var labelByID map[string]string
	if labelsOn {
		if labelByID, err = nodeLabelsByID(in, toVals); err != nil {
			return nil, err
		}
	}

	nodeSize := 6.0
	if in.Mark != nil && in.Mark.NodeSize != nil && *in.Mark.NodeSize > 0 {
		nodeSize = *in.Mark.NodeSize
	}
	nodeShape := "circle"
	if in.Mark != nil && in.Mark.NodeShape != "" {
		nodeShape = in.Mark.NodeShape
	}
	labelOffset := nodeSize + nodeLabelPadding
	if nodeShape == "none" {
		labelOffset = nodeLabelPadding
	}

	// The force layout fills the rect it is handed, so a node can land
	// flush against any edge. Shrink that rect by the label band when
	// labels are on, otherwise the outermost labels fall off the
	// canvas. Width is estimated from scene.LabelCharWidth — Prism
	// has no text measurement pass, and sibling labels on nodes that
	// the layout happens to place close together are not collision
	// tested against each other.
	plot := in.Layout
	if labelsOn {
		w := maxNodeLabelWidth(g, labelByID)
		// Every label hangs below its node, so only the bottom needs a
		// full line box; the top needs nothing.
		plot = insetForLabels(plot, w/2, w/2, 0,
			labelOffset+nodeLabelAscent+nodeLabelDescent)
	}

	opts := layout.ForceOpts{Width: plot.W, Height: plot.H}
	if in.Mark != nil {
		if in.Mark.Iterations != nil && *in.Mark.Iterations > 0 {
			opts.Iterations = *in.Mark.Iterations
		}
		if in.Mark.LinkDistance != nil && *in.Mark.LinkDistance > 0 {
			opts.LinkDistance = *in.Mark.LinkDistance
		}
		if in.Mark.Charge != nil {
			opts.Charge = *in.Mark.Charge
		}
		if in.Mark.Seed != nil {
			opts.Seed = *in.Mark.Seed
		}
	}

	pos, err := layout.ForceLayout(g, opts)
	if err != nil {
		return nil, prismerrors.New(
			"PRISM_ENCODE_NETWORK_NONFINITE",
			"network mark: "+err.Error(),
			map[string]any{"Mark": "network", "Reason": err.Error()},
		)
	}

	plotX, plotY := plot.X, plot.Y
	pixelByID := map[string][2]float64{}
	for _, p := range pos {
		pixelByID[p.ID] = [2]float64{plotX + p.X, plotY + p.Y}
	}

	out := make([]scene.Mark, 0, 2*len(pos)+g.EdgeCount())

	// Edges carry Style.Stroke but must NOT inherit in.Style's Fill:
	// in.Style is shared with the node marks below (Point/Rect, which
	// do need Fill), the same one-Style-two-geometries reuse tree.go
	// uses for its links/nodes. render/svg's renderLine already
	// hardcodes fill="none" on every polyline it emits, so a non-nil
	// Style.Fill here doesn't change what's painted — but it does
	// produce a second, duplicate "fill" attribute on the emitted
	// <polyline> (invalid SVG). Clear it on a local copy so the edge
	// mark only ever carries Stroke.
	edgeStyle := in.Style
	edgeStyle.Fill = nil
	// Edges as straight lines.
	for _, e := range g.Edges {
		from, fromOK := pixelByID[e.From]
		to, toOK := pixelByID[e.To]
		if !fromOK || !toOK {
			continue
		}
		mark := scene.Mark{Type: scene.MarkLine, Style: edgeStyle}
		mark.Line = &scene.LineGeom{Points: [][2]float64{{from[0], from[1]}, {to[0], to[1]}}}
		out = append(out, mark)
	}

	// Nodes.
	for _, p := range pos {
		x, y := plotX+p.X, plotY+p.Y
		var nodeMark scene.Mark
		switch nodeShape {
		case "rect":
			nodeMark = scene.Mark{Type: scene.MarkRect, Style: in.Style}
			nodeMark.Rect = &scene.RectGeom{
				X: x - nodeSize/2, Y: y - nodeSize/2,
				W: nodeSize, H: nodeSize,
			}
		case "none":
			continue
		default:
			nodeMark = scene.Mark{Type: scene.MarkPoint, Style: in.Style}
			nodeMark.Point = &scene.PointGeom{Cx: x, Cy: y, R: nodeSize}
		}
		out = append(out, nodeMark)
	}

	// Node labels last so they paint over both edges and glyphs. The
	// vertical offset is baked into Y because neither renderer honours
	// scene.TextGeom.Baseline — see nodeLabelAscent.
	if labelsOn {
		for _, p := range pos {
			out = append(out, scene.Mark{
				Type:  scene.MarkText,
				ID:    "network-label-" + p.ID,
				Style: in.LabelStyle,
				Text: &scene.TextGeom{
					X:        plotX + p.X,
					Y:        plotY + p.Y + labelOffset + nodeLabelAscent,
					Content:  nodeLabelFor(labelByID, p.ID),
					Anchor:   scene.AnchorMiddle,
					FontSize: nodeLabelFontSize,
				},
			})
		}
	}
	return out, nil
}
