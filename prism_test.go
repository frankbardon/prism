package prism_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	prism "github.com/frankbardon/prism"
)

const inlineBarSpec = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "name": "brand_scores",
    "values": [
      {"brand_id": "alpha", "score": 0.42},
      {"brand_id": "beta",  "score": 0.71},
      {"brand_id": "gamma", "score": 0.55}
    ]
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "brand_id", "type": "nominal"},
    "y": {"field": "score",    "type": "quantitative"}
  }
}`

func TestCompileFlattensSceneDoc(t *testing.T) {
	plan, err := prism.CompileJSON(context.Background(), []byte(inlineBarSpec), prism.CompileOptions{})
	if err != nil {
		t.Fatalf("CompileJSON: %v", err)
	}
	if plan == nil {
		t.Fatal("CompileJSON returned nil plan with no error")
	}
	if plan.Scene == nil {
		t.Fatal("Scene missing")
	}

	if len(plan.Marks) != 1 {
		t.Fatalf("Marks len = %d want 1", len(plan.Marks))
	}
	mark := plan.Marks[0]
	if mark.Type != "rect" {
		t.Errorf("Marks[0].Type = %q want rect (bar encodes as rect)", mark.Type)
	}
	if mark.InstanceCount != 3 {
		t.Errorf("Marks[0].InstanceCount = %d want 3", mark.InstanceCount)
	}

	if len(plan.Scales) < 2 {
		t.Errorf("Scales len = %d want >=2 (x + y)", len(plan.Scales))
	}
	var sawX, sawY bool
	for _, sc := range plan.Scales {
		if sc.Channel == "x" {
			sawX = true
		}
		if sc.Channel == "y" {
			sawY = true
		}
	}
	if !sawX || !sawY {
		t.Errorf("missing x/y scale: x=%v y=%v", sawX, sawY)
	}

	if len(plan.Data) == 0 {
		t.Error("Data bindings empty; expected the layer's inline dataset")
	}

	if plan.Layout.Width == 0 || plan.Layout.Height == 0 {
		t.Errorf("Layout = %+v; expected non-zero dimensions", plan.Layout)
	}
}

func TestCompileJSONShape(t *testing.T) {
	plan, err := prism.CompileJSON(context.Background(), []byte(inlineBarSpec), prism.CompileOptions{})
	if err != nil {
		t.Fatalf("CompileJSON: %v", err)
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := string(raw)
	for _, key := range []string{
		`"marks":`,
		`"scales":`,
		`"data":`,
		`"layout":`,
		`"diagnostics":`,
		`"scene":`,
	} {
		if !strings.Contains(body, key) {
			t.Errorf("CompiledPlan JSON missing %s", key)
		}
	}
}

func TestCompileRejectsNilSpec(t *testing.T) {
	_, err := prism.Compile(context.Background(), nil, prism.CompileOptions{})
	if err == nil {
		t.Fatal("expected error for nil spec")
	}
}

func TestCompileJSONReportsDecodeError(t *testing.T) {
	_, err := prism.CompileJSON(context.Background(), []byte(`{`), prism.CompileOptions{})
	if err == nil {
		t.Fatal("expected decode error")
	}
}
