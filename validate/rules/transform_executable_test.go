package rules

import (
	"reflect"
	"strings"
	"testing"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// checkTransformExecutable runs the rule alone against a spec decoded
// from JSON, so the fixtures exercise the real union decoder rather
// than hand-built variant pointers.
func checkTransformExecutable(t *testing.T, body string) []*errors.AppError {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return TransformExecutable{}.Check(s, validate.EmptyLookup{})
}

// transformSpec wraps one transform object in a minimal bar chart.
func transformSpec(transform string) string {
	return `{
      "$schema": "urn:prism:schema:v1:spec",
      "data": {"values": [{"k": "a", "m": "x", "v": 1}]},
      "transform": [` + transform + `],
      "mark": {"type": "bar"},
      "encoding": {
        "x": {"field": "k", "type": "nominal"},
        "y": {"field": "v", "type": "quantitative"}
      }
    }`
}

// TestPrismTransformExecutableRejectsUnbackedTransform pins the
// transforms that build a plan node nobody can execute. Today that is
// `pivot` alone.
func TestPrismTransformExecutableRejectsUnbackedTransform(t *testing.T) {
	cases := map[string]string{
		"pivot": `{"pivot": "m", "value": "v", "groupby": ["k"]}`,
	}
	for name, tr := range cases {
		t.Run(name, func(t *testing.T) {
			got := checkTransformExecutable(t, transformSpec(tr))
			if len(got) != 1 {
				t.Fatalf("want exactly 1 error, got %d: %v", len(got), got)
			}
			if got[0].Code != "PRISM_SPEC_067" {
				t.Fatalf("code = %s, want PRISM_SPEC_067", got[0].Code)
			}
			if got[0].Context["Transform"] != name {
				t.Fatalf("Context[Transform] = %v, want %q", got[0].Context["Transform"], name)
			}
			if got[0].Context["Path"] != "transform[0]" {
				t.Fatalf("Context[Path] = %v, want transform[0]", got[0].Context["Path"])
			}
		})
	}
}

// TestPrismTransformExecutableAcceptsWorkingTransforms is the half that
// proves the rule tracks reality rather than a hardcoded guess. Three
// of these were on the story's un-executable list or nearly so:
// `unpivot` had an executor landed only just before this rule, and
// `join` / `union` run through node-level Execute bodies that never
// migrated to the in-memory backend. All three draw.
func TestPrismTransformExecutableAcceptsWorkingTransforms(t *testing.T) {
	cases := map[string]string{
		"unpivot":    transformSpec(`{"unpivot": ["v"]}`),
		"filter":     transformSpec(`{"filter": {"op": "gt", "field": "v", "value": 0}}`),
		"aggregate":  transformSpec(`{"aggregate": [{"op": "sum", "field": "v", "as": "vs"}], "groupby": ["k"]}`),
		"sort":       transformSpec(`{"sort": [{"field": "v", "order": "descending"}]}`),
		"stack":      transformSpec(`{"stack": "v", "groupby": ["k"]}`),
		"crosstab":   transformSpec(`{"crosstab": {"rows": [{"field": "k"}], "columns": [{"field": "m"}], "cell": {"aggregate": "sum", "field": "v"}}}`),
		"regression": transformSpec(`{"regression": {"target": "v", "predictors": ["v"]}}`),
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
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if got := checkTransformExecutable(t, body); len(got) != 0 {
				t.Fatalf("%s must validate clean, got %v", name, got)
			}
		})
	}
}

// TestPrismTransformExecutableWalksCompositionChildren pins that a
// transform buried in a layer / concat / facet child is reported with a
// path naming the child, not silently skipped. The rule is worthless on
// a composite spec otherwise — `layer` is where a per-layer transform
// list usually lives.
func TestPrismTransformExecutableWalksCompositionChildren(t *testing.T) {
	cases := map[string]struct {
		body string
		path string
	}{
		"layer": {
			body: `{
              "$schema": "urn:prism:schema:v1:spec",
              "data": {"values": [{"k": "a", "m": "x", "v": 1}]},
              "layer": [
                {"mark": {"type": "bar"}, "encoding": {"x": {"field": "k", "type": "nominal"}}},
                {
                  "transform": [{"pivot": "m", "value": "v", "groupby": ["k"]}],
                  "mark": {"type": "line"},
                  "encoding": {"x": {"field": "k", "type": "nominal"}}
                }
              ]
            }`,
			path: "layer[1].transform[0]",
		},
		"vconcat": {
			body: `{
              "$schema": "urn:prism:schema:v1:spec",
              "data": {"values": [{"k": "a", "m": "x", "v": 1}]},
              "vconcat": [
                {
                  "transform": [{"pivot": "m", "value": "v", "groupby": ["k"]}],
                  "mark": {"type": "bar"},
                  "encoding": {"x": {"field": "k", "type": "nominal"}}
                }
              ]
            }`,
			path: "vconcat[0].transform[0]",
		},
		"facet_child_spec": {
			body: `{
              "$schema": "urn:prism:schema:v1:spec",
              "data": {"values": [{"k": "a", "m": "x", "v": 1}]},
              "facet": {"column": {"field": "k", "type": "nominal"}},
              "spec": {
                "transform": [{"pivot": "m", "value": "v", "groupby": ["k"]}],
                "mark": {"type": "bar"},
                "encoding": {"x": {"field": "k", "type": "nominal"}}
              }
            }`,
			path: "spec.transform[0]",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := checkTransformExecutable(t, tc.body)
			if len(got) != 1 {
				t.Fatalf("want exactly 1 error, got %d: %v", len(got), got)
			}
			if got[0].Context["Path"] != tc.path {
				t.Fatalf("Context[Path] = %v, want %q", got[0].Context["Path"], tc.path)
			}
		})
	}
}

// TestPrismTransformExecutableSilentOnTransformlessSpec pins the
// no-op: a spec with no transform block is reported exactly as before
// this rule existed.
func TestPrismTransformExecutableSilentOnTransformlessSpec(t *testing.T) {
	body := `{
      "$schema": "urn:prism:schema:v1:spec",
      "data": {"values": [{"k": "a", "v": 1}]},
      "mark": {"type": "bar"},
      "encoding": {
        "x": {"field": "k", "type": "nominal"},
        "y": {"field": "v", "type": "quantitative"}
      }
    }`
	if got := checkTransformExecutable(t, body); len(got) != 0 {
		t.Fatalf("want no errors, got %v", got)
	}
}

// TestPrismTransformExecutableCoversEveryVariant asserts the capability
// table names every variant spec.Transform carries, and that
// transformVariantName answers with the discriminator key for each.
// The union is the only source of truth for what a spec may say, so a
// seventeenth variant landing with no entry here would be treated as
// executable without anyone deciding that.
//
// Variants are enumerated by reflecting over spec.Transform rather than
// from a list written out here, because a list written out here rots
// exactly the way the capability table would.
//
// The cross-check that the entries are *correct* (rather than merely
// present) lives in internal/gates, which may import the planner and
// the backend this package may not.
func TestPrismTransformExecutableCoversEveryVariant(t *testing.T) {
	tt := reflect.TypeOf(spec.Transform{})
	if got, want := len(transformExecutors), tt.NumField(); got != want {
		t.Errorf("transformExecutors has %d entries, spec.Transform has %d variants", got, want)
	}
	for i := 0; i < tt.NumField(); i++ {
		f := tt.Field(i)
		want := strings.ToLower(f.Name)
		// A Transform with only this variant populated.
		v := reflect.New(tt).Elem()
		v.Field(i).Set(reflect.New(f.Type.Elem()))
		if got := transformVariantName(v.Interface().(spec.Transform)); got != want {
			t.Errorf("transformVariantName(%s set) = %q, want %q", f.Name, got, want)
		}
		if _, ok := transformExecutors[want]; !ok {
			t.Errorf("transform variant %q has no transformExecutors entry", want)
		}
	}
}
