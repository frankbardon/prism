package encode_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
	_ "github.com/frankbardon/prism/validate/rules"
)

func jsonUnmarshalImpl(body []byte, dst any) error {
	return json.Unmarshal(body, dst)
}

// fixtureLookup builds a minimal SchemaLookup from a spec's inline
// data block. Mirrors the lookupFor helper in validate's tests
// without dragging in the pulse-backed branch (selection fixtures
// only use inline values).
func fixtureLookup(s *spec.Spec) validate.SchemaLookup {
	staticLookup := validate.NewStaticLookup()
	register := func(name string, ds *spec.Data) {
		if ds == nil || name == "" || len(ds.Values) == 0 {
			return
		}
		shim := &validate.PulseSchemaShim{Name: name}
		seen := map[string]bool{}
		for _, row := range ds.Values {
			for k, v := range row {
				if seen[k] {
					continue
				}
				seen[k] = true
				shim.Fields = append(shim.Fields, validate.FieldShim{
					Name: k, Type: inferFixtureType(v),
				})
			}
		}
		if len(shim.Fields) == 0 {
			return
		}
		staticLookup.Register(name, shim)
	}
	if s.Data != nil {
		register(s.Data.Name, s.Data)
	}
	for name, ds := range s.Datasets {
		register(name, ds)
	}
	return staticLookup
}

func inferFixtureType(v any) string {
	switch v.(type) {
	case float64, float32, int, int64:
		return "quantitative"
	case bool:
		return "nominal"
	default:
		return "nominal"
	}
}

// TestPrismSelectionFixturesEncode walks every spec under
// examples/specs/selections/ + the two P01 selection fixtures at
// examples/specs/ and asserts they validate clean, build, execute,
// encode, and surface at least one Scene.Selections entry per
// declared selection.
//
// Drives the P13 demoable fixtures as a single gate so a regression
// in any of (validator, planner, encoder) surfaces immediately.
func TestPrismSelectionFixturesEncode(t *testing.T) {
	root := repoRoot(t)
	cases := []string{
		filepath.Join(root, "examples", "specs", "selection_point.json"),
		filepath.Join(root, "examples", "specs", "selection_interval.json"),
	}
	// Append every JSON under selections/.
	selDir := filepath.Join(root, "examples", "specs", "selections")
	entries, err := os.ReadDir(selDir)
	if err != nil {
		t.Fatalf("read selections dir: %v", err)
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		if filepath.Ext(ent.Name()) != ".json" {
			continue
		}
		cases = append(cases, filepath.Join(selDir, ent.Name()))
	}

	shape, err := validate.NewShapeValidator()
	if err != nil {
		t.Fatalf("NewShapeValidator: %v", err)
	}
	sem := validate.NewDefaultSemanticValidator()

	for _, path := range cases {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			// Shape + semantic validation must pass clean.
			var raw any
			if err := jsonUnmarshalFixture(body, &raw); err != nil {
				t.Fatalf("re-parse: %v", err)
			}
			if errs := shape.Validate(raw); len(errs) > 0 {
				t.Fatalf("shape errors: %+v", errs)
			}
			s, err := spec.DecodeBytes(body)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if errs := sem.Validate(s, fixtureLookup(s)); len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("semantic error: %s %s", e.Code, e.Message)
				}
				t.FailNow()
			}

			// Build → execute → encode.
			dag, tipID, err := build.Build(s, build.Options{
				FS:       afero.NewOsFs(),
				Resolver: resolve.New(nil),
				Backend:  inmem.New(),
			})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if len(res.Errors) > 0 {
				t.Fatalf("node errors: %+v", res.Errors)
			}
			doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{})
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}

			// Encode must surface at least one Selection per declared.
			sels := doc.Grid.Cells[0].Scene.Selections
			if len(sels) != len(s.Selection) {
				t.Fatalf("Selections len = %d, want %d (one per spec.Selection entry)",
					len(sels), len(s.Selection))
			}
			for _, sel := range sels {
				if sel.Reactive != scene.ReactiveClient {
					t.Errorf("selection %q Reactive = %q, want client (D004 default)", sel.ID, sel.Reactive)
				}
				switch sel.Kind {
				case scene.SelectionPoint:
					if sel.On == "" {
						t.Errorf("selection %q (point) On is empty; should default to click", sel.ID)
					}
				case scene.SelectionInterval:
					if sel.On != scene.EventBrush {
						t.Errorf("selection %q (interval) On = %q, want brush", sel.ID, sel.On)
					}
					if len(sel.Channels) == 0 {
						t.Errorf("selection %q (interval) Channels is empty", sel.ID)
					}
				default:
					t.Errorf("selection %q has unknown Kind %q", sel.ID, sel.Kind)
				}
			}
		})
	}
}

// jsonUnmarshalFixture is a thin wrapper around encoding/json that
// keeps the imports localised at the bottom of the file.
func jsonUnmarshalFixture(body []byte, dst any) error {
	return jsonUnmarshalImpl(body, dst)
}
