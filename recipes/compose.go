package recipes

import (
	"errors"
	"fmt"

	"github.com/frankbardon/prism/spec"
)

// TilesOptions configures Tiles.
type TilesOptions struct {
	// Label names each tile (drawn above it); Value is the big number,
	// printed through Value.Format.
	Label Field
	Value Field
	// FontSize of the numbers (28 when zero).
	FontSize float64
	Tooltip  []Field
	// Frame; a zero Height is 160.
	Frame Frame
}

// Tiles draws one big number per row, side by side, each named by its
// label above it — a stat-tile row for a handful of headline values.
func Tiles(rows []Row, o TilesOptions) (*spec.Spec, error) {
	if err := require("Tiles", "Label", o.Label.Name, "Value", o.Value.Name); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoRows
	}
	rows = tidy(rows, o.Label.Name)
	size := o.FontSize
	if size <= 0 {
		size = 28
	}
	x := categoryAxis(o.Label, distinct(rows, o.Label.Name), false)
	x.Axis = &spec.Axis{Orient: "top", Title: "", Domain: flag(false), Ticks: flag(false), Grid: flag(false)}
	s := newSpec()
	s.Data = data(rows)
	s.Mark = markDef(spec.MarkDef{Type: "text", FontSize: num(size)})
	s.Encoding = &spec.Encoding{
		X:       x,
		Text:    &spec.TextChannel{Field: o.Value.Name, Format: o.Value.Format},
		Tooltip: tooltip(o.Tooltip),
	}
	applyFrame(s, o.Frame, 160)
	return s, nil
}

// Stack stacks panels top to bottom (a vconcat). Each panel keeps its
// own scales, title and height (a recipe's BandHeight, say); the stack
// is their sum plus a 20 px gap. A single panel is returned as is.
func Stack(panels ...*spec.Spec) *spec.Spec {
	panels = nonNil(panels)
	if len(panels) == 1 {
		return panels[0]
	}
	s := newSpec()
	s.VConcat = panels
	return s
}

// Grid lays cells out left to right in rows of at most columns (a
// wrapped concat; 0 keeps one row). Cells keep their own scales —
// pin a shared domain on each (see Extent) for small multiples that
// compare at a glance. A single cell is returned as is.
func Grid(columns int, cells ...*spec.Spec) *spec.Spec {
	cells = nonNil(cells)
	if len(cells) == 1 {
		return cells[0]
	}
	s := newSpec()
	s.Concat = cells
	if columns > 0 && columns < len(cells) {
		c := columns
		s.Columns = &c
	}
	return s
}

// SmallMultiples splits rows by a category column (in order, else
// first appearance) and lays one cell per category out in a Grid of
// columns. cell builds each cell from its category's rows; the cell is
// titled with the category unless cell sets a title of its own.
func SmallMultiples(rows []Row, by string, order []string, columns int, cell func(key string, rows []Row, index int) (*spec.Spec, error)) (*spec.Spec, error) {
	if by == "" || cell == nil {
		return nil, errors.New("recipes: SmallMultiples needs a column and a cell builder")
	}
	keys, groups := Split(rows, by, order)
	if len(keys) == 0 {
		return nil, ErrNoRows
	}
	cells := make([]*spec.Spec, 0, len(keys))
	for i, k := range keys {
		c, err := cell(k, groups[k], i)
		if err != nil {
			return nil, fmt.Errorf("recipes: SmallMultiples cell %q: %w", k, err)
		}
		if c.Title == nil {
			t := k
			c.Title = &spec.TextOrTextObj{Text: &t}
		}
		cells = append(cells, c)
	}
	return Grid(columns, cells...), nil
}

// Split groups rows by a column's (stringified) value, returning the
// keys in order — the listed ones first, then any other in first-seen
// order — and each key's rows in row order. Rows with no value in the
// column are dropped.
func Split(rows []Row, col string, order []string) ([]string, map[string][]Row) {
	groups := map[string][]Row{}
	var seen []string
	for _, r := range rows {
		v, ok := r[col]
		if !ok || v == nil {
			continue
		}
		k, isStr := v.(string)
		if !isStr {
			k = fmt.Sprint(v)
		}
		if _, ok := groups[k]; !ok {
			seen = append(seen, k)
		}
		groups[k] = append(groups[k], r)
	}
	keys := make([]string, 0, len(seen))
	listed := map[string]bool{}
	for _, k := range order {
		if _, ok := groups[k]; ok && !listed[k] {
			keys = append(keys, k)
			listed[k] = true
		}
	}
	for _, k := range seen {
		if !listed[k] {
			keys = append(keys, k)
		}
	}
	return keys, groups
}

func nonNil(specs []*spec.Spec) []*spec.Spec {
	out := specs[:0:0]
	for _, s := range specs {
		if s != nil {
			out = append(out, s)
		}
	}
	return out
}

func errNeedOne(recipe, what string) error {
	return fmt.Errorf("recipes: %s needs %s", recipe, what)
}
