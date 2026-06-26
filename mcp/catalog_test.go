//go:build !js

package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/rpc"
)

// TestToolsCatalog asserts Tools(cfg) returns all four descriptors with
// non-empty reflected schemas, that the examples_search input schema exposes
// only "query" (the json:"-" Root/FS config fields are NOT reflected), and
// that each Invoke round-trips raw args → typed handler → output.
func TestToolsCatalog(t *testing.T) {
	cfg := Config{ServerName: "prism-test", Version: "9.9.9"}
	tools := Tools(cfg)
	if len(tools) != 4 {
		t.Fatalf("Tools returned %d descriptors; want 4", len(tools))
	}

	want := []string{"prism_plot", "prism_validate", "prism_describe", "prism_examples_search"}
	byName := map[string]ToolDescriptor{}
	for _, d := range tools {
		if d.Name == "" {
			t.Errorf("descriptor with empty name: %+v", d)
		}
		if d.Description == "" {
			t.Errorf("%s: empty description", d.Name)
		}
		if len(d.InputSchema) == 0 {
			t.Errorf("%s: empty input schema", d.Name)
		}
		if len(d.OutputSchema) == 0 {
			t.Errorf("%s: empty output schema", d.Name)
		}
		if d.Invoke == nil {
			t.Errorf("%s: nil Invoke", d.Name)
		}
		byName[d.Name] = d
	}
	for _, n := range want {
		if _, ok := byName[n]; !ok {
			t.Errorf("missing tool %q", n)
		}
	}

	// examples_search input schema must surface only "query" — never the
	// json:"-" Root/FS config fields.
	esIn := string(byName["prism_examples_search"].InputSchema)
	if !strings.Contains(esIn, "query") {
		t.Errorf("examples_search input schema missing 'query': %s", esIn)
	}
	for _, banned := range []string{"Root", "FS", `"-"`} {
		if strings.Contains(esIn, banned) {
			t.Errorf("examples_search input schema leaks %q: %s", banned, esIn)
		}
	}

	// Invoke round-trip: validate a known-good spec through the erased path.
	f := &rpc.PrismServer{Fs: afero.NewMemMapFs()}
	raw := json.RawMessage(`{"spec":` + jsonString(fixtureSpec) + `}`)
	out, err := byName["prism_validate"].Invoke(context.Background(), f, raw)
	if err != nil {
		t.Fatalf("validate Invoke: %v", err)
	}
	vout, ok := out.(ValidateOutput)
	if !ok {
		t.Fatalf("validate Invoke returned %T; want ValidateOutput", out)
	}
	if !vout.Ok {
		t.Errorf("validate Invoke ok=false; errors=%+v", vout.Errors)
	}

	// describe round-trips to a typed DescribeOutput summary.
	dout, err := byName["prism_describe"].Invoke(context.Background(), f, raw)
	if err != nil {
		t.Fatalf("describe Invoke: %v", err)
	}
	dsum, ok := dout.(DescribeOutput)
	if !ok {
		t.Fatalf("describe Invoke returned %T; want DescribeOutput", dout)
	}
	if !strings.Contains(dsum.Summary, "bar chart") {
		t.Errorf("describe summary missing 'bar chart': %q", dsum.Summary)
	}
}

// TestExamplesSearchOverrideBakedIntoClosure confirms Tools(cfg) bakes the
// examples override from Config into the examples_search Invoke closure rather
// than reading a global: a query carrying no Root in its raw args still hits
// the configured on-disk corpus.
func TestExamplesSearchOverrideBakedIntoClosure(t *testing.T) {
	exFS := afero.NewMemMapFs()
	_ = afero.WriteFile(exFS, "fixtures/only_override.json",
		[]byte(`{"$schema":"urn:prism:schema:v1:spec","title":"override only","mark":"bar","encoding":{}}`), 0o644)

	tools := Tools(Config{ExamplesRoot: "fixtures/", ExamplesFS: exFS})
	var es ToolDescriptor
	for _, d := range tools {
		if d.Name == "prism_examples_search" {
			es = d
		}
	}
	out, err := es.Invoke(context.Background(), nil, json.RawMessage(`{"query":"override"}`))
	if err != nil {
		t.Fatalf("examples_search Invoke: %v", err)
	}
	res, ok := out.(ExamplesSearchOutput)
	if !ok {
		t.Fatalf("examples_search Invoke returned %T; want ExamplesSearchOutput", out)
	}
	if len(res.Examples) != 1 || res.Examples[0].Name != "only_override" {
		t.Fatalf("expected the single on-disk override fixture; got %+v", res.Examples)
	}
}

// jsonString quotes s as a JSON string literal for embedding in a raw payload.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
