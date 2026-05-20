package marks

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// fmtFloat formats with 3-decimal precision (matches render.FormatFloat
// rounding) without pulling the render package into encode/marks.
func fmtFloat(v float64) string {
	return strconv.FormatFloat(roundTo(v, 3), 'f', -1, 64)
}

func roundTo(v float64, decimals int) float64 {
	if v < 0 {
		return -roundTo(-v, decimals)
	}
	p := 1.0
	for i := 0; i < decimals; i++ {
		p *= 10
	}
	return float64(int64(v*p+0.5)) / p
}

// SankeyNode is one node in the sankey diagram. Exposed for parity
// tests (TestPrismSankeyLayout in sankey_test.go).
type SankeyNode struct {
	ID      string
	Depth   int
	Column  int
	Rank    int     // vertical rank within column (0-based)
	X       float64 // top-left pixel
	Y       float64
	W       float64
	H       float64
	Inflow  float64
	Outflow float64
}

// SankeyLink is one link between two nodes. Source/Target are node
// IDs; SourceY / TargetY are the y-pixel offsets within the source
// and target nodes (cumulative-flow stacking).
type SankeyLink struct {
	Source string
	Target string
	Value  float64
	SX     float64 // source-side x (right edge of source node)
	SY     float64 // source-side y (center of link's slice within source)
	TX     float64 // target-side x (left edge of target node)
	TY     float64 // target-side y (center of link's slice within target)
	Width  float64 // visual stroke width = Value * heightScale
}

// SankeyLayout bundles the encoder's layout output. Exposed for the
// PHASE.md gate to hand-verify deterministic positions.
type SankeyLayout struct {
	Nodes []SankeyNode
	Links []SankeyLink
}

// Sankey layout geometry constants (D065).
const (
	sankeyNodeWidth   = 12.0
	sankeyNodePadding = 4.0
)

// encodeSankey emits one RectGeom per node + one PathGeom per link
// using the depth-first relaxation algorithm documented in D065.
//
// Place-once heuristic: column = node depth (longest source-only-to-
// node path); within-column ordering by total flow (inflow+outflow)
// descending; vertical stacking proportional to flow; cubic-bezier
// links with control points at midpoint x.
//
// Cycles are rejected with PRISM_ENCODE_001 (sankey graphs must be
// DAGs).
func encodeSankey(in Inputs) ([]scene.Mark, error) {
	layout, err := ComputeSankeyLayout(in)
	if err != nil {
		return nil, err
	}

	out := make([]scene.Mark, 0, len(layout.Nodes)+len(layout.Links))

	// Emit one RectGeom per node.
	for _, n := range layout.Nodes {
		style := in.Style
		if in.Color != nil {
			c := lookupCategoryColor(n.ID, in.Color.Categories, in.Color.Palette)
			if c != nil {
				style.Fill = c
			}
		}
		out = append(out, scene.Mark{
			Type:  scene.MarkRect,
			ID:    fmt.Sprintf("sankey-node-%s", n.ID),
			Style: style,
			Rect: &scene.RectGeom{
				X: n.X,
				Y: n.Y,
				W: n.W,
				H: n.H,
			},
		})
	}

	// Emit one PathGeom per link (cubic bezier).
	for _, l := range layout.Links {
		style := in.Style
		// Links inherit source node's color when color is bound on
		// the source field.
		if in.Color != nil {
			c := lookupCategoryColor(l.Source, in.Color.Categories, in.Color.Palette)
			if c != nil {
				// Use stroke colour for the link path; fill = none.
				style.Stroke = c
				style.Fill = nil
				style.StrokeWidth = l.Width
				style.Opacity = 0.4
			}
		} else {
			style.Stroke = in.Style.Fill
			style.Fill = nil
			style.StrokeWidth = l.Width
			style.Opacity = 0.4
		}
		mid := (l.SX + l.TX) / 2
		d := fmt.Sprintf("M%s,%s C%s,%s %s,%s %s,%s",
			fmtFloat(l.SX), fmtFloat(l.SY),
			fmtFloat(mid), fmtFloat(l.SY),
			fmtFloat(mid), fmtFloat(l.TY),
			fmtFloat(l.TX), fmtFloat(l.TY))
		out = append(out, scene.Mark{
			Type:  scene.MarkPath,
			ID:    fmt.Sprintf("sankey-link-%s-%s", l.Source, l.Target),
			Style: style,
			Path:  &scene.PathGeom{D: d},
		})
	}
	return out, nil
}

// ComputeSankeyLayout runs the place-once layout algorithm. Exposed
// for the PHASE.md test gate so positions can be hand-verified.
func ComputeSankeyLayout(in Inputs) (*SankeyLayout, error) {
	if in.Source.Field == "" || in.Target.Field == "" || in.Value.Field == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"sankey mark requires source, target, and value channel bindings.",
			map[string]any{"Field": "<source|target|value>", "Source": "<encoding>", "Available": joinFieldNames(in.Table)},
		)
	}

	sources, err := readField(in.Table, in.Source.Field)
	if err != nil {
		return nil, err
	}
	targets, err := readField(in.Table, in.Target.Field)
	if err != nil {
		return nil, err
	}
	values, err := readField(in.Table, in.Value.Field)
	if err != nil {
		return nil, err
	}
	n := len(sources)
	if len(targets) != n || len(values) != n {
		return nil, fmt.Errorf("encodeSankey: column length mismatch (s=%d t=%d v=%d)", n, len(targets), len(values))
	}
	if n == 0 {
		return &SankeyLayout{}, nil
	}

	// Coerce values to float64; canonicalise source/target IDs to strings.
	srcs := make([]string, n)
	tgts := make([]string, n)
	vals := make([]float64, n)
	for i := 0; i < n; i++ {
		srcs[i] = fmt.Sprintf("%v", sources[i])
		tgts[i] = fmt.Sprintf("%v", targets[i])
		f, ok := toFloat64(values[i])
		if !ok {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("sankey value at row %d is not numeric (got %T).", i, values[i]),
				map[string]any{"Field": in.Value.Field, "Source": "<value>", "Available": "numeric"},
			)
		}
		if f < 0 {
			return nil, prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("sankey value at row %d is negative (%g); flows must be non-negative.", i, f),
				map[string]any{"Field": in.Value.Field, "Source": "<value>", "Available": "non-negative"},
			)
		}
		vals[i] = f
	}

	// Build node table in first-seen order across (sources, targets).
	nodeIdx := map[string]int{}
	var order []string
	for i := 0; i < n; i++ {
		for _, name := range []string{srcs[i], tgts[i]} {
			if _, ok := nodeIdx[name]; !ok {
				nodeIdx[name] = len(order)
				order = append(order, name)
			}
		}
	}
	N := len(order)

	// Adjacency: out-edges per node + per-node total inflow/outflow.
	type edge struct {
		toIdx int
		val   float64
		row   int // back-ref into srcs/tgts/vals
	}
	outEdges := make([][]edge, N)
	inEdges := make([][]edge, N)
	inflow := make([]float64, N)
	outflow := make([]float64, N)
	for i := 0; i < n; i++ {
		si := nodeIdx[srcs[i]]
		ti := nodeIdx[tgts[i]]
		outEdges[si] = append(outEdges[si], edge{toIdx: ti, val: vals[i], row: i})
		inEdges[ti] = append(inEdges[ti], edge{toIdx: si, val: vals[i], row: i})
		outflow[si] += vals[i]
		inflow[ti] += vals[i]
	}

	// Depth via longest-path DAG traversal. Detect cycles by tracking
	// visit colours.
	depth := make([]int, N)
	state := make([]int, N) // 0=white, 1=gray (in stack), 2=black (done)
	var dfs func(u int) error
	dfs = func(u int) error {
		if state[u] == 1 {
			return prismerrors.New(
				"PRISM_ENCODE_001",
				fmt.Sprintf("sankey graph contains a cycle involving node %q.", order[u]),
				map[string]any{"Field": "<sankey>", "Source": "<graph>", "Available": "DAG required"},
			)
		}
		if state[u] == 2 {
			return nil
		}
		state[u] = 1
		for _, e := range outEdges[u] {
			if err := dfs(e.toIdx); err != nil {
				return err
			}
		}
		state[u] = 2
		return nil
	}
	for i := 0; i < N; i++ {
		if state[i] == 0 {
			if err := dfs(i); err != nil {
				return nil, err
			}
		}
	}
	// Compute longest-path-from-source depth: source-only nodes (no
	// in-edges) get depth 0; other nodes get max(predecessor depth) + 1.
	// Iterate in topological-equivalent order using a queue based on
	// in-degree (Kahn's algorithm).
	indeg := make([]int, N)
	for i := 0; i < N; i++ {
		indeg[i] = len(inEdges[i])
	}
	queue := []int{}
	for i := 0; i < N; i++ {
		if indeg[i] == 0 {
			queue = append(queue, i)
			depth[i] = 0
		}
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, e := range outEdges[u] {
			if depth[u]+1 > depth[e.toIdx] {
				depth[e.toIdx] = depth[u] + 1
			}
			indeg[e.toIdx]--
			if indeg[e.toIdx] == 0 {
				queue = append(queue, e.toIdx)
			}
		}
	}
	totalCols := 0
	for _, d := range depth {
		if d+1 > totalCols {
			totalCols = d + 1
		}
	}
	if totalCols < 1 {
		totalCols = 1
	}

	// Group node indices by column.
	cols := make([][]int, totalCols)
	for i := 0; i < N; i++ {
		cols[depth[i]] = append(cols[depth[i]], i)
	}

	// Within each column, sort by (inflow + outflow) desc for stable
	// visual stacking.
	for c := 0; c < totalCols; c++ {
		col := cols[c]
		sort.SliceStable(col, func(a, b int) bool {
			ta := inflow[col[a]] + outflow[col[a]]
			tb := inflow[col[b]] + outflow[col[b]]
			return ta > tb
		})
	}

	// Compute heightScale = (plot.H - maxNodesInAnyCol*padding) /
	// maxColumnTotal (per-column flow). Per D065 we use the per-column
	// maximum so the tallest column fits.
	maxColumnFlow := 0.0
	maxNodesInCol := 0
	for c := 0; c < totalCols; c++ {
		colFlow := 0.0
		for _, idx := range cols[c] {
			t := inflow[idx]
			if outflow[idx] > t {
				t = outflow[idx]
			}
			colFlow += t
		}
		if colFlow > maxColumnFlow {
			maxColumnFlow = colFlow
		}
		if len(cols[c]) > maxNodesInCol {
			maxNodesInCol = len(cols[c])
		}
	}
	heightScale := 1.0
	if maxColumnFlow > 0 {
		availH := in.Layout.H - float64(maxNodesInCol-1)*sankeyNodePadding
		if availH < 1 {
			availH = 1
		}
		heightScale = availH / maxColumnFlow
	}

	// Column spacing.
	colSpacing := 0.0
	if totalCols > 1 {
		colSpacing = (in.Layout.W - float64(totalCols)*sankeyNodeWidth) / float64(totalCols-1)
		if colSpacing < 0 {
			colSpacing = 0
		}
	}

	// Place nodes.
	nodes := make([]SankeyNode, N)
	// Build a reverse map: column-local rank → node idx (for layout pass).
	for c := 0; c < totalCols; c++ {
		cursorY := in.Layout.Y
		for rank, idx := range cols[c] {
			t := inflow[idx]
			if outflow[idx] > t {
				t = outflow[idx]
			}
			h := t * heightScale
			x := in.Layout.X + float64(c)*(sankeyNodeWidth+colSpacing)
			nodes[idx] = SankeyNode{
				ID:      order[idx],
				Depth:   depth[idx],
				Column:  c,
				Rank:    rank,
				X:       x,
				Y:       cursorY,
				W:       sankeyNodeWidth,
				H:       h,
				Inflow:  inflow[idx],
				Outflow: outflow[idx],
			}
			cursorY += h + sankeyNodePadding
		}
	}

	// Place links. For each source node, walk its out-edges in row
	// order and accumulate sourceYOffset; for each target node,
	// accumulate targetYOffset in row order.
	srcOffsets := make([]float64, N) // running offset per source node
	tgtOffsets := make([]float64, N) // running offset per target node
	links := make([]SankeyLink, 0, n)
	for i := 0; i < n; i++ {
		si := nodeIdx[srcs[i]]
		ti := nodeIdx[tgts[i]]
		linkH := vals[i] * heightScale
		sx := nodes[si].X + nodes[si].W
		sy := nodes[si].Y + srcOffsets[si] + linkH/2
		tx := nodes[ti].X
		ty := nodes[ti].Y + tgtOffsets[ti] + linkH/2
		links = append(links, SankeyLink{
			Source: srcs[i],
			Target: tgts[i],
			Value:  vals[i],
			SX:     sx,
			SY:     sy,
			TX:     tx,
			TY:     ty,
			Width:  linkH,
		})
		srcOffsets[si] += linkH
		tgtOffsets[ti] += linkH
	}

	return &SankeyLayout{Nodes: nodes, Links: links}, nil
}
