// Package recipes builds Prism specs for common analysis charts from
// tidy rows: a forest (coefficient / interval) plot, bars and grouped
// bars, lines, scatters with reference lines, heatmaps, summary boxes,
// big-number tiles, and the stacks and grids that compose them into
// small multiples.
//
// A recipe takes rows ([]Row, one map per observation, keyed by column
// name) plus an options struct naming the columns to draw and how they
// read, and returns a *spec.Spec with the rows inlined in data.values.
// The spec is plain Prism: validate, compile and render it like any
// other, patch it, or marshal it to JSON for a browser. Recipes hold
// no state and know nothing about where the rows came from; anything
// domain-specific (which columns are estimates, what counts as
// significant, how a value is labelled) is the caller's to decide and
// arrives as a column or an option.
//
// Every recipe leans on the same vocabulary:
//
//   - a Field names a column and how it reads: Title becomes the axis,
//     legend or tooltip label, Format the d3-format specifier for its
//     tooltip line (and its cell label on a heatmap or tile);
//   - category columns are drawn in Order when one is given (the first
//     at the left, or at the top of a vertical list), else in first-seen
//     row order, and are stringified so numeric keys still form bands;
//   - a Frame titles and sizes the chart; a zero height takes the
//     recipe's own (BandHeight for one band per category).
//
// Recipes return ErrNoRows when there is nothing to draw and an error
// naming the option when a required column is not set.
package recipes

import (
	"errors"
	"fmt"
	"math"

	"github.com/frankbardon/prism/spec"
)

// SchemaURN is the $schema every spec a recipe builds carries, on the
// root and on every composition child (the schema requires it there).
const SchemaURN = "urn:prism:schema:v1:spec"

// Row is one tidy observation: column name to value. Values are the
// JSON scalars (string, float64, int, bool) or nil.
type Row = map[string]any

// ErrNoRows reports that a recipe was handed no rows to draw.
var ErrNoRows = errors.New("recipes: no rows to draw")

// Field names a column and how it reads.
type Field struct {
	// Name is the column.
	Name string
	// Title labels the column on its axis, legend or tooltip line;
	// empty reads as Name.
	Title string
	// Format is a d3-format specifier (".3~g", ",.0f", ".1%") for the
	// column's tooltip line and, where a recipe prints the value, its
	// label. Empty prints the value as is.
	Format string
}

// F is shorthand for a Field with just a column name.
func F(name string) Field { return Field{Name: name} }

// label is what the field reads as.
func (f Field) label() string {
	if f.Title != "" {
		return f.Title
	}
	return f.Name
}

// Frame titles and sizes a chart. Zero values leave the choice to the
// recipe (no title; the renderer's width; the recipe's height).
type Frame struct {
	Title  string
	Width  float64
	Height float64
}

// Color colours marks by a categorical column.
type Color struct {
	Field Field
	// Domain lists the categories in palette order: the first takes
	// the first colour, whichever row it appears in first, and the
	// legend lists them in this order. Empty uses first appearance.
	Domain []string
	// Scheme names a palette (a built-in scheme or one the theme
	// defines); Range lists colours outright and wins over Scheme.
	Scheme string
	Range  []string
	// HideLegend drops the legend; otherwise it sits right of the
	// plot, titled with the field's label.
	HideLegend bool
}

// Default band geometry, matching what Studio's builders used: a band
// per category row plus room for the axis, title and legend.
const (
	RowHeight   = 36.0
	PanelChrome = 96.0
)

// BandHeight is a chart height that gives n category bands RowHeight
// each, plus PanelChrome for the title and axis.
func BandHeight(n int) float64 {
	if n < 1 {
		n = 1
	}
	return float64(n)*RowHeight + PanelChrome
}

// --- spec construction helpers ---------------------------------------

func newSpec() *spec.Spec { return &spec.Spec{Schema: SchemaURN} }

// applyFrame sets title / size on s; height falls back to def when the
// frame leaves it zero (and def is positive).
func applyFrame(s *spec.Spec, f Frame, def float64) {
	if f.Title != "" {
		t := f.Title
		s.Title = &spec.TextOrTextObj{Text: &t}
	}
	if f.Width > 0 {
		s.Width = dim(f.Width)
	}
	h := f.Height
	if h <= 0 {
		h = def
	}
	if h > 0 {
		s.Height = dim(h)
	}
}

func dim(v float64) *spec.Dimension { return &spec.Dimension{Number: &v} }

func num(v float64) *float64 { return &v }

func flag(v bool) *bool { return &v }

func data(rows []Row) *spec.Data { return &spec.Data{Values: rows} }

func mark(t string) *spec.Mark { return &spec.Mark{Shorthand: t} }

func markDef(d spec.MarkDef) *spec.Mark { return &spec.Mark{Def: &d} }

// position builds a position channel bound to f.
func position(f Field, typ string) *spec.PositionChannel {
	return &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: f.Name, Type: typ, Title: f.Title}}
}

// field binds a bare column (a span companion, or an internal
// position whose axis is titled elsewhere).
func bare(name, typ string) *spec.PositionChannel {
	return &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: name, Type: typ}}
}

// categoryAxis is a nominal position channel in order (first at the
// left; with top, first at the top of a vertical band list).
func categoryAxis(f Field, order []string, top bool) *spec.PositionChannel {
	ch := position(f, "nominal")
	if len(order) > 0 {
		ch.Sort = anyList(order)
	}
	if top {
		ch.Scale = &spec.Scale{Reverse: flag(true)}
	}
	return ch
}

// valueAxis is a quantitative position channel, pinned to domain when
// it has two bounds.
func valueAxis(f Field, domain []float64) *spec.PositionChannel {
	ch := position(f, "quantitative")
	if len(domain) == 2 {
		ch.Scale = &spec.Scale{Domain: []any{domain[0], domain[1]}}
	}
	return ch
}

// colorChannel binds c, or returns nil when c is nil.
func colorChannel(c *Color) *spec.MarkChannel {
	if c == nil || c.Field.Name == "" {
		return nil
	}
	ch := &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: c.Field.Name, Type: "nominal", Title: c.Field.Title}}
	if len(c.Domain) > 0 || c.Scheme != "" || len(c.Range) > 0 {
		ch.Scale = &spec.Scale{Scheme: c.Scheme}
		if len(c.Domain) > 0 {
			ch.Scale.Domain = anyList(c.Domain)
		}
		if len(c.Range) > 0 {
			ch.Scale.Range = anyList(c.Range)
		}
	}
	if c.HideLegend {
		ch.LegendHidden = true
	} else {
		ch.Legend = &spec.Legend{Orient: "right"}
	}
	return ch
}

// withoutLegend is ch with its legend hidden (a second layer coloured
// by the same column keys nothing new).
func withoutLegend(ch *spec.MarkChannel) *spec.MarkChannel {
	if ch == nil {
		return nil
	}
	c := *ch
	c.Legend = nil
	c.LegendHidden = true
	return &c
}

// tooltip binds the tooltip lines, nil when there are none.
func tooltip(fields []Field) *spec.TooltipChannel {
	if len(fields) == 0 {
		return nil
	}
	lines := make([]spec.TextChannel, len(fields))
	for i, f := range fields {
		lines[i] = spec.TextChannel{Field: f.Name, Title: f.Title, Format: f.Format}
	}
	return &spec.TooltipChannel{Multi: lines}
}

func anyList(xs []string) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

// layer wraps children as a layered spec over rows.
func layer(rows []Row, children ...*spec.Spec) *spec.Spec {
	s := newSpec()
	s.Data = data(rows)
	s.Layer = children
	return s
}

// --- row helpers ------------------------------------------------------

// require reports an error for every required option left empty.
func require(recipe string, named ...string) error {
	for i := 0; i+1 < len(named); i += 2 {
		if named[i+1] == "" {
			return fmt.Errorf("recipes: %s needs %s", recipe, named[i])
		}
	}
	return nil
}

// tidy copies rows with each category column stringified (a band scale
// keys on strings, so a numeric year or id must become one), leaving
// the caller's maps untouched.
func tidy(rows []Row, categories ...string) []Row {
	out := make([]Row, len(rows))
	for i, r := range rows {
		c := make(Row, len(r))
		for k, v := range r {
			c[k] = v
		}
		for _, k := range categories {
			if k == "" {
				continue
			}
			if v, ok := c[k]; ok && v != nil {
				if _, isStr := v.(string); !isStr {
					c[k] = fmt.Sprint(v)
				}
			}
		}
		out[i] = c
	}
	return out
}

// distinct lists a column's string values in first-seen order.
func distinct(rows []Row, col string) []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range rows {
		s, ok := r[col].(string)
		if !ok || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// categoryCount is how many bands a category column draws: its order
// plus any category the order leaves out.
func categoryCount(rows []Row, col string, order []string) int {
	n := len(order)
	listed := map[string]bool{}
	for _, o := range order {
		listed[o] = true
	}
	for _, c := range distinct(rows, col) {
		if !listed[c] {
			n++
		}
	}
	return n
}

// finite reads a numeric cell.
func finite(v any) (float64, bool) {
	var f float64
	switch x := v.(type) {
	case float64:
		f = x
	case float32:
		f = float64(x)
	case int:
		f = float64(x)
	case int64:
		f = float64(x)
	case int32:
		f = float64(x)
	default:
		return 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// withAll keeps the rows whose columns all hold finite numbers.
func withAll(rows []Row, cols ...string) []Row {
	var out []Row
	for _, r := range rows {
		ok := true
		for _, c := range cols {
			if _, isNum := finite(r[c]); !isNum {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, r)
		}
	}
	return out
}

// Extent is the [lo, hi] of the finite values in the given columns
// across rows; ok is false when there are none. Use it to share one
// value domain across the cells of a grid.
func Extent(rows []Row, cols ...string) (lo, hi float64, ok bool) {
	for _, r := range rows {
		for _, c := range cols {
			v, isNum := finite(r[c])
			if !isNum {
				continue
			}
			if !ok {
				lo, hi, ok = v, v, true
				continue
			}
			lo = math.Min(lo, v)
			hi = math.Max(hi, v)
		}
	}
	return lo, hi, ok
}
