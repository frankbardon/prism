// transform_executable_sync_test.go keeps validate's PRISM_SPEC_067 rule
// (validate/rules/transform_executable.go) honest against the thing it
// describes: whether running a spec that names a transform actually
// produces a table.
//
// Why this exists. validate/ may not import plan/ or compile/ — validate
// is no-execute by structural ban — so the rule cannot read the backend's
// dispatch capability at runtime and has to state it in a table. A stated
// table rots, and this one was born rotten: the story that commissioned
// the rule named `join`, `union` and `pivot` as the un-executable set,
// and `join` / `union` both run fine (their Execute bodies live on the
// nodes themselves, in plan/nodes/join_execute.go and union_execute.go,
// and never migrated to the in-memory backend). `unpivot` had been on
// that list too until its executor landed days earlier.
//
// So the table is not trusted. This gate drives one minimal spec per
// transform variant through the real planner and the real in-memory
// backend and requires the rule's verdict to match what happens. It
// fails just as loudly when a transform the rule rejects starts working
// — the next person to implement `pivot` — as when one it accepts stops.
//
// Three invariants:
//
//  1. COVERAGE — every variant on spec.Transform has a probe spec here.
//     The variants are enumerated by reflection, so a seventeenth cannot
//     slip through unprobed.
//  2. TRUTH — for each probe, "the rule emits PRISM_SPEC_067" and "the
//     execution fails as not-implemented" agree.
//  3. END TO END — executability is measured through plan.Execute, not
//     by reading the backend's switch. A transform needs BOTH halves: an
//     executor, and (for a backend-routed node) the SetBackend wiring
//     plan/build installs. An executor with no wiring is dead code and
//     the transform still fails; only an end-to-end probe sees that.
package gates

import (
	goerrors "errors"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
	"github.com/frankbardon/prism/validate/rules"
)

// transformProbeSpecs maps a spec.Transform variant's discriminator key
// to a minimal spec exercising it. Each one must reach the transform
// node at execute time; a probe that fails upstream (bad field, wrong
// column kind) would report the transform as executable by accident, so
// keep them runnable.
//
// Keys are the lowercased spec.Transform field names — the coverage
// check derives the expected set that way rather than from a list.
var transformProbeSpecs = map[string]string{
	"filter":     probeSpec(`{"filter": {"op": "gt", "field": "v", "value": 0}}`),
	"calculate":  probeSpec(`{"calculate": {"op": "add", "operands": [{"field": "v"}, {"literal": 1}]}, "as": "v2"}`),
	"aggregate":  probeSpec(`{"aggregate": [{"op": "sum", "field": "v", "as": "vs"}], "groupby": ["k"]}`),
	"bin":        probeSpec(`{"bin": true, "field": "v", "as": "vb"}`),
	"window":     probeSpec(`{"window": [{"op": "rank", "as": "r"}]}`),
	"pivot":      probeSpec(`{"pivot": "m", "value": "v", "groupby": ["k"]}`),
	"unpivot":    probeSpec(`{"unpivot": ["v"]}`),
	"sample":     probeSpec(`{"sample": 1}`),
	"sort":       probeSpec(`{"sort": [{"field": "v", "order": "descending"}]}`),
	"limit":      probeSpec(`{"limit": 1}`),
	"crosstab":   probeSpec(`{"crosstab": {"rows": [{"field": "k"}], "columns": [{"field": "m"}], "cell": {"aggregate": "sum", "field": "v"}}}`),
	"regression": probeSpec(`{"regression": {"target": "v", "predictors": ["v"]}}`),
	"timeunit":   probeSpec(`{"timeunit": "month", "field": "t", "as": "tm"}`),
	"stack":      probeSpec(`{"stack": "v", "groupby": ["k"]}`),

	// join / union need a second dataset to reach for.
	"join": `{
      "$schema": "urn:prism:schema:v1:spec",
      "datasets": {
        "left": {"values": [{"k": "a", "v": 1}]},
        "right": {"values": [{"k": "a", "w": 2}]}
      },
      "data": {"name": "left"},
      "transform": [{"join": "inner", "with": "right", "on": "k"}],
      "mark": {"type": "bar"},
      "encoding": {
        "x": {"field": "k", "type": "nominal"},
        "y": {"field": "w", "type": "quantitative"}
      }
    }`,
	"union": `{
      "$schema": "urn:prism:schema:v1:spec",
      "datasets": {
        "a": {"values": [{"k": "a", "v": 1}]},
        "b": {"values": [{"k": "b", "v": 2}]}
      },
      "data": {"name": "a"},
      "transform": [{"union": ["a", "b"]}],
      "mark": {"type": "bar"},
      "encoding": {
        "x": {"field": "k", "type": "nominal"},
        "y": {"field": "v", "type": "quantitative"}
      }
    }`,
}

// probeSpec wraps one transform object in a minimal bar chart over a
// two-row inline table. `t` is declared date-typed and carries epoch-day
// numbers so the timeunit probe has a real date column to truncate.
func probeSpec(transform string) string {
	return `{
      "$schema": "urn:prism:schema:v1:spec",
      "data": {
        "values": [
          {"k": "a", "m": "x", "v": 1, "t": 19724},
          {"k": "b", "m": "y", "v": 2, "t": 19755}
        ],
        "fields": [
          {"name": "k", "type": "string"},
          {"name": "m", "type": "string"},
          {"name": "v", "type": "float"},
          {"name": "t", "type": "date"}
        ]
      },
      "transform": [` + transform + `],
      "mark": {"type": "bar"},
      "encoding": {
        "x": {"field": "k", "type": "nominal"},
        "y": {"field": "v", "type": "quantitative"}
      }
    }`
}

// TestPrismTransformExecutableRuleMatchesBackend is the cross-check.
func TestPrismTransformExecutableRuleMatchesBackend(t *testing.T) {
	// COVERAGE — every variant spec.Transform carries has a probe.
	tt := reflect.TypeOf(spec.Transform{})
	for i := 0; i < tt.NumField(); i++ {
		key := strings.ToLower(tt.Field(i).Name)
		if _, ok := transformProbeSpecs[key]; !ok {
			t.Errorf("spec.Transform variant %s has no probe spec; add one so the rule's verdict for %q can be checked", tt.Field(i).Name, key)
		}
	}
	for key := range transformProbeSpecs {
		found := false
		for i := 0; i < tt.NumField(); i++ {
			if strings.ToLower(tt.Field(i).Name) == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("probe spec %q names no spec.Transform variant", key)
		}
	}

	for key, body := range transformProbeSpecs {
		t.Run(key, func(t *testing.T) {
			s, err := spec.DecodeBytes([]byte(body))
			if err != nil {
				t.Fatalf("decode probe: %v", err)
			}

			ruleRejects := false
			for _, ae := range (rules.TransformExecutable{}).Check(s, validate.EmptyLookup{}) {
				if ae.Code == "PRISM_SPEC_067" {
					ruleRejects = true
				}
			}

			backendMissing, execErr := probeNotImplemented(t, s)

			// A probe that fails for an unrelated reason would report
			// its transform as executable by accident, so the probes
			// that are meant to run must actually run.
			if !backendMissing && execErr != nil {
				t.Errorf("probe for %q failed for a reason other than a missing implementation: %v.\n"+
					"Fix the probe — as written it no longer proves anything about %q.", key, execErr, key)
			}

			if ruleRejects != backendMissing {
				switch {
				case ruleRejects:
					t.Fatalf("transform %q: PRISM_SPEC_067 rejects it, but executing it works now (execute error: %v).\n"+
						"Someone implemented it — flip its entry in transformExecutors "+
						"(validate/rules/transform_executable.go) to true and update the PRISM_SPEC_067 fixups.", key, execErr)
				default:
					t.Fatalf("transform %q: PRISM_SPEC_067 accepts it, but executing it fails as not-implemented (%v).\n"+
						"A spec naming it passes validate and then dies at execute — flip its entry in "+
						"transformExecutors (validate/rules/transform_executable.go) to false.", key, execErr)
				}
			}
		})
	}
}

// probeNotImplemented builds and executes s against the in-memory
// backend and reports whether any node failed with the not-implemented
// error shape.
//
// The signal has to be precise: plan.Execute's codeFor falls back to
// PRISM_COMPILE_001 for ANY node error that is not an AppError, so a
// bad probe (a timeunit over a string column, say) reports the same
// code as a missing implementation. Only the two notImplemented
// constructors — plan/nodes/stub.go and compile/inmem/backend.go —
// raise it as a typed AppError carrying NodeType and Phase, so that is
// what is matched.
func probeNotImplemented(t *testing.T, s *spec.Spec) (bool, error) {
	t.Helper()
	dag, _, err := build.Build(s, build.Options{
		FS:      afero.NewMemMapFs(),
		Backend: inmem.New(),
	})
	if err != nil {
		t.Fatalf("build probe plan: %v", err)
	}
	res, err := plan.Execute(t.Context(), dag, plan.ExecOpts{Workers: 1})
	if err != nil {
		t.Fatalf("execute probe plan: %v", err)
	}
	var firstErr error
	for _, ne := range res.Errors {
		if firstErr == nil {
			firstErr = ne.Err
		}
		var ae *prismerrors.AppError
		if !goerrors.As(ne.Err, &ae) || ae.Code != "PRISM_COMPILE_001" {
			continue
		}
		if _, ok := ae.Context["NodeType"]; !ok {
			continue
		}
		if _, ok := ae.Context["Phase"]; !ok {
			continue
		}
		return true, ne.Err
	}
	return false, firstErr
}
