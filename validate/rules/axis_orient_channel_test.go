package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// orientChannel builds a position channel carrying an axis block with
// the given orient. An empty orient leaves the axis block configured
// but silent about the side.
func orientChannel(field, ty, orient string) *spec.PositionChannel {
	ch := pos(field, ty)
	ch.Axis = &spec.Axis{Orient: orient}
	return ch
}

func orientSpec(enc *spec.Encoding) *spec.Spec {
	return &spec.Spec{
		Schema:   "urn:prism:schema:v1:spec",
		Mark:     &spec.Mark{Shorthand: "bar"},
		Encoding: enc,
	}
}

func TestAxisOrientChannelAcceptsPerChannelSides(t *testing.T) {
	cases := []struct{ x, y string }{
		{"top", "left"},
		{"bottom", "right"},
		{"", ""},
	}
	for _, c := range cases {
		s := orientSpec(&spec.Encoding{
			X: orientChannel("cat", "nominal", c.x),
			Y: orientChannel("val", "quantitative", c.y),
		})
		if errs := (AxisOrientChannel{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
			t.Errorf("x=%q y=%q: expected no errors, got: %+v", c.x, c.y, errs)
		}
	}
}

func TestAxisOrientChannelRejectsCrossAxisOrient(t *testing.T) {
	s := orientSpec(&spec.Encoding{
		X: orientChannel("cat", "nominal", "left"),
		Y: orientChannel("val", "quantitative", "quantitative"),
	})
	errs := (AxisOrientChannel{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 2 {
		t.Fatalf("expected two errors, got: %+v", errs)
	}
	for _, e := range errs {
		if e.Code != "PRISM_SPEC_044" {
			t.Errorf("code = %s, want PRISM_SPEC_044", e.Code)
		}
	}
	if got := errs[0].Context["Allowed"]; got != "bottom, top" {
		t.Errorf("x Allowed = %v, want \"bottom, top\"", got)
	}
	if got := errs[1].Context["Allowed"]; got != "left, right" {
		t.Errorf("y Allowed = %v, want \"left, right\"", got)
	}
}

func TestAxisOrientChannelRejectsUnknownOrient(t *testing.T) {
	s := orientSpec(&spec.Encoding{X: orientChannel("cat", "nominal", "topmost")})
	errs := (AxisOrientChannel{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_044" {
		t.Fatalf("expected one PRISM_SPEC_044, got: %+v", errs)
	}
	if got := errs[0].Context["Orient"]; got != "topmost" {
		t.Errorf("Orient detail = %v, want \"topmost\"", got)
	}
}

func TestAxisOrientChannelRejectsSpanChannelOrient(t *testing.T) {
	s := orientSpec(&spec.Encoding{
		X:  pos("start", "quantitative"),
		X2: orientChannel("end", "quantitative", "right"),
	})
	errs := (AxisOrientChannel{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Context["Channel"] != "x2" {
		t.Fatalf("expected one error naming x2, got: %+v", errs)
	}
}

func TestAxisOrientChannelWalksLayerChildren(t *testing.T) {
	child := orientSpec(&spec.Encoding{X: orientChannel("cat", "nominal", "right")})
	s := &spec.Spec{Schema: "urn:prism:schema:v1:spec", Layer: []*spec.Spec{child}}
	errs := (AxisOrientChannel{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 {
		t.Fatalf("expected one error from the layer child, got: %+v", errs)
	}
	if got := errs[0].Context["Path"]; got != "layer[0]" {
		t.Errorf("Path = %v, want layer[0]", got)
	}
}

func TestAxisOrientChannelWalksFacetChild(t *testing.T) {
	child := orientSpec(&spec.Encoding{Y: orientChannel("val", "quantitative", "top")})
	s := &spec.Spec{Schema: "urn:prism:schema:v1:spec", ChildSpec: child}
	errs := (AxisOrientChannel{}).Check(s, validate.EmptyLookup{})
	if len(errs) != 1 {
		t.Fatalf("expected one error from the facet child, got: %+v", errs)
	}
	if got := errs[0].Context["Path"]; got != "spec" {
		t.Errorf("Path = %v, want spec", got)
	}
}
