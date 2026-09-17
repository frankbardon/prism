package rules

import (
	"strings"
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// leafSpec is a minimal, legal flat chart used as a composition child.
func leafSpec(mark string) *spec.Spec {
	return &spec.Spec{
		Mark: &spec.Mark{Shorthand: mark},
		Encoding: &spec.Encoding{
			X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "month", Type: "ordinal"}},
			Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "visits", Type: "quantitative"}},
		},
	}
}

func parentEncoding() *spec.Encoding {
	return &spec.Encoding{
		X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "month", Type: "ordinal"}},
	}
}

func checkParentEncoding(s *spec.Spec) []string {
	var codes []string
	for _, e := range (CompositeParentEncoding{}).Check(s, validate.EmptyLookup{}) {
		codes = append(codes, e.Code)
	}
	return codes
}

func TestPrismCompositeParentEncodingAcceptsFlatSpec(t *testing.T) {
	s := leafSpec("bar")
	s.Schema = "urn:prism:schema:v1:spec"
	if errs := (CompositeParentEncoding{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors on a flat spec, got: %+v", errs)
	}
}

func TestPrismCompositeParentEncodingAcceptsCleanComposites(t *testing.T) {
	cases := map[string]*spec.Spec{
		"layer":   {Layer: []*spec.Spec{leafSpec("bar"), leafSpec("line")}},
		"concat":  {Concat: []*spec.Spec{leafSpec("bar")}},
		"hconcat": {HConcat: []*spec.Spec{leafSpec("bar")}},
		"vconcat": {VConcat: []*spec.Spec{leafSpec("bar")}},
		"facet": {
			Facet: &spec.Facet{Column: &spec.FacetChannel{Field: "region", Type: "nominal"}},
			// The child's own encoding is the chart that is drawn.
			ChildSpec: leafSpec("bar"),
		},
		"repeat": {
			Repeat:    &spec.Repeat{Column: []string{"visits", "signups"}},
			ChildSpec: leafSpec("line"),
		},
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			s.Schema = "urn:prism:schema:v1:spec"
			if errs := (CompositeParentEncoding{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
				t.Fatalf("expected no errors, got: %+v", errs)
			}
		})
	}
}

func TestPrismCompositeParentEncodingRejectsEveryOperator(t *testing.T) {
	cases := map[string]*spec.Spec{
		"layer":   {Encoding: parentEncoding(), Layer: []*spec.Spec{leafSpec("bar")}},
		"concat":  {Encoding: parentEncoding(), Concat: []*spec.Spec{leafSpec("bar")}},
		"hconcat": {Encoding: parentEncoding(), HConcat: []*spec.Spec{leafSpec("bar")}},
		"vconcat": {Encoding: parentEncoding(), VConcat: []*spec.Spec{leafSpec("bar")}},
		"facet": {
			Encoding:  parentEncoding(),
			Facet:     &spec.Facet{Column: &spec.FacetChannel{Field: "region", Type: "nominal"}},
			ChildSpec: leafSpec("bar"),
		},
		"repeat": {
			Encoding:  parentEncoding(),
			Repeat:    &spec.Repeat{Column: []string{"visits"}},
			ChildSpec: leafSpec("line"),
		},
	}
	for name, s := range cases {
		t.Run(name, func(t *testing.T) {
			s.Schema = "urn:prism:schema:v1:spec"
			errs := (CompositeParentEncoding{}).Check(s, validate.EmptyLookup{})
			if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_054" {
				t.Fatalf("expected exactly one PRISM_SPEC_054, got: %+v", errs)
			}
			if got := errs[0].Context["Operator"]; got != name {
				t.Fatalf("Operator = %v, want %q", got, name)
			}
			if got := errs[0].Context["Path"]; got != "the spec root" {
				t.Fatalf("Path = %v, want %q", got, "the spec root")
			}
			wantChild := name
			if name == "facet" || name == "repeat" {
				wantChild = "spec"
			}
			if got := errs[0].Context["Child"]; got != wantChild {
				t.Fatalf("Child = %v, want %q", got, wantChild)
			}
			if len(errs[0].Fixups) == 0 {
				t.Fatal("expected at least one fixup")
			}
		})
	}
}

func TestPrismCompositeParentEncodingWalksNestedComposites(t *testing.T) {
	// A layer nested inside a concat panel: the offending node is two
	// levels down, and the path must name it.
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Concat: []*spec.Spec{
			leafSpec("bar"),
			{
				VConcat: []*spec.Spec{
					{
						Encoding: parentEncoding(),
						Layer:    []*spec.Spec{leafSpec("bar"), leafSpec("line")},
					},
				},
			},
		},
	}
	errs := (CompositeParentEncoding{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_054" {
		t.Fatalf("expected exactly one PRISM_SPEC_054, got: %+v", errs)
	}
	if got := errs[0].Context["Path"]; got != "concat[1].vconcat[0]" {
		t.Fatalf("Path = %v, want %q", got, "concat[1].vconcat[0]")
	}
	if !strings.Contains(errs[0].Message, "concat[1].vconcat[0]") {
		t.Fatalf("message does not name the node: %q", errs[0].Message)
	}
}

func TestPrismCompositeParentEncodingWalksFacetChildComposite(t *testing.T) {
	// The `spec` child of a facet parent is legal on its own, but a
	// composite living there is subject to the same rule.
	s := &spec.Spec{
		Schema: "urn:prism:schema:v1:spec",
		Facet:  &spec.Facet{Row: &spec.FacetChannel{Field: "region", Type: "nominal"}},
		ChildSpec: &spec.Spec{
			Encoding: parentEncoding(),
			Layer:    []*spec.Spec{leafSpec("bar"), leafSpec("line")},
		},
	}
	errs := (CompositeParentEncoding{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_054" {
		t.Fatalf("expected exactly one PRISM_SPEC_054, got: %+v", errs)
	}
	if got := errs[0].Context["Path"]; got != "spec" {
		t.Fatalf("Path = %v, want %q", got, "spec")
	}
}

func TestPrismCompositeParentEncodingFiresPerOffendingNode(t *testing.T) {
	s := &spec.Spec{
		Schema:   "urn:prism:schema:v1:spec",
		Encoding: parentEncoding(),
		HConcat: []*spec.Spec{
			{
				Encoding: parentEncoding(),
				Layer:    []*spec.Spec{leafSpec("bar")},
			},
			leafSpec("line"),
		},
	}
	if got := checkParentEncoding(s); len(got) != 2 {
		t.Fatalf("expected two errors (root + hconcat[0]), got %d: %v", len(got), got)
	}
}

func TestPrismCompositeParentEncodingRegistered(t *testing.T) {
	var found bool
	for _, r := range validate.NewDefaultSemanticValidator().Rules() {
		if r.Code() == "PRISM_SPEC_054" {
			found = true
		}
	}
	if !found {
		t.Fatal("PRISM_SPEC_054 is not in the default rule set")
	}
}

func TestPrismCompositeParentEncodingRejectsAtDecodeLevel(t *testing.T) {
	// End-to-end through the real decoder, so the rule is proven against
	// a spec as an author would write it.
	src := `{
      "$schema": "urn:prism:schema:v1:spec",
      "encoding": {"x": {"field": "month", "type": "ordinal"}},
      "layer": [
        {"mark": "bar", "encoding": {"y": {"field": "visits", "type": "quantitative"}}},
        {"mark": "line", "encoding": {"y": {"field": "signups", "type": "quantitative"}}}
      ]
    }`
	s, err := spec.DecodeBytes([]byte(src))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := checkParentEncoding(s); len(got) != 1 || got[0] != "PRISM_SPEC_054" {
		t.Fatalf("expected one PRISM_SPEC_054, got: %v", got)
	}
}
