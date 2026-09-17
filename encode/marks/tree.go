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
//   - text:   optional per-node label (E4-S5). Opt-in: with no text
//     channel no label marks are emitted at all.
//
// Mark-level options (spec.MarkDef): orient (horizontal / vertical /
// radial — vertical default in v0.2), link_shape (step / curve /
// straight), node_shape (circle / rect / none), node_size.
//
// Output decomposes into existing scene-IR primitives so SVG
// renders without changes:
//
//   - one PathGeom per parent → child edge,
//   - one PointGeom or RectGeom per node (based on node_shape),
//   - one TextGeom per node when the text channel is bound. Labels
//     are emitted for every node, including the node_shape: "none"
//     case (the dendrogram default) where there is no glyph to skip.
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
	// Node labels (E4-S5). The comment that stood here claimed v0.2
	// used the target id verbatim as the label; it did not — the
	// encoder emitted no TextGeom at all, so this function's own
	// "one TextGeom per node" line and the docs' "text — optional
	// per-node label" were both describing a feature that did not
	// exist. Labels now resolve through textContents (the text mark's
	// own resolver) keyed by target id, and stay opt-in on the
	// channel.
	labelsOn := nodeLabelsBound(in)
	var labelByID map[string]string
	if labelsOn {
		if labelByID, err = nodeLabelsByID(in, children); err != nil {
			return nil, err
		}
	}

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

	// Label offset from the node's centre, and the matching inset on
	// the plot rect so the outermost labels (the root's, and the leaf
	// row's) land inside it rather than off the canvas. Width is
	// estimated from scene.LabelCharWidth — Prism has no text
	// measurement pass.
	labelOffset := nodeSize + nodeLabelPadding
	if nodeShape == "none" {
		labelOffset = nodeLabelPadding
	}
	plot := in.Layout
	if labelsOn {
		w := maxNodeLabelWidth(g, labelByID)
		if orient == "horizontal" {
			// Labels run outward from the node on the x axis and are
			// vertically centred on it.
			plot = insetForLabels(plot, w+labelOffset, w+labelOffset,
				nodeLabelAscent-nodeLabelDescent, nodeLabelDescent+nodeLabelDescent)
		} else {
			// Labels are centred on the node's x and sit a full
			// ascent above it (internal) / a full line below it (leaf).
			plot = insetForLabels(plot, w/2, w/2,
				labelOffset+nodeLabelAscent,
				labelOffset+nodeLabelAscent+nodeLabelDescent)
		}
	}
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

	// Capacity: one mark per edge, per node, and (when labels are on)
	// per label.
	out := make([]scene.Mark, 0, 2*len(pos)+len(g.Edges))

	// Links carry Style.Stroke (for the line itself) but must NOT
	// inherit in.Style's Fill: in.Style is shared with the node marks
	// below (Point/Rect, which do need Fill), the same one-Style-two-
	// geometries reuse network.go uses. render/svg's renderPath —
	// unlike renderLine — does not hardcode fill="none" on every
	// element it emits, so a non-nil Fill here would paint the link's
	// open multi-segment `d=` path as a solid filled blob rather than
	// a thin stroked line. Clear it on a local copy; renderPath treats
	// a nil Style.Fill as an explicit "no fill" (mirroring renderLine).
	linkStyle := in.Style
	linkStyle.Fill = nil
	// Edges first so nodes render on top.
	for _, e := range g.Edges {
		from, fromOK := pixelByID[e.From]
		to, toOK := pixelByID[e.To]
		if !fromOK || !toOK {
			continue
		}
		mark := scene.Mark{Type: scene.MarkPath, Style: linkStyle}
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

	// Node labels last so they paint over both links and glyphs.
	// Placement follows the classic tidy-tree convention: a label sits
	// on the far side of its node from that node's subtree, so it can
	// never collide with the links or children below / beside it.
	// Vertical orient (the default, tree grows downwards): internal
	// nodes label above, leaves label below, both centred. Horizontal
	// orient (tree grows rightwards): internal nodes label to the
	// left (anchor end), leaves to the right (anchor start), both
	// vertically centred.
	//
	// Known limitation: sibling labels within one depth row are not
	// collision-tested against each other. Prism runs no text
	// measurement pass, so the only honest input is the
	// scene.LabelCharWidth estimate already spent on the plot inset;
	// hiding a node's name (the axis overlap heuristic's answer)
	// loses information a tree label cannot afford to lose.
	if labelsOn {
		for _, p := range pos {
			x, y := resolveXY(p.X, p.Y)
			geom := &scene.TextGeom{
				X:        x,
				Y:        y,
				Content:  nodeLabelFor(labelByID, p.ID),
				FontSize: nodeLabelFontSize,
			}
			leaf := len(g.Children(p.ID)) == 0
			if orient == "horizontal" {
				// Nudge the alphabetic baseline down so the glyph
				// reads as vertically centred on the node — the same
				// nudge render/svg/axes.go applies to left / right
				// axis tick labels.
				geom.Y = y + nodeLabelDescent
				if leaf {
					geom.Anchor = scene.AnchorStart
					geom.X = x + labelOffset
				} else {
					geom.Anchor = scene.AnchorEnd
					geom.X = x - labelOffset
				}
			} else {
				geom.Anchor = scene.AnchorMiddle
				if leaf {
					geom.Y = y + labelOffset + nodeLabelAscent
				} else {
					geom.Y = y - labelOffset
				}
			}
			out = append(out, scene.Mark{
				Type:  scene.MarkText,
				ID:    "tree-label-" + p.ID,
				Style: in.LabelStyle,
				Text:  geom,
			})
		}
	}
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
