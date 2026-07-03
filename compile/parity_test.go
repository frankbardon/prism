package compile_test

import (
	"context"
	"math"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	specpkg "github.com/frankbardon/prism/spec"
)

// frozenAggregateParity is the per-brand value Pulse produced for each
// aggregate alias over the tiny cohort (groupby brand_id, field score),
// captured via pulse.Process at the point Pulse left the ingestion path.
// distinct / mode are excluded (non-numeric semantics on score); wmean /
// ratio need sibling columns the fixture lacks. See inline_fixture_test.go.
var frozenAggregateParity = map[string]map[string]float64{
	"ci0":        {"alpha": 0.48800750120297964, "beta": 0.4834411379461078, "delta": 0.49234532326657476, "gamma": 0.4670611330908475},
	"ci1":        {"alpha": 0.5189373283927206, "beta": 0.5141967503760725, "delta": 0.5508974237545249, "gamma": 0.5075610619306765},
	"count":      {"alpha": 383.0, "beta": 314.0, "delta": 103.0, "gamma": 200.0},
	"frequency":  {"alpha": 1.0, "beta": 1.0, "delta": 2.0, "gamma": 1.0},
	"kurtosis":   {"alpha": -0.128547318058295, "beta": 0.054680995778235264, "delta": 1.5364883560948295, "gamma": -0.21899457910906373},
	"max":        {"alpha": 0.9927196416020506, "beta": 0.9802453973404129, "delta": 0.9191491216547075, "gamma": 0.8827701202056496},
	"mean":       {"alpha": 0.5034724147978498, "beta": 0.49881894416109007, "delta": 0.5216213735105499, "gamma": 0.4873110975107617},
	"median":     {"alpha": 0.5109896264180319, "beta": 0.49612714980150013, "delta": 0.5316102948001258, "gamma": 0.48548358826625204},
	"min":        {"alpha": 0.11749990261053342, "beta": 0.133629153704558, "delta": 0.0, "gamma": 0.13159351029785515},
	"null_count": {"alpha": 0.0, "beta": 0.0, "delta": 0.0, "gamma": 0.0},
	"q1":         {"alpha": 0.4073671354362559, "beta": 0.41331734534816855, "delta": 0.4529092535849327, "gamma": 0.3867090562811494},
	"q3":         {"alpha": 0.6061653449168604, "beta": 0.5926348434023753, "delta": 0.6373723832543785, "gamma": 0.587602854209222},
	"range":      {"alpha": 0.8752197389915172, "beta": 0.846616243635855, "delta": 0.9191491216547075, "gamma": 0.7511766099077944},
	"skewness":   {"alpha": -0.013092448558578176, "beta": 0.04084784597778826, "delta": -0.6984367756542236, "gamma": 0.03533235560422917},
	"stdev":      {"alpha": 0.15421658731126814, "beta": 0.13880926721571848, "delta": 0.15085665672950635, "gamma": 0.14574803615051243},
	"sum":        {"alpha": 192.82993486757647, "beta": 156.62914846658228, "delta": 53.72700147158665, "gamma": 97.46221950215234},
	"variance":   {"alpha": 0.023782755801933987, "beta": 0.019268012664964737, "delta": 0.022757730879604112, "gamma": 0.021242490041731083},
}

// TestPrismAggregateValueParity is the frozen parity gate. For every
// Pulse-backed aggregate alias, computing the aggregate via Prism's
// in-memory backend over the tiny cohort's inline rows must equal the
// value Pulse produced for the same request (captured in
// frozenAggregateParity). Tolerance is 1e-6.
func TestPrismAggregateValueParity(t *testing.T) {
	fs := afero.NewMemMapFs()
	resolver := tinyResolver(t)

	for alias, want := range frozenAggregateParity {
		s := &specpkg.Spec{
			Data: &specpkg.Data{Source: tinyRef},
			Transform: []specpkg.Transform{
				{Aggregate: &specpkg.AggregateTransform{
					Groupby:   []string{"brand_id"},
					Aggregate: []specpkg.AggregateOp{{Op: alias, Field: "score", As: "value"}},
				}},
			},
		}
		dag, _, err := build.Build(s, build.Options{
			FS:       fs,
			Resolver: resolver,
			Backend:  inmem.New(),
		})
		if err != nil {
			t.Fatalf("%s: build: %v", alias, err)
		}
		res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{Workers: 1})
		if err != nil {
			t.Fatalf("%s: execute: %v", alias, err)
		}
		final := finalTable(dag, res)
		if final == nil {
			t.Fatalf("%s: no tip table", alias)
		}
		got := tableToBrandValueMap(t, final, "value")

		if len(got) != len(want) {
			t.Errorf("%s: group count Prism=%d frozen=%d", alias, len(got), len(want))
		}
		for brand, wantVal := range want {
			gotVal, ok := got[brand]
			if !ok {
				t.Errorf("%s: brand %q missing from Prism output", alias, brand)
				continue
			}
			if math.Abs(gotVal-wantVal) > 1e-6 {
				t.Errorf("%s/%s: Prism=%v frozen(Pulse)=%v (delta=%g)",
					alias, brand, gotVal, wantVal, math.Abs(gotVal-wantVal))
			}
		}
	}
}
