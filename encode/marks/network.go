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
//
// Mark-def options: node_shape, node_size, iterations, link_distance,
// charge, seed.
//
// Output decomposes into LineGeom (one per edge) + Point/Rect geoms
// (one per unique node). The SVG / PDF renderers handle both
// primitives without changes.
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

	opts := layout.ForceOpts{Width: in.Layout.W, Height: in.Layout.H}
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

	plotX, plotY := in.Layout.X, in.Layout.Y
	pixelByID := map[string][2]float64{}
	for _, p := range pos {
		pixelByID[p.ID] = [2]float64{plotX + p.X, plotY + p.Y}
	}

	nodeSize := 6.0
	if in.Mark != nil && in.Mark.NodeSize != nil && *in.Mark.NodeSize > 0 {
		nodeSize = *in.Mark.NodeSize
	}
	nodeShape := "circle"
	if in.Mark != nil && in.Mark.NodeShape != "" {
		nodeShape = in.Mark.NodeShape
	}

	out := make([]scene.Mark, 0, len(pos)+g.EdgeCount())

	// Edges as straight lines.
	for _, e := range g.Edges {
		from, fromOK := pixelByID[e.From]
		to, toOK := pixelByID[e.To]
		if !fromOK || !toOK {
			continue
		}
		mark := scene.Mark{Type: scene.MarkLine, Style: in.Style}
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
	return out, nil
}
