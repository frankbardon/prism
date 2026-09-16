package encode

import (
	"testing"

	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

func clipTable(t *testing.T) (map[plan.NodeID]*table.Table, plan.NodeID) {
	t.Helper()
	tbl, _, err := table.FromInline("readings", []map[string]any{
		{"day": "mon", "bpm": 95.0},
		{"day": "tue", "bpm": 118.0},
		{"day": "wed", "bpm": 150.0},
	}, nil)
	if err != nil {
		t.Fatalf("FromInline: %v", err)
	}
	const tip = plan.NodeID("tip")
	return map[plan.NodeID]*table.Table{tip: tbl}, tip
}

func clipSpec(domain []any, clip *bool) *spec.Spec {
	y := posChannel("bpm", "quantitative")
	if domain != nil {
		y.Scale = &spec.Scale{Domain: domain}
	}
	mark := &spec.Mark{Shorthand: "line"}
	if clip != nil {
		mark = &spec.Mark{Def: &spec.MarkDef{Type: "line", Clip: clip}}
	}
	return &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Mark:   mark,
		Encoding: &spec.Encoding{
			X: posChannel("day", "nominal"),
			Y: y,
		},
	}
}

func boolPtr(b bool) *bool { return &b }

// A data-derived domain puts every mark inside the plot rect, so no
// clip is armed and the Scene IR stays exactly as it was before E2-S2.
func TestPrismPlotClipAbsentWithoutExplicitDomain(t *testing.T) {
	tables, tip := clipTable(t)
	doc, err := Encode(clipSpec(nil, nil), tables, tip, EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	sc := doc.Grid.Cells[0].Scene
	if sc.ClipRef != "" {
		t.Errorf("ClipRef = %q, want empty", sc.ClipRef)
	}
	if sc.Defs != nil {
		t.Errorf("Defs = %+v, want nil", sc.Defs)
	}
}

// An explicit scale.domain is the one way a row can map outside the
// plot rect, so it arms the clip and registers the plot rect in Defs.
func TestPrismPlotClipArmedByExplicitDomain(t *testing.T) {
	tables, tip := clipTable(t)
	doc, err := Encode(clipSpec([]any{90.0, 130.0}, nil), tables, tip, EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	sc := doc.Grid.Cells[0].Scene
	want := plotClipID(sc.ID)
	if sc.ClipRef != want {
		t.Fatalf("ClipRef = %q, want %q", sc.ClipRef, want)
	}
	if sc.Defs == nil {
		t.Fatal("Defs is nil, want a clips entry")
	}
	got, ok := sc.Defs.Clips[want]
	if !ok {
		t.Fatalf("Defs.Clips has no %q entry (keys %v)", want, sc.Defs.Clips)
	}
	if got != sc.Plot {
		t.Errorf("clip rect = %+v, want the plot rect %+v", got, sc.Plot)
	}
	// The out-of-domain row must still be encoded — clipping is not
	// dropping and not clamping.
	if n := len(sc.Layers[0].Marks); n == 0 {
		t.Fatal("no marks encoded")
	}
	var overflowed bool
	for _, m := range sc.Layers[0].Marks {
		if m.Line == nil {
			continue
		}
		for _, p := range m.Line.Points {
			if p[1] < sc.Plot.Y-1e-6 {
				overflowed = true
			}
		}
	}
	if !overflowed {
		t.Error("no point above the plot top edge; the 150 row was dropped or clamped")
	}
}

// mark_def.clip overrides the default in both directions.
func TestPrismPlotClipMarkDefOverride(t *testing.T) {
	tables, tip := clipTable(t)

	doc, err := Encode(clipSpec(nil, boolPtr(true)), tables, tip, EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode(clip:true): %v", err)
	}
	if sc := doc.Grid.Cells[0].Scene; sc.ClipRef == "" {
		t.Error(`clip: true did not arm the clip`)
	}

	doc, err = Encode(clipSpec([]any{90.0, 130.0}, boolPtr(false)), tables, tip, EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode(clip:false): %v", err)
	}
	if sc := doc.Grid.Cells[0].Scene; sc.ClipRef != "" {
		t.Errorf(`clip: false left ClipRef = %q, want empty`, sc.ClipRef)
	}
}

// The clip is resolved per plot rect, so a layer composite gets one
// entry covering every layer, and a single layer asking for it is
// enough to arm it.
func TestPrismPlotClipScopeAcrossLayers(t *testing.T) {
	base := clipSpec(nil, nil)
	pinned := clipSpec([]any{90.0, 130.0}, nil)
	parent := &spec.Spec{Layer: []*spec.Spec{base, pinned}}
	if !wantsPlotClip(parent) {
		t.Error("a layer pinning a domain should arm the shared plot clip")
	}
	off := clipSpec(nil, boolPtr(false))
	if wantsPlotClip(&spec.Spec{Layer: []*spec.Spec{off, pinned}}) {
		t.Error("clip: false on a layer should disarm the shared plot clip")
	}
	on := clipSpec(nil, boolPtr(true))
	if !wantsPlotClip(&spec.Spec{Layer: []*spec.Spec{off, on}}) {
		t.Error("clip: true should outrank clip: false across layers")
	}
	// Concat / facet / repeat children each own a plot rect, so a
	// pinned child does not arm the parent's clip.
	if wantsPlotClip(&spec.Spec{Concat: []*spec.Spec{pinned}}) {
		t.Error("a concat child must not arm the parent scene's clip")
	}
}

// Renaming a cell scene re-keys its clip so a multi-cell grid never
// emits two clipPath elements sharing an id.
func TestPrismPlotClipRenameAndOffset(t *testing.T) {
	tables, tip := clipTable(t)
	doc, err := Encode(clipSpec([]any{90.0, 130.0}, nil), tables, tip, EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	sc := doc.Grid.Cells[0].Scene
	before := sc.Defs.Clips[sc.ClipRef]

	offsetScene(&sc, 40, 25)
	renameScene(&sc, "scene-r1-c2-0")

	if want := plotClipID("scene-r1-c2-0"); sc.ClipRef != want {
		t.Fatalf("ClipRef = %q, want %q", sc.ClipRef, want)
	}
	if len(sc.Defs.Clips) != 1 {
		t.Fatalf("Clips = %v, want exactly one re-keyed entry", sc.Defs.Clips)
	}
	got := sc.Defs.Clips[sc.ClipRef]
	if got.X != before.X+40 || got.Y != before.Y+25 {
		t.Errorf("clip rect = %+v, want %+v shifted by (40, 25)", got, before)
	}
	if got != sc.Plot {
		t.Errorf("clip rect %+v drifted from the offset plot rect %+v", got, sc.Plot)
	}
}
