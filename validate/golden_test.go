package validate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
	_ "github.com/frankbardon/prism/validate/rules"
)

// TestPrismSpecGoldensValidateOffline is the end-to-end gallery harness.
// Every fixture under testdata/specs/ (positive) must validate clean.
// Every fixture under testdata/specs/invalid/ (negative) must fire at
// least one error whose code matches the fixture's expected code,
// derived from the file basename via the negativeCodeMap below.
//
// The validator is built fresh for the test run and is "offline" by
// construction: the JSON Schema bundle is embedded in the binary and no
// network is touched.
func TestPrismSpecGoldensValidateOffline(t *testing.T) {
	shape, err := validate.NewShapeValidator()
	if err != nil {
		t.Fatalf("NewShapeValidator: %v", err)
	}
	sem := validate.NewDefaultSemanticValidator()

	// Resolve testdata path relative to the repo root.
	root := repoRoot(t)
	positives := filepath.Join(root, "testdata", "specs")
	negatives := filepath.Join(root, "testdata", "specs", "invalid")

	// Positives.
	posEntries, err := os.ReadDir(positives)
	if err != nil {
		t.Fatalf("read positives dir: %v", err)
	}
	for _, ent := range posEntries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		t.Run("positive/"+ent.Name(), func(t *testing.T) {
			p := filepath.Join(positives, ent.Name())
			body, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			typed, err := spec.DecodeBytes(body)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			var raw any
			if err := json.Unmarshal(body, &raw); err != nil {
				t.Fatalf("re-parse: %v", err)
			}
			if shapeErrs := shape.Validate(raw); len(shapeErrs) > 0 {
				t.Fatalf("shape errors on positive fixture: %+v", shapeErrs)
			}
			lookup := lookupFor(typed)
			if semErrs := sem.Validate(typed, lookup); len(semErrs) > 0 {
				for _, e := range semErrs {
					t.Errorf("semantic error on positive fixture: %s %s", e.Code, e.Message)
				}
			}
		})
	}

	// Negatives.
	negEntries, err := os.ReadDir(negatives)
	if err != nil {
		t.Fatalf("read negatives dir: %v", err)
	}
	for _, ent := range negEntries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		t.Run("negative/"+ent.Name(), func(t *testing.T) {
			wantCode, ok := negativeCodeMap[ent.Name()]
			if !ok {
				t.Skipf("no expected code mapped for %s", ent.Name())
			}
			p := filepath.Join(negatives, ent.Name())
			body, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			typed, err := spec.DecodeBytes(body)
			if err != nil {
				// strict decode failure counts as PRISM_SPEC_009-ish; treat
				// any error here as satisfying the "rejects bad spec" gate.
				if wantCode == "PRISM_SPEC_009" {
					return
				}
				t.Fatalf("decode: %v (wanted code %s)", err, wantCode)
			}
			var raw any
			if err := json.Unmarshal(body, &raw); err != nil {
				t.Fatalf("re-parse: %v", err)
			}
			shapeErrs := shape.Validate(raw)
			lookup := lookupFor(typed)
			semErrs := sem.Validate(typed, lookup)
			all := append([]string{}, codesOfShape(shapeErrs)...)
			for _, e := range semErrs {
				all = append(all, e.Code)
			}
			if !contains(all, wantCode) {
				t.Fatalf("expected code %s, got: shape=%+v semantic=%+v",
					wantCode, shapeErrs, semErrs)
			}
		})
	}
}

// negativeCodeMap maps fixture filename → expected PRISM_SPEC_* code.
// Adding a new negative fixture requires adding the mapping here.
var negativeCodeMap = map[string]string{
	"unknown_field.json":            "PRISM_SPEC_001",
	"mean_on_categorical.json":      "PRISM_SPEC_002",
	"theta_on_bar.json":             "PRISM_SPEC_003",
	"selection_undefined.json":      "PRISM_SPEC_004",
	"dataset_undefined.json":        "PRISM_SPEC_005",
	"bad_expression.json":           "PRISM_SPEC_006",
	"log_scale_on_categorical.json": "PRISM_SPEC_007",
	"pie_without_theta.json":        "PRISM_SPEC_008",
	"bad_schema_ref.json":           "PRISM_SPEC_009",
}

// codesOfShape extracts a stable string slice of shape-error codes.
// Shape validator errors are not coded; we collapse to "PRISM_SPEC_009"
// to denote a "structural error" code that the negative fixtures can
// target uniformly via the map.
func codesOfShape(es []validate.ShapeError) []string {
	if len(es) == 0 {
		return nil
	}
	return []string{"PRISM_SPEC_009"}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func repoRoot(t *testing.T) string {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			t.Fatalf("go.mod not found in any parent of %s", cwd)
		}
		cwd = parent
	}
}

// lookupFor mirrors the CLI's inline-dataset lookup builder: anything
// declared in datasets or in data.values + data.fields registers a
// minimal schema shim.
func lookupFor(s *spec.Spec) validate.SchemaLookup {
	lookup := validate.NewStaticLookup()
	if s == nil {
		return lookup
	}
	register := func(name string, ds *spec.Data) {
		if ds == nil || name == "" {
			return
		}
		shim := &validate.PulseSchemaShim{Name: name}
		if len(ds.Values) > 0 {
			seen := map[string]bool{}
			for _, row := range ds.Values {
				for k, v := range row {
					if seen[k] {
						continue
					}
					seen[k] = true
					shim.Fields = append(shim.Fields, validate.FieldShim{
						Name: k, Type: inferType(v),
					})
				}
			}
		}
		for _, f := range ds.Fields {
			shim.Fields = append(shim.Fields, validate.FieldShim{Name: f.Name, Type: storageMeasure(f.Type)})
		}
		if len(shim.Fields) == 0 {
			return
		}
		lookup.Register(name, shim)
	}
	if s.Data != nil {
		register(s.Data.Name, s.Data)
	}
	for name, ds := range s.Datasets {
		register(name, ds)
	}
	return lookup
}

func inferType(v any) string {
	switch v.(type) {
	case float64, float32, int, int64, int32:
		return "quantitative"
	case string:
		return "nominal"
	case bool:
		return "nominal"
	default:
		return ""
	}
}

func storageMeasure(storage string) string {
	switch strings.ToLower(storage) {
	case "int", "int8", "int16", "int32", "int64", "float", "float32", "float64":
		return "quantitative"
	case "date", "datetime", "duration":
		return "temporal"
	default:
		return "nominal"
	}
}
