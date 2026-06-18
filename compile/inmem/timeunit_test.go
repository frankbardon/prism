package inmem

import (
	"context"
	"testing"
	"time"

	"github.com/frankbardon/pulse/encoding"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// dateTable builds a one-column date table from RFC-3339 day strings.
func dateTable(t *testing.T, days []string) *table.Table {
	t.Helper()
	col := make(table.DateColumn, len(days))
	for i, d := range days {
		ts, err := time.Parse("2006-01-02", d)
		if err != nil {
			t.Fatalf("parse %q: %v", d, err)
		}
		col[i] = ts.Unix() / secondsPerDay
	}
	schema := &encoding.Schema{Fields: []encoding.Field{{Name: "d", Type: encoding.FieldTypeDate}}}
	tbl, err := table.NewTable(schema, map[string]table.Column{"d": col}, len(days), "datesrc")
	if err != nil {
		t.Fatalf("NewTable: %v", err)
	}
	return tbl
}

// dayOf returns the days-since-epoch for an RFC-3339 day string.
func dayOf(t *testing.T, d string) int64 {
	t.Helper()
	ts, err := time.Parse("2006-01-02", d)
	if err != nil {
		t.Fatalf("parse %q: %v", d, err)
	}
	return ts.Unix() / secondsPerDay
}

func TestExecuteTimeUnitTruncation(t *testing.T) {
	cases := []struct {
		unit string
		in   string
		want string
	}{
		{"year", "2024-03-15", "2024-01-01"},
		{"quarter", "2024-05-20", "2024-04-01"},
		{"quarter", "2024-12-31", "2024-10-01"},
		{"month", "2024-03-15", "2024-03-01"},
		{"week", "2024-03-15", "2024-03-11"}, // 2024-03-15 is a Friday → Monday 2024-03-11
		{"day", "2024-03-15", "2024-03-15"},
	}
	for _, c := range cases {
		in := dateTable(t, []string{c.in})
		n := nodes.NewTimeUnit("tu:1", "src", "d", c.unit, "bucket")
		out, err := executeTimeUnit(context.Background(), n, []*table.Table{in})
		if err != nil {
			t.Fatalf("%s/%s: executeTimeUnit: %v", c.unit, c.in, err)
		}
		col, ok := out.Column("bucket")
		if !ok {
			t.Fatalf("%s: missing bucket column", c.unit)
		}
		got, _ := col.ValueAt(0).(int64)
		if want := dayOf(t, c.want); got != want {
			t.Errorf("%s(%s) = %d days, want %s (%d days)", c.unit, c.in, got, c.want, want)
		}
	}
}

func TestPrismInMemBackendDispatchTimeUnit(t *testing.T) {
	in := dateTable(t, []string{"2024-03-15", "2024-03-20"})
	b := New()
	n := nodes.NewTimeUnit("tu:1", "src", "d", "month", "bucket")
	out, err := b.Compile(context.Background(), n, []*table.Table{in})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	col, ok := out.Column("bucket")
	if !ok {
		t.Fatal("missing bucket column")
	}
	want := dayOf(t, "2024-03-01")
	for i := 0; i < out.NumRows(); i++ {
		if got, _ := col.ValueAt(i).(int64); got != want {
			t.Errorf("row %d bucket = %d, want %d", i, got, want)
		}
	}
}

func TestExecuteTimeUnitBadUnit(t *testing.T) {
	in := dateTable(t, []string{"2024-01-01"})
	n := nodes.NewTimeUnit("tu:1", "src", "d", "fortnight", "bucket")
	if _, err := executeTimeUnit(context.Background(), n, []*table.Table{in}); err == nil {
		t.Fatal("expected error for unsupported unit")
	}
}
