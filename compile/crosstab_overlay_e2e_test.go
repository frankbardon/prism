package compile_test

import (
	"context"
	"math"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// TestCrosstabShareOfRowOverlay runs a brand_id × age crosstab with a
// share_of_row overlay against tiny.pulse end-to-end and asserts the
// overlay column is present and the shares sum to 1.0 within each row
// group (the defining property of share-of-row). This exercises the
// matrix-shape overlay path: the base + overlay matrices are
// coordinate-joined into long rows, so a correct join is required for
// the per-row shares to reconstruct.
func TestCrosstabShareOfRowOverlay(t *testing.T) {
	cohortPath := fixturePath(t)
	fs := afero.NewOsFs()

	s := &spec.Spec{
		Data: &spec.Data{Source: cohortPath},
		Transform: []spec.Transform{
			{Crosstab: &spec.CrosstabTransform{Crosstab: spec.CrosstabBody{
				Rows:    []spec.CrosstabGroup{{Field: "brand_id"}},
				Columns: []spec.CrosstabGroup{{Field: "age", Type: "category"}},
				// count needs a field in the crosstab cell context;
				// count(field) skips nulls, which is what we want here.
				Cell:     spec.CrosstabCell{Aggregate: "count", Field: "score", As: "n"},
				Overlays: []spec.CrosstabOverlay{{Kind: "share_of_row", As: "row_share"}},
			}}},
		},
	}

	dag, _, err := build.Build(s, build.Options{
		FS:       fs,
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{Workers: 1})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("execute had %d node errors; first = %v", len(res.Errors), res.Errors[0])
	}
	final := finalTable(dag, res)
	if final == nil {
		t.Fatal("no tip table")
	}

	brandCol, ok := final.Column("brand_id")
	if !ok {
		t.Fatal("missing brand_id column")
	}
	shareCol, ok := final.Column("row_share")
	if !ok {
		t.Fatal("missing row_share overlay column")
	}
	nCol, ok := final.Column("n")
	if !ok {
		t.Fatal("missing n cell column")
	}

	// Sum the per-cell row_share within each brand. Each brand's shares
	// must total 1.0 (cells with a present count). Empty cells carry
	// share 0, so the sum is unaffected.
	perBrand := map[string]float64{}
	total := 0.0
	for i := 0; i < final.NumRows(); i++ {
		brand, _ := brandCol.ValueAt(i).(string)
		share, _ := shareCol.ValueAt(i).(float64)
		count, _ := nCol.ValueAt(i).(float64)
		perBrand[brand] += share
		total += count
	}
	if total == 0 {
		t.Fatal("crosstab produced no records — fixture or cell op broken")
	}
	if len(perBrand) == 0 {
		t.Fatal("no brand groups in output")
	}
	for brand, sum := range perBrand {
		if math.Abs(sum-1.0) > 1e-6 {
			t.Errorf("brand %q row_share sum = %v, want 1.0", brand, sum)
		}
	}
}
