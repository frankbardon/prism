package marks

import (
	"testing"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
)

// Local linear scale for tests so we do not depend on the encode
// package (which would create an import cycle). Identical math to
// encode.LinearScale.
type linScale struct {
	dmin, dmax, rmin, rmax float64
}

func (s *linScale) Apply(v any) (float64, error) {
	var f float64
	switch x := v.(type) {
	case int64:
		f = float64(x)
	case float64:
		f = x
	default:
		f = 0
	}
	if s.dmax == s.dmin {
		return (s.rmin + s.rmax) / 2, nil
	}
	t := (f - s.dmin) / (s.dmax - s.dmin)
	return s.rmin + t*(s.rmax-s.rmin), nil
}
func (s *linScale) Domain() []any { return []any{s.dmin, s.dmax} }

// Local band scale for tests.
type bandScaleT struct {
	cats     []string
	rmin     float64
	rmax     float64
	padding  float64
}

func (s *bandScaleT) Apply(v any) (float64, error) {
	cat := v.(string)
	step := (s.rmax - s.rmin) / float64(len(s.cats))
	pad := step * s.padding / 2
	for i, c := range s.cats {
		if c == cat {
			return s.rmin + float64(i)*step + pad, nil
		}
	}
	return 0, nil
}
func (s *bandScaleT) Domain() []any {
	out := make([]any, len(s.cats))
	for i, c := range s.cats {
		out[i] = c
	}
	return out
}
func (s *bandScaleT) BandWidth() float64 {
	step := (s.rmax - s.rmin) / float64(len(s.cats))
	return step * (1 - s.padding)
}

// buildTable constructs a minimal table with the supplied columns
// for tests. fields is name → (kind, values). Inferred schema; no
// hashing; rowCount = len of first column.
func buildTable(t *testing.T, fields map[string]any) *table.Table {
	t.Helper()
	var schema encoding.Schema
	cols := map[string]table.Column{}
	rowCount := -1
	order := []string{}
	for name := range fields {
		order = append(order, name)
	}
	// Stable order: insertion via sort.
	// Tests pass small handful of fields; alphabetical order is fine.
	sortStrings(order)
	for _, name := range order {
		switch v := fields[name].(type) {
		case []string:
			schema.Fields = append(schema.Fields, encoding.Field{
				Name:       name,
				Type:       encoding.FieldTypeCategoricalU8,
				Dictionary: encoding.NewDictionary(),
			})
			cols[name] = table.StringColumn(v)
			if rowCount < 0 {
				rowCount = len(v)
			}
		case []float64:
			schema.Fields = append(schema.Fields, encoding.Field{Name: name, Type: encoding.FieldTypeF64})
			cols[name] = table.FloatColumn(v)
			if rowCount < 0 {
				rowCount = len(v)
			}
		case []int64:
			schema.Fields = append(schema.Fields, encoding.Field{Name: name, Type: encoding.FieldTypeU64})
			cols[name] = table.IntColumn(v)
			if rowCount < 0 {
				rowCount = len(v)
			}
		default:
			t.Fatalf("buildTable: unsupported value type %T for %s", v, name)
		}
	}
	tbl, err := table.NewTable(&schema, cols, rowCount, "xxh64:test")
	if err != nil {
		t.Fatalf("table.NewTable: %v", err)
	}
	return tbl
}

func sortStrings(xs []string) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

// plotRect is the canonical 740×540 plot region every test uses.
func plotRect() scene.Rect {
	return scene.Rect{X: 40, Y: 20, W: 740, H: 540}
}
