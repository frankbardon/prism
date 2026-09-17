package marks

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// orgTable is the four-node hierarchy the label tests share:
//
//	ceo (root, internal)
//	├── cto (internal)
//	│   └── vp  (leaf)
//	└── cfo (leaf)
func orgTable(t *testing.T) *scene.Rect {
	t.Helper()
	r := plotRect()
	return &r
}

func orgInputs(t *testing.T, text *spec.TextChannel, def *spec.MarkDef) Inputs {
	t.Helper()
	return Inputs{
		Table: buildTable(t, map[string]any{
			"parent": []string{"", "ceo", "cto", "ceo"},
			"id":     []string{"ceo", "cto", "vp", "cfo"},
			"name":   []string{"Chief", "Tech", "Veep", "Finance"},
			"score":  []float64{1.25, 2.5, 3.75, 4},
		}),
		Source: Channel{Field: "parent"},
		Target: Channel{Field: "id"},
		Layout: *orgTable(t),
		Text:   text,
		Mark:   def,
	}
}

func textMarks(marks []scene.Mark) []scene.Mark {
	var out []scene.Mark
	for _, m := range marks {
		if m.Type == scene.MarkText && m.Text != nil {
			out = append(out, m)
		}
	}
	return out
}

func labelByNode(marks []scene.Mark, prefix string) map[string]*scene.TextGeom {
	out := map[string]*scene.TextGeom{}
	for _, m := range marks {
		if m.Type != scene.MarkText || m.Text == nil {
			continue
		}
		if len(m.ID) > len(prefix) && m.ID[:len(prefix)] == prefix {
			out[m.ID[len(prefix):]] = m.Text
		}
	}
	return out
}

func geomCenter(m scene.Mark) (float64, float64, bool) {
	switch {
	case m.Point != nil:
		return m.Point.Cx, m.Point.Cy, true
	case m.Rect != nil:
		return m.Rect.X + m.Rect.W/2, m.Rect.Y + m.Rect.H/2, true
	}
	return 0, 0, false
}

func nodeCenters(marks []scene.Mark) [][2]float64 {
	var out [][2]float64
	for _, m := range marks {
		if x, y, ok := geomCenter(m); ok {
			out = append(out, [2]float64{x, y})
		}
	}
	return out
}

// A tree with no text channel emits no label marks at all and keeps
// its historical geometry — the plot rect is not inset.
func TestPrismTreeWithoutTextChannelEmitsNoLabels(t *testing.T) {
	in := orgInputs(t, nil, nil)
	marks, _, err := Encode("tree", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if got := len(textMarks(marks)); got != 0 {
		t.Fatalf("label marks = %d, want 0 with no text channel", got)
	}
	// Root sits flush against the top of the un-inset plot rect.
	centers := nodeCenters(marks)
	if len(centers) != 4 {
		t.Fatalf("node marks = %d, want 4", len(centers))
	}
	if centers[0][1] != in.Layout.Y {
		t.Errorf("root Y = %v, want the un-inset plot top %v", centers[0][1], in.Layout.Y)
	}
}

// The text channel's field supplies node content, keyed by the target
// (node identity) column.
func TestPrismTreeNodeLabelsFromTextChannel(t *testing.T) {
	in := orgInputs(t, &spec.TextChannel{Field: "name", Type: "nominal"}, nil)
	marks, _, err := Encode("tree", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	labels := labelByNode(marks, "tree-label-")
	want := map[string]string{"ceo": "Chief", "cto": "Tech", "vp": "Veep", "cfo": "Finance"}
	if len(labels) != len(want) {
		t.Fatalf("label marks = %d, want %d", len(labels), len(want))
	}
	for id, content := range want {
		got, ok := labels[id]
		if !ok {
			t.Fatalf("no label mark for node %q", id)
		}
		if got.Content != content {
			t.Errorf("label[%s] = %q, want %q", id, got.Content, content)
		}
		if got.FontSize != nodeLabelFontSize {
			t.Errorf("label[%s].FontSize = %v, want %v", id, got.FontSize, nodeLabelFontSize)
		}
	}
}

// format runs through the same encode/format path the text mark uses.
func TestPrismTreeNodeLabelsHonourFormat(t *testing.T) {
	in := orgInputs(t, &spec.TextChannel{Field: "score", Type: "quantitative", Format: ".1f"}, nil)
	marks, _, err := Encode("tree", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	labels := labelByNode(marks, "tree-label-")
	if got := labels["ceo"].Content; got != "1.2" {
		t.Errorf("label[ceo] = %q, want %q", got, "1.2")
	}
	if got := labels["cfo"].Content; got != "4.0" {
		t.Errorf("label[cfo] = %q, want %q", got, "4.0")
	}
}

// A text channel carrying only `value` labels every node with the
// literal; a node with no row of its own falls back to its id.
func TestPrismTreeNodeLabelValueLiteral(t *testing.T) {
	in := orgInputs(t, &spec.TextChannel{Value: "node"}, nil)
	marks, _, err := Encode("tree", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for id, g := range labelByNode(marks, "tree-label-") {
		if g.Content != "node" {
			t.Errorf("label[%s] = %q, want %q", id, g.Content, "node")
		}
	}
}

// Vertical orient (the default): internal nodes label above their
// glyph, leaves below, both centred.
func TestPrismTreeLabelPlacementVertical(t *testing.T) {
	in := orgInputs(t, &spec.TextChannel{Field: "name", Type: "nominal"}, nil)
	marks, _, err := Encode("tree", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	labels := labelByNode(marks, "tree-label-")
	centers := map[string][2]float64{}
	ids := []string{"ceo", "cto", "vp", "cfo"} // tree pre-order
	for i, c := range nodeCenters(marks) {
		centers[ids[i]] = c
	}
	for _, id := range ids {
		g := labels[id]
		if g.Anchor != scene.AnchorMiddle {
			t.Errorf("label[%s].Anchor = %q, want middle", id, g.Anchor)
		}
		if g.X != centers[id][0] {
			t.Errorf("label[%s].X = %v, want the node's X %v", id, g.X, centers[id][0])
		}
	}
	for _, id := range []string{"ceo", "cto"} { // internal
		if labels[id].Y >= centers[id][1] {
			t.Errorf("internal label[%s].Y = %v, want above node Y %v", id, labels[id].Y, centers[id][1])
		}
	}
	for _, id := range []string{"vp", "cfo"} { // leaves
		if labels[id].Y <= centers[id][1] {
			t.Errorf("leaf label[%s].Y = %v, want below node Y %v", id, labels[id].Y, centers[id][1])
		}
	}
}

// Horizontal orient: internal nodes label to the left of their glyph
// (anchor end), leaves to the right (anchor start).
func TestPrismTreeLabelPlacementHorizontal(t *testing.T) {
	in := orgInputs(t, &spec.TextChannel{Field: "name", Type: "nominal"}, &spec.MarkDef{Orient: "horizontal"})
	marks, _, err := Encode("tree", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	labels := labelByNode(marks, "tree-label-")
	centers := map[string][2]float64{}
	ids := []string{"ceo", "cto", "vp", "cfo"}
	for i, c := range nodeCenters(marks) {
		centers[ids[i]] = c
	}
	for _, id := range []string{"ceo", "cto"} { // internal
		if labels[id].Anchor != scene.AnchorEnd {
			t.Errorf("internal label[%s].Anchor = %q, want end", id, labels[id].Anchor)
		}
		if labels[id].X >= centers[id][0] {
			t.Errorf("internal label[%s].X = %v, want left of node X %v", id, labels[id].X, centers[id][0])
		}
	}
	for _, id := range []string{"vp", "cfo"} { // leaves
		if labels[id].Anchor != scene.AnchorStart {
			t.Errorf("leaf label[%s].Anchor = %q, want start", id, labels[id].Anchor)
		}
		if labels[id].X <= centers[id][0] {
			t.Errorf("leaf label[%s].X = %v, want right of node X %v", id, labels[id].X, centers[id][0])
		}
	}
}

// The plot rect is inset when labels are on, so every label's
// estimated box stays inside the rect the encoder was handed.
func TestPrismTreeLabelsStayInsidePlot(t *testing.T) {
	for _, orient := range []string{"", "horizontal"} {
		in := orgInputs(t, &spec.TextChannel{Field: "name", Type: "nominal"}, &spec.MarkDef{Orient: orient})
		marks, _, err := Encode("tree", in)
		if err != nil {
			t.Fatalf("Encode(orient=%q): %v", orient, err)
		}
		for _, m := range textMarks(marks) {
			g := m.Text
			w := float64(len([]rune(g.Content))) * scene.LabelCharWidth
			left, right := g.X-w/2, g.X+w/2
			switch g.Anchor {
			case scene.AnchorStart:
				left, right = g.X, g.X+w
			case scene.AnchorEnd:
				left, right = g.X-w, g.X
			}
			if left < in.Layout.X || right > in.Layout.Right() {
				t.Errorf("orient=%q label %q spans [%v,%v], outside plot [%v,%v]",
					orient, g.Content, left, right, in.Layout.X, in.Layout.Right())
			}
			if g.Y-nodeLabelAscent < in.Layout.Y || g.Y+nodeLabelDescent > in.Layout.Bottom() {
				t.Errorf("orient=%q label %q at Y=%v, outside plot [%v,%v]",
					orient, g.Content, g.Y, in.Layout.Y, in.Layout.Bottom())
			}
		}
	}
}

// node_shape: "none" (the dendrogram default) emits no glyph but must
// still emit every label — that is the whole point of the variant.
func TestPrismDendrogramLabelsWithoutGlyphs(t *testing.T) {
	in := orgInputs(t, &spec.TextChannel{Field: "name", Type: "nominal"}, nil)
	marks, _, err := Encode("dendrogram", in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if got := len(nodeCenters(marks)); got != 0 {
		t.Errorf("glyph marks = %d, want 0 for node_shape none", got)
	}
	if got := len(textMarks(marks)); got != 4 {
		t.Errorf("label marks = %d, want 4", got)
	}
}

func networkInputs(t *testing.T, text *spec.TextChannel) Inputs {
	t.Helper()
	return Inputs{
		Table: buildTable(t, map[string]any{
			"from":  []string{"app", "app", "ui"},
			"to":    []string{"ui", "data", "icons"},
			"label": []string{"UI kit", "Data layer", "Icons"},
		}),
		Source: Channel{Field: "from"},
		Target: Channel{Field: "to"},
		Layout: plotRect(),
		Text:   text,
	}
}

// A network with no text channel keeps its historical geometry: no
// label marks, and the force layout still fills the whole plot rect.
func TestPrismNetworkWithoutTextChannelEmitsNoLabels(t *testing.T) {
	plain, _, err := Encode("network", networkInputs(t, nil))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if got := len(textMarks(plain)); got != 0 {
		t.Fatalf("label marks = %d, want 0 with no text channel", got)
	}
	labelled, _, err := Encode("network", networkInputs(t, &spec.TextChannel{Field: "label", Type: "nominal"}))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if nodeCenters(plain) == nil || nodeCenters(labelled) == nil {
		t.Fatal("expected node marks in both runs")
	}
	// Labelling insets the force rect, so the node positions must
	// differ — proving the un-labelled run was left untouched.
	if nodeCenters(plain)[0] == nodeCenters(labelled)[0] {
		t.Error("labelled run reused the un-inset layout rect")
	}
}

// Network rows are edges, so a row's label binds to that row's target
// node; a node that only ever appears as a source falls back to its id.
func TestPrismNetworkNodeLabels(t *testing.T) {
	marks, _, err := Encode("network", networkInputs(t, &spec.TextChannel{Field: "label", Type: "nominal"}))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	labels := labelByNode(marks, "network-label-")
	want := map[string]string{
		"ui":    "UI kit",
		"data":  "Data layer",
		"icons": "Icons",
		"app":   "app", // pure source: no row of its own, falls back to id
	}
	if len(labels) != len(want) {
		t.Fatalf("label marks = %d, want %d", len(labels), len(want))
	}
	for id, content := range want {
		g, ok := labels[id]
		if !ok {
			t.Fatalf("no label mark for node %q", id)
		}
		if g.Content != content {
			t.Errorf("label[%s] = %q, want %q", id, g.Content, content)
		}
		if g.Anchor != scene.AnchorMiddle {
			t.Errorf("label[%s].Anchor = %q, want middle", id, g.Anchor)
		}
	}
}

func TestPrismInsetForLabelsClamps(t *testing.T) {
	r := scene.Rect{X: 10, Y: 20, W: 100, H: 50}
	got := insetForLabels(r, 5, 7, 2, 3)
	want := scene.Rect{X: 15, Y: 22, W: 88, H: 45}
	if got != want {
		t.Errorf("insetForLabels(small) = %+v, want %+v", got, want)
	}
	// An inset larger than 40% of the dimension is clamped per side so
	// the rect never inverts.
	got = insetForLabels(r, 400, 400, 400, 400)
	want = scene.Rect{X: 50, Y: 40, W: 20, H: 10}
	if got != want {
		t.Errorf("insetForLabels(huge) = %+v, want %+v", got, want)
	}
	if got := insetForLabels(r, -3, -3, -3, -3); got != r {
		t.Errorf("insetForLabels(negative) = %+v, want %+v", got, r)
	}
}
