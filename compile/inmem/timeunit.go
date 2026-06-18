package inmem

import (
	"context"
	"fmt"
	"time"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

const secondsPerDay = 86400

// executeTimeUnit appends a date column (n.As()) carrying each row's
// source date truncated to the calendar period start (n.Unit()). Dates
// are days-since-epoch; truncation runs in UTC. Week starts Monday
// (ISO). Null source dates pass through as null (zero) in the output.
func executeTimeUnit(_ context.Context, n *nodes.TimeUnitNode, ins []*table.Table) (*table.Table, error) {
	in, err := requireOneInput(n, ins)
	if err != nil {
		return nil, err
	}
	if n.As() == "" {
		return nil, fmt.Errorf("TimeUnitNode: missing 'as' name")
	}
	src, ok := in.Column(n.Field())
	if !ok {
		return nil, fmt.Errorf("TimeUnitNode: source field %q not in input table", n.Field())
	}
	trunc, err := truncatorFor(n.Unit())
	if err != nil {
		return nil, err
	}

	rows := in.NumRows()
	out := make(table.DateColumn, rows)
	for i := 0; i < rows; i++ {
		if src.IsNull(i) {
			continue
		}
		days, ok := dayValue(src.ValueAt(i))
		if !ok {
			return nil, fmt.Errorf("TimeUnitNode: field %q row %d is not a date (got %T)", n.Field(), i, src.ValueAt(i))
		}
		t := time.Unix(days*secondsPerDay, 0).UTC()
		out[i] = trunc(t).Unix() / secondsPerDay
	}

	schema := cloneSchemaShallow(in.Schema())
	schema.Fields = append(schema.Fields, encoding.Field{Name: n.As(), Type: encoding.FieldTypeDate})
	cols := make(map[string]table.Column, len(in.FieldNames())+1)
	for _, name := range in.FieldNames() {
		c, _ := in.Column(name)
		cols[name] = c
	}
	cols[n.As()] = out
	return table.NewTable(schema, cols, rows, hashChain(in.Hash(), n.Fingerprint()))
}

// dayValue coerces a date cell (stored as int64 days-since-epoch) to
// its day count. Float values (defensive) are floored.
func dayValue(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	}
	return 0, false
}

// truncatorFor returns the period-start truncation for a unit, in UTC.
func truncatorFor(unit string) (func(time.Time) time.Time, error) {
	switch unit {
	case "year":
		return func(t time.Time) time.Time {
			return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		}, nil
	case "quarter":
		return func(t time.Time) time.Time {
			q := (int(t.Month()) - 1) / 3
			return time.Date(t.Year(), time.Month(q*3+1), 1, 0, 0, 0, 0, time.UTC)
		}, nil
	case "month":
		return func(t time.Time) time.Time {
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		}, nil
	case "week":
		return func(t time.Time) time.Time {
			// ISO week: Monday start. weekday Mon=1..Sun=7.
			wd := (int(t.Weekday()) + 6) % 7 // Mon=0..Sun=6
			d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			return d.AddDate(0, 0, -wd)
		}, nil
	case "day":
		return func(t time.Time) time.Time {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		}, nil
	}
	return nil, fmt.Errorf("TimeUnitNode: unsupported unit %q (use year/quarter/month/week/day)", unit)
}
