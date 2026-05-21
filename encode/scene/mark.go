package scene

import "fmt"

// MarkType is the canonical mark-type discriminator.
type MarkType string

// Mark types. The first five are the P05 core set; the remaining
// four are declared for JSON stability and round-trip parity but
// raise PRISM_WARN_MARK_NOT_IMPLEMENTED at encode time today.
const (
	MarkRect     MarkType = "rect"
	MarkLine     MarkType = "line"
	MarkArea     MarkType = "area"
	MarkPoint    MarkType = "point"
	MarkRule     MarkType = "rule"
	MarkArc      MarkType = "arc"
	MarkText     MarkType = "text"
	MarkPath     MarkType = "path"
	MarkImage    MarkType = "image"
	MarkGeoshape MarkType = "geoshape"
)

// Mark is the atomic visual primitive. Discriminated union: exactly
// one of the nine *Geom pointers is non-nil. JSON shape uses
// omitempty so unused slots disappear from the wire form even though
// they are nullable.
type Mark struct {
	Type    MarkType `json:"type"`
	ID      string   `json:"id,omitempty"`
	Style   Style    `json:"style,omitempty"`
	Tooltip *Tooltip `json:"tooltip,omitempty"`
	Datum   *Datum   `json:"datum,omitempty"`

	Rect     *RectGeom    `json:"rect,omitempty"`
	Line     *LineGeom    `json:"line,omitempty"`
	Area     *AreaGeom    `json:"area,omitempty"`
	Point    *PointGeom   `json:"point,omitempty"`
	Rule     *RuleGeom    `json:"rule,omitempty"`
	Arc      *ArcGeom     `json:"arc,omitempty"`
	Text     *TextGeom    `json:"text,omitempty"`
	Path     *PathGeom    `json:"path,omitempty"`
	Image    *ImageGeom   `json:"image,omitempty"`
	Geoshape *PolygonGeom `json:"geoshape,omitempty"`
}

// Validate confirms that exactly one geometry pointer is non-nil and
// that its Type field matches the populated geom. Defensive helper
// used by the encoder before passing marks to the renderer.
func (m *Mark) Validate() error {
	count := 0
	var have MarkType
	if m.Rect != nil {
		count++
		have = MarkRect
	}
	if m.Line != nil {
		count++
		have = MarkLine
	}
	if m.Area != nil {
		count++
		have = MarkArea
	}
	if m.Point != nil {
		count++
		have = MarkPoint
	}
	if m.Rule != nil {
		count++
		have = MarkRule
	}
	if m.Arc != nil {
		count++
		have = MarkArc
	}
	if m.Text != nil {
		count++
		have = MarkText
	}
	if m.Path != nil {
		count++
		have = MarkPath
	}
	if m.Image != nil {
		count++
		have = MarkImage
	}
	if m.Geoshape != nil {
		count++
		have = MarkGeoshape
	}
	if count == 0 {
		return fmt.Errorf("Mark: no geometry populated")
	}
	if count > 1 {
		return fmt.Errorf("Mark: %d geometries populated (expected 1)", count)
	}
	if m.Type != "" && m.Type != have {
		return fmt.Errorf("Mark: Type=%q but %q geometry populated", m.Type, have)
	}
	return nil
}

// Datum is the back-reference from a mark to its source row. Used
// for selection wiring and tooltip rendering.
type Datum struct {
	LayerID string         `json:"layer_id"`
	RowID   int64          `json:"row_id"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// DatumRef is a thin reference to a row in a layer (used by
// SelectionState.Points). Carries no field bag.
type DatumRef struct {
	LayerID string `json:"layer_id"`
	RowID   int64  `json:"row_id"`
}
