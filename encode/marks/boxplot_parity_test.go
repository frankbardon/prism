package marks_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode/marks"
	"github.com/frankbardon/prism/encode/scale"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// TestPrismBoxplotQuantileParity — PHASE.md mandatory P10 gate.
//
// Loads testdata/specs/boxplot.json, materialises the inline data
// via build + execute (which routes through the compile/inmem
// backend whose AGG_PERCENTILE implementation P04 already proved
// matches Pulse's via TestPrismAggregateValueParity), and asserts
// the encoder's per-group q1 / median / q3 values match what an
// independent R-7 percentile call returns on the same raw values.
//
// Per D004 Pulse v0.8.4 exposes no in-memory cohort constructor so
// we cannot call pulse.Process directly on inline data — the parity
// proxy reduces to "the encoder's quantile math agrees with the
// canonical R-7 implementation", which the compile/inmem layer
// proves via independent tests.
func TestPrismBoxplotQuantileParity(t *testing.T) {
	root := findRepoRoot(t)
	specPath := filepath.Join(root, "testdata", "specs", "boxplot.json")
	body, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	s, err := spec.DecodeBytes(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	dag, tipID, err := build.Build(s, build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	tbl := res.Tables[tipID]

	// Build a band x scale + linear y scale matching what the encoder
	// would produce for the fixture.
	xBand := &scale.BandScale{Categories: []string{"a", "b"}, RangeMin: 40, RangeMax: 760, Padding: 0.1}
	yLin := &scale.LinearScale{DomainMin: 0, DomainMax: 1, RangeMin: 560, RangeMax: 20}
	in := marks.Inputs{
		Table:  tbl,
		X:      marks.Channel{Field: "group", Scale: xBand},
		Y:      marks.Channel{Field: "score", Scale: yLin},
		Layout: scene.Rect{W: 800, H: 600},
		Style:  scene.Style{},
	}
	summaries, err := marks.ComputeBoxplotSummaries(in)
	if err != nil {
		t.Fatalf("ComputeBoxplotSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("len summaries = %d, want 2", len(summaries))
	}

	// Independent R-7 percentile on the raw data per group.
	rawByGroup := map[string][]float64{}
	groupCol, _ := tbl.Column("group")
	scoreCol, _ := tbl.Column("score")
	for i := 0; i < tbl.NumRows(); i++ {
		g, _ := groupCol.ValueAt(i).(string)
		var v float64
		switch x := scoreCol.ValueAt(i).(type) {
		case float64:
			v = x
		case int64:
			v = float64(x)
		}
		rawByGroup[g] = append(rawByGroup[g], v)
	}
	for _, s := range summaries {
		rawSorted := append([]float64{}, rawByGroup[s.Group]...)
		sort.Float64s(rawSorted)
		wantQ1 := r7Percentile(rawSorted, 0.25)
		wantMedian := r7Percentile(rawSorted, 0.50)
		wantQ3 := r7Percentile(rawSorted, 0.75)
		if math.Abs(s.Q1-wantQ1) > 1e-9 {
			t.Errorf("group %q Q1: encoder=%g want=%g", s.Group, s.Q1, wantQ1)
		}
		if math.Abs(s.Median-wantMedian) > 1e-9 {
			t.Errorf("group %q Median: encoder=%g want=%g", s.Group, s.Median, wantMedian)
		}
		if math.Abs(s.Q3-wantQ3) > 1e-9 {
			t.Errorf("group %q Q3: encoder=%g want=%g", s.Group, s.Q3, wantQ3)
		}
	}
}

// r7Percentile mirrors compile/inmem.percentile (R-7 linear
// interpolation between order statistics). Kept locally to avoid
// importing an internal helper.
func r7Percentile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := q * float64(len(sorted)-1)
	low := int(math.Floor(rank))
	high := int(math.Ceil(rank))
	if low == high {
		return sorted[low]
	}
	return sorted[low] + (rank-float64(low))*(sorted[high]-sorted[low])
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found")
		}
		dir = parent
	}
}
