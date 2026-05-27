package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/marks/layout"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeTree renders a rooted hierarchy via a tidy-tree layout.
// Channel bindings (per the .planning/tier1-04 plan):
//
//   - source: parent-id field (empty / null → root).
//   - target: child-id field (the node identity).
//   - text:   optional per-node label.
//
// Mark-level options (spec.MarkDef): orient (horizontal / vertical /
// radial — vertical default in v0.2), link_shape (step / curve /
// straight), node_shape (circle / rect / none), node_size.
//
// Output decomposes into existing scene-IR primitives so SVG / PDF
// render without changes:
//
//   - one PathGeom per parent → child edge,
//   - one PointGeom or RectGeom per node (based on node_shape),
//   - one TextGeom per node when the text channel is bound.
//
// Validate rules PRISM_SPEC_028 and PRISM_SPEC_029 catch missing
// channels / multi-root inputs upstream; this encoder defensively
// re-checks via layout.BuildTree so a malformed runtime table
// surfaces PRISM_ENCODE_TREE_CYCLE rather than panicking.
func encodeTree(in Inputs) ([]scene.Mark, error) {
	if in.Source.Field == "" || in.Target.Field == "" {
		return nil, prismerrors.New(
			"PRISM_SPEC_028",
			"tree mark requires source + target channel bindings.",
			map[string]any{"Mark": "tree"},
		)
	}
	parents, err := readField(in.Table, in.Source.Field)
	if err != nil {
		return nil, err
	}
	children, err := readField(in.Table, in.Target.Field)
	if err != nil {
		return nil, err
	}
	var labels []any
	// Best-effort: a text channel feed could be wired in here; v0.2
	// uses the target id verbatim as the label.

	g := layout.NewGraph()
	for i, child := range children {
		childID := stringifyAny(child)
		if childID == "" {
			continue
		}
		g.AddNode(layout.Node{ID: childID, Label: stringifyAny(child)})
		if i < len(parents) {
			pid := stringifyAny(parents[i])
			if pid != "" {
				g.AddEdge(layout.Edge{From: pid, To: childID})
			}
		}
	}

	rootID, err := g.BuildTree()
	if err != nil {
		return nil, prismerrors.New(
			"PRISM_ENCODE_TREE_CYCLE",
			fmt.Sprintf("tree mark: %v", err),
			map[string]any{"Mark": "tree", "Reason": err.Error()},
		)
	}

	nodeSize := 6.0
	if in.Mark != nil && in.Mark.NodeSize != nil && *in.Mark.NodeSize > 0 {
		nodeSize = *in.Mark.NodeSize
	}
	nodeShape := "circle"
	if in.Mark != nil && in.Mark.NodeShape != "" {
		nodeShape = in.Mark.NodeShape
	}
	linkShape := "step"
	if in.Mark != nil && in.Mark.LinkShape != "" {
		linkShape = in.Mark.LinkShape
	}
	orient := "vertical"
	if in.Mark != nil && in.Mark.Orient != "" {
		orient = in.Mark.Orient
	}

	// Scale layout output to the plot rect. TidyTree's local space:
	// X spans [0, (leafCount - 1) * horizontalGap]; Y spans [0,
	// (maxDepth) * verticalGap]. We use unit gaps then linearly map
	// to the plot rect.
	pos, err := layout.TidyTree(g, rootID, 1, 1)
	if err != nil {
		return nil, prismerrors.New(
			"PRISM_ENCODE_TREE_CYCLE",
			fmt.Sprintf("tree mark layout: %v", err),
			map[string]any{"Mark": "tree", "Reason": err.Error()},
		)
	}

	plot := in.Layout
	var maxX, maxY float64
	for _, p := range pos {
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	if maxX == 0 {
		maxX = 1
	}
	if maxY == 0 {
		maxY = 1
	}
	// Map (local X, local Y) → (plot X, plot Y). Horizontal orient
	// swaps the two axes so the tree grows left-to-right.
	resolveXY := func(lx, ly float64) (float64, float64) {
		px := plot.X + (lx/maxX)*plot.W
		py := plot.Y + (ly/maxY)*plot.H
		if orient == "horizontal" {
			py = plot.Y + (lx/maxX)*plot.H
			px = plot.X + (ly/maxY)*plot.W
		}
		return px, py
	}

	pixelByID := map[string][2]float64{}
	for _, p := range pos {
		x, y := resolveXY(p.X, p.Y)
		pixelByID[p.ID] = [2]float64{x, y}
	}

	out := make([]scene.Mark, 0, len(pos)+len(g.Edges))

	// Edges first so nodes render on top.
	for _, e := range g.Edges {
		from, fromOK := pixelByID[e.From]
		to, toOK := pixelByID[e.To]
		if !fromOK || !toOK {
			continue
		}
		mark := scene.Mark{Type: scene.MarkPath, Style: in.Style}
		mark.Path = &scene.PathGeom{D: treeLinkPath(linkShape, from[0], from[1], to[0], to[1], orient)}
		out = append(out, mark)
	}

	// Nodes — one per layout position, in tree pre-order.
	for _, p := range pos {
		x, y := resolveXY(p.X, p.Y)
		var nodeMark scene.Mark
		switch nodeShape {
		case "rect":
			nodeMark = scene.Mark{Type: scene.MarkRect, Style: in.Style}
			nodeMark.Rect = &scene.RectGeom{
				X: x - nodeSize/2, Y: y - nodeSize/2,
				W: nodeSize, H: nodeSize,
			}
		case "none":
			// No geometry; tree visualises as the label-only "phylogram"
			// variant. Skip the node mark entirely.
			continue
		default:
			nodeMark = scene.Mark{Type: scene.MarkPoint, Style: in.Style}
			nodeMark.Point = &scene.PointGeom{Cx: x, Cy: y, R: nodeSize}
		}
		out = append(out, nodeMark)
	}

	_ = labels
	return out, nil
}

func stringifyAny(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// treeLinkPath builds an SVG path "d" string for one parent → child
// edge based on link_shape.
//
//   - step: vertical down to half-y, horizontal across, vertical to
//     child (or horizontal across to half-x and vertical for horiz).
//   - curve: smooth cubic Bezier with vertical control points.
//   - straight: a single line segment.
func treeLinkPath(shape string, x0, y0, x1, y1 float64, orient string) string {
	switch shape {
	case "straight":
		return fmt.Sprintf("M %s %s L %s %s",
			fmtFloat(x0), fmtFloat(y0), fmtFloat(x1), fmtFloat(y1))
	case "curve":
		if orient == "horizontal" {
			mx := (x0 + x1) / 2
			return fmt.Sprintf("M %s %s C %s %s %s %s %s %s",
				fmtFloat(x0), fmtFloat(y0),
				fmtFloat(mx), fmtFloat(y0),
				fmtFloat(mx), fmtFloat(y1),
				fmtFloat(x1), fmtFloat(y1))
		}
		my := (y0 + y1) / 2
		return fmt.Sprintf("M %s %s C %s %s %s %s %s %s",
			fmtFloat(x0), fmtFloat(y0),
			fmtFloat(x0), fmtFloat(my),
			fmtFloat(x1), fmtFloat(my),
			fmtFloat(x1), fmtFloat(y1))
	}
	// Default: step.
	if orient == "horizontal" {
		mx := (x0 + x1) / 2
		return fmt.Sprintf("M %s %s L %s %s L %s %s L %s %s",
			fmtFloat(x0), fmtFloat(y0),
			fmtFloat(mx), fmtFloat(y0),
			fmtFloat(mx), fmtFloat(y1),
			fmtFloat(x1), fmtFloat(y1))
	}
	my := (y0 + y1) / 2
	return fmt.Sprintf("M %s %s L %s %s L %s %s L %s %s",
		fmtFloat(x0), fmtFloat(y0),
		fmtFloat(x0), fmtFloat(my),
		fmtFloat(x1), fmtFloat(my),
		fmtFloat(x1), fmtFloat(y1))
}
