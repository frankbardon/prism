package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func progressSpec(def *spec.MarkDef, enc *spec.Encoding) *spec.Spec {
	return &spec.Spec{
		Schema:   "urn:prism:schema:v1:spec",
		Mark:     &spec.Mark{Def: def},
		Encoding: enc,
	}
}

func metricRowEncoding() *spec.Encoding {
	return &spec.Encoding{
		X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}},
		Y: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "metric", Type: "nominal"}},
	}
}

func TestProgressStructureAcceptsCanonicalMetricRows(t *testing.T) {
	thickness := 0.45
	s := progressSpec(&spec.MarkDef{
		Type:      "progress",
		Total:     float64(100),
		Thickness: &thickness,
	}, metricRowEncoding())
	if errs := (ProgressStructure{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestProgressStructureAcceptsFieldNameTotal(t *testing.T) {
	s := progressSpec(&spec.MarkDef{Type: "progress", Total: "quota"}, metricRowEncoding())
	if errs := (ProgressStructure{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors for a field-name total, got: %+v", errs)
	}
}

func TestProgressStructureRequiresBothPositionChannels(t *testing.T) {
	enc := &spec.Encoding{X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score", Type: "quantitative"}}}
	s := progressSpec(&spec.MarkDef{Type: "progress"}, enc)
	errs := ProgressStructure{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_061" {
		t.Fatalf("expected exactly one PRISM_SPEC_061, got: %+v", errs)
	}
}

func TestProgressStructureRejectsOutOfRangeThickness(t *testing.T) {
	for _, bad := range []float64{0, -0.2, 1.5} {
		v := bad
		s := progressSpec(&spec.MarkDef{Type: "progress", Thickness: &v}, metricRowEncoding())
		errs := ProgressStructure{}.Check(s, validate.EmptyLookup{})
		if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_061" {
			t.Fatalf("thickness %v: expected one PRISM_SPEC_061, got: %+v", bad, errs)
		}
	}
}

func TestProgressStructureRejectsNonPositiveTotal(t *testing.T) {
	s := progressSpec(&spec.MarkDef{Type: "progress", Total: float64(0)}, metricRowEncoding())
	errs := ProgressStructure{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_061" {
		t.Fatalf("expected one PRISM_SPEC_061, got: %+v", errs)
	}
}

func TestProgressStructureWalksLayerChildren(t *testing.T) {
	child := progressSpec(&spec.MarkDef{Type: "progress", Total: float64(-1)}, metricRowEncoding())
	s := &spec.Spec{Schema: "urn:prism:schema:v1:spec", Layer: []*spec.Spec{child}}
	errs := ProgressStructure{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Context["Path"] != "layer[0]" {
		t.Fatalf("expected one error tagged layer[0], got: %+v", errs)
	}
}

func TestProgressStructureIgnoresOtherMarks(t *testing.T) {
	s := progressSpec(&spec.MarkDef{Type: "bar"}, metricRowEncoding())
	if errs := (ProgressStructure{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors for a non-progress mark, got: %+v", errs)
	}
}

// progress reads the shared mark.orient vocabulary rather than
// bullet's per-mark orientation field, so PRISM_SPEC_046 must accept
// both values on it.
func TestProgressReadsSharedMarkOrient(t *testing.T) {
	for _, o := range []string{"vertical", "horizontal"} {
		s := progressSpec(&spec.MarkDef{Type: "progress", Orient: o}, metricRowEncoding())
		if errs := (MarkOrientSupported{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
			t.Fatalf("orient %q: expected no errors, got: %+v", o, errs)
		}
	}
	s := progressSpec(&spec.MarkDef{Type: "progress", Orient: "radial"}, metricRowEncoding())
	errs := MarkOrientSupported{}.Check(s, validate.EmptyLookup{})
	if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_046" {
		t.Fatalf("expected one PRISM_SPEC_046 for radial, got: %+v", errs)
	}
}
