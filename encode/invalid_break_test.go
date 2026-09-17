package encode_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// encodeInvalidJSON drives a flat spec through Build → Execute →
// Encode and returns the scene as JSON for substring assertions.
func encodeInvalidJSON(t *testing.T, body string) string {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	dag, tipID, err := build.Build(s, build.Options{
		FS:       afero.NewMemMapFs(),
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
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func invalidFixture(mode string) string {
	return `{"$schema":"urn:prism:schema:v1:spec",
	 "data":{"values":[
	   {"p":"Q1","v":10},{"p":"Q2","v":14},{"p":"Q3","v":null},{"p":"Q4","v":19}]},
	 "mark":{"type":"line"` + mode + `},
	 "encoding":{"x":{"field":"p","type":"nominal"},"y":{"field":"v","type":"quantitative"}}}`
}

// TestPrismInvalidBreakKeepsTheCategory is the behaviour the feature
// exists for. Under the default the unmeasured period leaves the scale
// domain with its row, so the surviving points sit EVENLY SPACED and
// nothing in the drawing says a period is missing. "break" keeps its
// slot.
func TestPrismInvalidBreakKeepsTheCategory(t *testing.T) {
	if got := encodeInvalidJSON(t, invalidFixture("")); strings.Contains(got, `"Q3"`) {
		t.Error(`default mode kept "Q3" in the scene; it should leave with its row`)
	}
	if got := encodeInvalidJSON(t, invalidFixture(`,"invalid":"break"`)); !strings.Contains(got, `"Q3"`) {
		t.Error(`"break" dropped "Q3"; the category must hold its slot on the axis`)
	}
}

// TestPrismInvalidBreakSplitsThePath asserts the path actually breaks
// rather than closing over the hole — the difference between showing a
// gap and asserting continuity across it.
func TestPrismInvalidBreakSplitsThePath(t *testing.T) {
	filtered := encodeInvalidJSON(t, invalidFixture(""))
	if got := strings.Count(filtered, `"line-0"`); got != 1 {
		t.Errorf("default mode emitted %d line-0 marks, want 1 unbroken path", got)
	}
	broken := encodeInvalidJSON(t, invalidFixture(`,"invalid":"break"`))
	if !strings.Contains(broken, `"line-0-0"`) {
		t.Error(`"break" did not split the path (no line-0-0 segment)`)
	}
}

// TestPrismInvalidStrandedPointStillDraws guards a regression this
// feature could easily have introduced: Q4 is alone between a gap and
// the end of the series, and a one-point polyline renders nothing. If
// "break" emitted one, choosing it would HIDE a row the author asked
// to keep — worse than the "filter" it was chosen over, which at least
// drew that row as part of the line.
func TestPrismInvalidStrandedPointStillDraws(t *testing.T) {
	broken := encodeInvalidJSON(t, invalidFixture(`,"invalid":"break"`))
	if !strings.Contains(broken, `"point"`) {
		t.Error("a measurement stranded between two gaps drew nothing; it must fall back to a dot")
	}
}

// TestPrismInvalidDefaultMatchesExplicitFilter pins the promise that
// makes this safe to ship: an absent key and an explicit "filter"
// encode identically, so no existing chart moves.
func TestPrismInvalidDefaultMatchesExplicitFilter(t *testing.T) {
	if encodeInvalidJSON(t, invalidFixture("")) != encodeInvalidJSON(t, invalidFixture(`,"invalid":"filter"`)) {
		t.Error(`absent "invalid" and explicit "filter" differ; the default must be filter`)
	}
}

// TestPrismInvalidIsInertOnCleanData asserts the mode changes nothing
// when no row is null, or "break" would be a silent restyle rather
// than a null policy.
func TestPrismInvalidIsInertOnCleanData(t *testing.T) {
	clean := func(mode string) string {
		return `{"$schema":"urn:prism:schema:v1:spec",
		 "data":{"values":[{"p":"Q1","v":10},{"p":"Q2","v":14}]},
		 "mark":{"type":"line"` + mode + `},
		 "encoding":{"x":{"field":"p","type":"nominal"},"y":{"field":"v","type":"quantitative"}}}`
	}
	if encodeInvalidJSON(t, clean("")) != encodeInvalidJSON(t, clean(`,"invalid":"break"`)) {
		t.Error(`"break" changed a scene containing no nulls`)
	}
}
