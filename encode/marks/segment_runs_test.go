package marks

import (
	"reflect"
	"testing"
)

// segmentRuns is what turns mark.invalid:"break" into visible gaps, so
// its edge cases are the feature's edge cases.
func TestPrismSegmentRuns(t *testing.T) {
	cases := []struct {
		name string
		skip []bool
		idxs []int
		want [][]int
	}{{
		// The whole of "filter" mode and every chart with clean data.
		// One run, returned as-is, so callers emit exactly one mark per
		// group and no existing output moves.
		name: "no mask is one run",
		skip: nil,
		idxs: []int{0, 1, 2, 3},
		want: [][]int{{0, 1, 2, 3}},
	}, {
		name: "hole in the middle splits",
		skip: []bool{false, false, true, false},
		idxs: []int{0, 1, 2, 3},
		want: [][]int{{0, 1}, {3}},
	}, {
		// Leading and trailing gaps contribute no run rather than an
		// empty one, or the encoder would emit a mark with no points.
		name: "leading gap contributes no run",
		skip: []bool{true, false, false},
		idxs: []int{0, 1, 2},
		want: [][]int{{1, 2}},
	}, {
		name: "trailing gap contributes no run",
		skip: []bool{false, false, true},
		idxs: []int{0, 1, 2},
		want: [][]int{{0, 1}},
	}, {
		name: "consecutive holes make one gap, not two",
		skip: []bool{false, true, true, false},
		idxs: []int{0, 1, 2, 3},
		want: [][]int{{0}, {3}},
	}, {
		// Every row null is an error upstream (PRISM_ENCODE_NULL_ALL_ROWS),
		// but the helper must not invent a run if it ever gets here.
		name: "all skipped yields nothing",
		skip: []bool{true, true},
		idxs: []int{0, 1},
		want: nil,
	}, {
		// Runs follow the ORDER GIVEN, not table order: line and area
		// sort a group by x before splitting, so a gap is between
		// neighbours in the drawn path, not in the source rows.
		name: "respects the caller's ordering",
		skip: []bool{false, true, false, false},
		idxs: []int{3, 2, 1, 0},
		want: [][]int{{3, 2}, {0}},
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := segmentRuns(Inputs{Skip: c.skip}, c.idxs)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("segmentRuns(%v, %v) = %v, want %v", c.skip, c.idxs, got, c.want)
			}
		})
	}
}

// TestPrismSkipRowToleratesShortMask guards the helper against an
// index outside the mask rather than panicking mid-encode.
func TestPrismSkipRowToleratesShortMask(t *testing.T) {
	in := Inputs{Skip: []bool{true, false}}
	for _, i := range []int{-1, 2, 99} {
		if skipRow(in, i) {
			t.Errorf("skipRow(out-of-range %d) = true, want false", i)
		}
	}
	if !skipRow(in, 0) {
		t.Error("skipRow(0) = false, want true")
	}
}
