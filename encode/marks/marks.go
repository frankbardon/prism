// Package marks holds the per-mark encoders that turn rows of a
// materialised table into scene.Mark entries with pixel-resolved
// geometry. P05 supports five marks: bar (Rect), line, area, point,
// rule. Other types (arc/text/path/image, plus composite/specialty)
// emit a PRISM_WARN_MARK_NOT_IMPLEMENTED warning.
package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// Channel binds an encoding channel to a resolved Scale and the
// field name on the upstream table. Created by encode.Encode and
// handed to each mark encoder.
type Channel struct {
	Field string
	Scale Scale
}

// Scale is the minimal scale surface mark encoders need. Matches
// encode.Scale by structural duck-typing without an import cycle.
type Scale interface {
	Apply(value any) (float64, error)
	Domain() []any
}

// BandScaler is the optional capability bar marks ask for to size
// rect widths. Implemented by encode.BandScale.
type BandScaler interface {
	BandWidth() float64
}

// ColorChannel binds a color encoding (categorical field + palette).
type ColorChannel struct {
	Field      string
	Categories []string
	Palette    []*scene.Color
}

// Inputs carries the per-Encode-call context: table, encoded
// channels, layout, mark style.
type Inputs struct {
	Table  *table.Table
	X      Channel
	Y      Channel
	Color  *ColorChannel
	Layout scene.Rect // the Plot region
	Style  scene.Style
	Mark   *spec.MarkDef // mark-level overrides; nil ok
}

// Encode dispatches markType to its per-mark helper. Returns the
// generated marks + an optional warning (for unsupported types).
// Errors bubble PRISM_ENCODE_001 or PRISM_RENDER_001 from the
// helpers.
func Encode(markType string, in Inputs) ([]scene.Mark, *scene.Warning, error) {
	switch markType {
	case "bar":
		marks, err := encodeBar(in)
		return marks, nil, err
	case "line":
		marks, err := encodeLine(in)
		return marks, nil, err
	case "area":
		marks, err := encodeArea(in)
		return marks, nil, err
	case "point":
		marks, err := encodePoint(in)
		return marks, nil, err
	case "rule":
		marks, err := encodeRule(in)
		return marks, nil, err
	case "text":
		marks, err := encodeText(in)
		return marks, nil, err
	case "tick":
		marks, err := encodeTick(in)
		return marks, nil, err
	case "rect":
		marks, err := encodeRect(in)
		return marks, nil, err
	case "arc", "path", "image",
		"pie", "donut", "histogram", "heatmap", "boxplot", "violin",
		"sankey", "funnel", "sparkline":
		return nil, &scene.Warning{
			Code:    scene.WarnMarkNotImplemented,
			Message: fmt.Sprintf("mark type %q lands in a later phase; layer rendered without marks.", markType),
			Details: map[string]any{"mark": markType},
		}, nil
	}
	return nil, nil, prismerrors.New(
		"PRISM_ENCODE_001",
		fmt.Sprintf("Unknown mark type %q.", markType),
		map[string]any{"Field": "<mark>", "Source": "<spec>", "Available": "bar|line|area|point|rule"},
	)
}

// readField returns a slice of values from the named column on tbl.
// Returns PRISM_ENCODE_001 when the field is missing.
func readField(tbl *table.Table, name string) ([]any, error) {
	col, ok := tbl.Column(name)
	if !ok {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("Field %q not present in upstream table.", name),
			map[string]any{"Field": name, "Source": "<table>", "Available": joinFieldNames(tbl)},
		)
	}
	out := make([]any, col.Len())
	for i := 0; i < col.Len(); i++ {
		out[i] = col.ValueAt(i)
	}
	return out, nil
}

// joinFieldNames returns the table's columns as a comma-separated
// string for error context.
func joinFieldNames(tbl *table.Table) string {
	names := tbl.FieldNames()
	if len(names) == 0 {
		return ""
	}
	out := names[0]
	for _, n := range names[1:] {
		out += ", " + n
	}
	return out
}
