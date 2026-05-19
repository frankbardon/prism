package encode

import (
	"fmt"

	encresolve "github.com/frankbardon/prism/encode/resolve"
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// encodeRepeatComposite turns a repeat CompositeDAG into a SceneDoc.
// Per D056 the builder returned one ChildDAG per cell (row-major)
// with the field-name substitution already applied; this function
// runs the flat Encode per cell and stitches the resulting Scenes
// into a grid in the same pattern concat / hconcat use (P08).
//
// Per D057 repeat defaults `resolve.scale.{x,y}` to "independent"
// (every cell plots a different field, so a shared y-domain rarely
// makes sense). The defaults override is applied here on top of
// encode/resolve.FromSpec's standard Vega-Lite defaults; the spec's
// `resolve` block still wins when set.
func encodeRepeatComposite(s *spec.Spec, composite *plan.CompositeDAG, childTables []map[plan.NodeID]*table.Table, opts EncodeOpts) (*scene.SceneDoc, error) {
	if s.Repeat == nil {
		return nil, fmt.Errorf("encode: repeat composite missing Repeat block on parent spec")
	}
	if len(composite.Children) == 0 {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"Repeat composite has no children.",
			map[string]any{"Field": "<repeat>", "Source": "<builder>", "Available": ""},
		)
	}

	outerW := opts.Width
	if outerW == 0 {
		outerW = 800
	}
	outerH := opts.Height
	if outerH == 0 {
		outerH = 600
	}
	sceneTheme, err := resolveTheme(opts, s.Theme)
	if err != nil {
		return nil, err
	}

	rows := composite.Rows
	cols := composite.Cols
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	gap := 20.0
	cellW := (outerW - gap*float64(cols-1)) / float64(cols)
	cellH := (outerH - gap*float64(rows-1)) / float64(rows)
	if cellW < 1 {
		cellW = outerW / float64(cols)
	}
	if cellH < 1 {
		cellH = outerH / float64(rows)
	}

	// Resolve scale modes (D057: default to independent for x/y on
	// repeat unless the spec overrides).
	resolution := encresolve.FromSpec(composite.Resolve)
	if composite.Resolve == nil || composite.Resolve.Scale == nil || composite.Resolve.Scale.X == "" {
		r := resolution[scene.ChannelX]
		r.Scale = encresolve.ModeIndependent
		r.Axis = encresolve.ModeIndependent
		resolution[scene.ChannelX] = r
	}
	if composite.Resolve == nil || composite.Resolve.Scale == nil || composite.Resolve.Scale.Y == "" {
		r := resolution[scene.ChannelY]
		r.Scale = encresolve.ModeIndependent
		r.Axis = encresolve.ModeIndependent
		resolution[scene.ChannelY] = r
	}
	// Independent defaults mean we do NOT compute shared scales for
	// repeat in the common case; the per-cell Encode resolves each
	// axis from its own partition's values. If the spec explicitly
	// requests shared, fall back to the same union path facet uses.

	var cells []scene.SceneCell
	var warnings []scene.Warning
	for i, child := range composite.Children {
		row := i / cols
		col := i % cols
		offsetX := float64(col) * (cellW + gap)
		offsetY := float64(row) * (cellH + gap)

		childOpts := opts
		childOpts.Width = cellW
		childOpts.Height = cellH
		// No shared-scale override under independent defaults.
		childOpts.OverrideXScale = nil
		childOpts.OverrideYScale = nil

		childDoc, err := Encode(child.Spec, childTables[i], child.Tip, childOpts)
		if err != nil {
			return nil, fmt.Errorf("repeat child %d: %w", i, err)
		}
		if len(childDoc.Grid.Cells) == 0 {
			continue
		}
		childScene := childDoc.Grid.Cells[0].Scene
		offsetScene(&childScene, offsetX, offsetY)
		childScene.ID = fmt.Sprintf("scene-%d", i)
		cells = append(cells, scene.SceneCell{
			Row:   row,
			Col:   col,
			Scene: childScene,
		})
		warnings = append(warnings, childDoc.Warnings...)
	}

	doc := scene.NewDoc()
	doc.Theme = sceneTheme
	doc.Grid = scene.SceneGrid{
		Layout: scene.GridLayout{Rows: rows, Cols: cols, GapPx: int(gap)},
		Cells:  cells,
	}
	doc.Warnings = warnings
	return doc, nil
}
