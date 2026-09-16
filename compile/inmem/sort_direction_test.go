package inmem

import (
	"context"
	"testing"

	"github.com/frankbardon/prism/plan/nodes"
	"github.com/frankbardon/prism/table"
)

// TestPrismSortDirectionSpellings pins the E5-S4 fix: the executors
// used to compare the wire value against the literal "desc" while the
// JSON Schema advertised only "ascending" / "descending", so the
// documented spelling silently sorted ascending.
func TestPrismSortDirectionSpellings(t *testing.T) {
	cases := []struct {
		order    string
		wantDesc bool
	}{
		{"", false},
		{"ascending", false},
		{"asc", false},
		{"descending", true},
		{"desc", true},
	}
	for _, tc := range cases {
		t.Run("sort/"+tc.order, func(t *testing.T) {
			in := helperInlineTable(t)
			n := nodes.NewSort("s:1", "src", []nodes.SortKey{{Field: "score", Order: tc.order}})
			out, err := New().Compile(context.Background(), n, []*table.Table{in})
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			col, _ := out.Column("score")
			head := col.ValueAt(0).(float64)
			tail := col.ValueAt(out.NumRows() - 1).(float64)
			if gotDesc := head > tail; gotDesc != tc.wantDesc {
				t.Fatalf("order %q: head=%v tail=%v, descending=%v want %v",
					tc.order, head, tail, gotDesc, tc.wantDesc)
			}
		})
	}
}

// TestPrismWindowSortDirectionSpellings covers the same fix on the
// window node's per-partition ordering.
func TestPrismWindowSortDirectionSpellings(t *testing.T) {
	for _, order := range []string{"descending", "desc"} {
		t.Run(order, func(t *testing.T) {
			in := helperInlineTable(t)
			n := nodes.NewWindow("w:1", "src",
				[]nodes.WindowOp{{Op: "row_number", As: "rn"}},
				nil,
				[]nodes.SortKey{{Field: "score", Order: order}},
				nil,
			)
			out, err := New().Compile(context.Background(), n, []*table.Table{in})
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			scores, _ := out.Column("score")
			rns, _ := out.Column("rn")
			// Row number 1 must land on the highest score.
			best, bestScore := -1, 0.0
			for i := 0; i < out.NumRows(); i++ {
				v := scores.ValueAt(i).(float64)
				if best == -1 || v > bestScore {
					best, bestScore = i, v
				}
			}
			if got := rns.ValueAt(best).(float64); got != 1 {
				t.Fatalf("order %q: highest score got rn=%v, want 1", order, got)
			}
		})
	}
}
