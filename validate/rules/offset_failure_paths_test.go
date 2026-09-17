package rules

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/internal/validatorutil"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// Epic E2's QA gate (E2-S4): every offset failure path is an error,
// and it is the RIGHT error.
//
// The sibling offset_channel_test.go drives each rule's Check method
// directly with a hand-built *spec.Spec. That proves the rule logic.
// It cannot prove the thing an author actually experiences, because a
// spec never reaches a semantic rule unless it decodes strictly AND
// clears the JSON Schema shape validator first — and a shape
// violation is reported as the generic PRISM_SPEC_009, which would
// mask the specific code entirely.
//
// These cases therefore run the SAME three stages `prism validate`
// runs, from a JSON document an author could paste, and assert that
// the offset code is what comes back.

// offsetValidate mirrors cmd/prism/cmd_validate.go: strict decode,
// then shape, then semantic. It returns the shape codes and the
// semantic codes separately so a test can tell "rejected for the
// right reason" from "rejected before the rule ever ran".
func offsetValidate(t *testing.T, doc string) (shape []string, semantic []string) {
	t.Helper()

	typed, err := spec.DecodeBytes([]byte(doc))
	if err != nil {
		t.Fatalf("decode: %v\n%s", err, doc)
	}

	var raw any
	if err := json.Unmarshal([]byte(doc), &raw); err != nil {
		t.Fatalf("re-parse for shape validation: %v", err)
	}
	sv, err := validate.NewShapeValidator()
	if err != nil {
		t.Fatalf("init shape validator: %v", err)
	}
	for _, se := range sv.Validate(raw) {
		shape = append(shape, se.InstanceLocation+": "+se.Message)
	}

	sem := validate.NewDefaultSemanticValidator()
	for _, e := range sem.Validate(typed, validatorutil.BuildLookup(typed, afero.NewMemMapFs())) {
		semantic = append(semantic, e.Code)
	}
	sort.Strings(semantic)
	return shape, semantic
}

// requireOffsetCodes asserts the exact set of semantic codes and that
// nothing was rejected at shape level first.
func requireOffsetCodes(t *testing.T, doc string, want ...string) {
	t.Helper()
	shape, semantic := offsetValidate(t, doc)
	if len(shape) != 0 {
		t.Fatalf("spec was rejected by the SHAPE validator before any rule ran, so %v could never be reported:\n%s",
			want, strings.Join(shape, "\n"))
	}
	sort.Strings(want)
	if strings.Join(semantic, ",") != strings.Join(want, ",") {
		t.Fatalf("semantic codes = %v, want %v", semantic, want)
	}
}

// --- the documents -------------------------------------------------
//
// Each is the smallest spec that reaches exactly one failure path.

const (
	// PRISM_SPEC_063 — a mark that cannot dodge.
	offsetDocUnsupportedMark = `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10}]},
	 "mark":{"type":"tick"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"v","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"}}}`

	// PRISM_SPEC_064, first condition — no band on the bound axis.
	offsetDocNoBand = `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":1,"s":"a","v":10}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"quantitative"},
	   "y":{"field":"v","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"}}}`

	// PRISM_SPEC_064, second condition — both offset channels bound.
	offsetDocBothAxes = `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"v","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"},
	   "y_offset":{"field":"s","type":"nominal"}}}`

	// PRISM_SPEC_065 — an explicit stack beside the offset.
	offsetDocExplicitStack = `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"v","type":"quantitative","stack":"zero"},
	   "x_offset":{"field":"s","type":"nominal"}}}`

	// PRISM_SPEC_066 — a span on the offset's own axis.
	offsetDocSameAxisSpan = `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","q2":"Q2","s":"a","v":10}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "x2":{"field":"q2","type":"nominal"},
	   "y":{"field":"v","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"}}}`

	// Clean: the canonical grouped bar.
	offsetDocGroupedBar = `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","v":10},{"q":"Q1","s":"b","v":6},
	   {"q":"Q2","s":"a","v":14},{"q":"Q2","s":"b","v":9}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"aggregate":"sum","field":"v","type":"quantitative"},
	   "color":{"field":"s","type":"nominal"},
	   "x_offset":{"field":"s","type":"nominal"}}}`

	// Clean: ranged AND dodged — the span is on the OTHER axis.
	offsetDocRangedDodged = `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"q":"Q1","s":"a","lo":2,"hi":10},{"q":"Q1","s":"b","lo":1,"hi":6}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"lo","type":"quantitative"},
	   "y2":{"field":"hi","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"}}}`
)

// --- one test per code ----------------------------------------------

func TestPrismOffsetFailurePathUnsupportedMark(t *testing.T) {
	requireOffsetCodes(t, offsetDocUnsupportedMark, "PRISM_SPEC_063")
}

// PRISM_SPEC_064 carries two independent conditions, and each is
// asserted on its own document: an author who trips one must not be
// told about the other.
func TestPrismOffsetFailurePathNoBandOnBoundAxis(t *testing.T) {
	requireOffsetCodes(t, offsetDocNoBand, "PRISM_SPEC_064")
}

func TestPrismOffsetFailurePathBothOffsetChannelsBound(t *testing.T) {
	requireOffsetCodes(t, offsetDocBothAxes, "PRISM_SPEC_064")
}

func TestPrismOffsetFailurePathExplicitStack(t *testing.T) {
	requireOffsetCodes(t, offsetDocExplicitStack, "PRISM_SPEC_065")
}

func TestPrismOffsetFailurePathSameAxisSpan(t *testing.T) {
	requireOffsetCodes(t, offsetDocSameAxisSpan, "PRISM_SPEC_066")
}

// --- the negatives ---------------------------------------------------

func TestPrismOffsetFailurePathsAcceptValidSpecs(t *testing.T) {
	cases := []struct {
		name string
		doc  string
	}{
		{"grouped-bar", offsetDocGroupedBar},
		{"ranged-dodged-bar", offsetDocRangedDodged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireOffsetCodes(t, tc.doc)
		})
	}
}

// --- no rule masks another -------------------------------------------

// A quantitative x with BOTH an x2 and an x_offset trips two rules at
// once: there is no band to subdivide (064) and the span claims the
// axis anyway (066). Both must be reported. Reporting only the first
// would leave an author fixing the type, revalidating, and meeting the
// second error — and reporting only 066 would hide that the axis was
// never banded to begin with.
func TestPrismOffsetFailurePathsDoNotMaskEachOther(t *testing.T) {
	requireOffsetCodes(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"a":1,"b":2,"s":"x","v":10}]},
	 "mark":{"type":"bar"},
	 "encoding":{
	   "x":{"field":"a","type":"quantitative"},
	   "x2":{"field":"b","type":"quantitative"},
	   "y":{"field":"v","type":"quantitative"},
	   "x_offset":{"field":"s","type":"nominal"}}}`,
		"PRISM_SPEC_064", "PRISM_SPEC_066")
}

// An unsupported mark that ALSO stacks explicitly reports both, for
// the same reason.
func TestPrismOffsetFailurePathsReportEveryTrippedRule(t *testing.T) {
	requireOffsetCodes(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10}]},
	 "mark":{"type":"tick"},
	 "encoding":{
	   "x":{"field":"q","type":"nominal"},
	   "y":{"field":"v","type":"quantitative","stack":"zero"},
	   "x_offset":{"field":"s","type":"nominal"}}}`,
		"PRISM_SPEC_063", "PRISM_SPEC_065")
}

// --- composition ------------------------------------------------------

// An invalid binding inside a layer child is caught: the rules walk
// children, and a parent-only check would let the whole class through.
// The sibling layer is valid, which also pins that a legal child is
// not collateral damage.
func TestPrismOffsetFailurePathInsideLayerChild(t *testing.T) {
	requireOffsetCodes(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10}]},
	 "layer":[
	  {"$schema":"urn:prism:schema:v1:spec","mark":{"type":"bar"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}},
	  {"$schema":"urn:prism:schema:v1:spec","mark":{"type":"tick"},
	   "encoding":{
	     "x":{"field":"q","type":"nominal"},
	     "y":{"field":"v","type":"quantitative"},
	     "x_offset":{"field":"s","type":"nominal"}}}]}`,
		"PRISM_SPEC_063")
}

// Two levels deep, through a concat cell into a layer.
func TestPrismOffsetFailurePathInsideNestedComposition(t *testing.T) {
	requireOffsetCodes(t, `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[{"q":"Q1","s":"a","v":10}]},
	 "concat":[
	  {"$schema":"urn:prism:schema:v1:spec","layer":[
	   {"$schema":"urn:prism:schema:v1:spec","mark":{"type":"bar"},
	    "encoding":{
	      "x":{"field":"q","type":"nominal"},
	      "y":{"field":"v","type":"quantitative"},
	      "x_offset":{"field":"s","type":"nominal"},
	      "y_offset":{"field":"s","type":"nominal"}}}]}]}`,
		"PRISM_SPEC_064")
}
