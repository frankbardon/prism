package rules

import (
	"testing"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// offsetCh builds a bound offset channel.
func offsetCh(field string) *spec.OffsetChannel {
	return &spec.OffsetChannel{Field: field, Type: "nominal"}
}

// groupedBarEncoding is the canonical valid grouped-bar encoding: a
// banded category axis, an aggregated measure, a colour grouping and
// an offset on the category axis. None of the four rules may fire on
// it.
func groupedBarEncoding() *spec.Encoding {
	return &spec.Encoding{
		X:       pos("month", "ordinal"),
		Y:       pos("visits", "quantitative"),
		Color:   &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "channel", Type: "nominal"}},
		XOffset: offsetCh("channel"),
	}
}

func offsetSpec(mark string, enc *spec.Encoding) *spec.Spec {
	return &spec.Spec{
		Schema:   "urn:prism:schema:v1:spec",
		Mark:     &spec.Mark{Shorthand: mark},
		Encoding: enc,
	}
}

// allOffsetRules is the set this story registers, so a "valid spec
// triggers nothing" assertion covers all four at once.
func allOffsetRules() []validate.SemanticRule {
	return []validate.SemanticRule{
		OffsetMarkSupported{},
		OffsetAxisCoherent{},
		OffsetStackExclusive{},
		OffsetSpanExclusive{},
	}
}

func checkAllOffsetRules(s *spec.Spec) []*errors.AppError {
	var out []*errors.AppError
	for _, r := range allOffsetRules() {
		out = append(out, r.Check(s, validate.EmptyLookup{})...)
	}
	return out
}

// requireCodes asserts the exact multiset of codes, order-insensitive
// on count. Assertions are on AppError.Code, never message text.
func requireCodes(t *testing.T, got []*errors.AppError, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d errors %v, got %d: %+v", len(want), want, len(got), got)
	}
	for i, e := range got {
		if e.Code != want[i] {
			t.Fatalf("error %d: expected %s, got %s (%+v)", i, want[i], e.Code, got)
		}
	}
}

// --- the valid spec -------------------------------------------------

func TestPrismOffsetRulesAcceptGroupedBar(t *testing.T) {
	if errs := checkAllOffsetRules(offsetSpec("bar", groupedBarEncoding())); len(errs) != 0 {
		t.Fatalf("expected no errors on a valid grouped bar, got: %+v", errs)
	}
}

func TestPrismOffsetRulesIgnoreSpecWithoutOffset(t *testing.T) {
	// Every pre-E1 spec omits the channels; all four rules stay silent.
	s := offsetSpec("tick", &spec.Encoding{
		X:  pos("month", "ordinal"),
		Y:  pos("visits", "quantitative"),
		X2: pos("visits_end", "quantitative"),
	})
	s.Encoding.Y.Stack = "zero"
	if errs := checkAllOffsetRules(s); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismOffsetRulesIgnoreOffsetWithNoField(t *testing.T) {
	// An offset object binding no field subdivides nothing, exactly as
	// spec.ResolveOffset reads it.
	enc := groupedBarEncoding()
	enc.XOffset = &spec.OffsetChannel{Type: "nominal"}
	if errs := checkAllOffsetRules(offsetSpec("tick", enc)); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

// --- PRISM_SPEC_063: unsupported mark -------------------------------

func TestPrismOffsetMarkSupportedAcceptsBar(t *testing.T) {
	if errs := (OffsetMarkSupported{}).Check(offsetSpec("bar", groupedBarEncoding()), validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismOffsetMarkSupportedRejectsBandSeatedMarks(t *testing.T) {
	// Each of these reaches CategorySlots, so without the rule the
	// offset would silently change nothing.
	for _, mark := range []string{"rect", "tick", "boxplot", "violin", "heatmap", "winloss", "progress", "line", "point", "area"} {
		errs := (OffsetMarkSupported{}).Check(offsetSpec(mark, groupedBarEncoding()), validate.EmptyLookup{})
		if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_063" {
			t.Errorf("mark %q: expected one PRISM_SPEC_063, got: %+v", mark, errs)
		}
	}
}

func TestPrismOffsetMarkSupportedIgnoresUnresolvedMark(t *testing.T) {
	s := &spec.Spec{Schema: "urn:prism:schema:v1:spec", Encoding: groupedBarEncoding()}
	if errs := (OffsetMarkSupported{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors without a mark, got: %+v", errs)
	}
}

func TestPrismOffsetCapableMarksHoldsExactlyBar(t *testing.T) {
	if len(offsetCapableMarks) != 1 {
		t.Fatalf("expected exactly one offset-capable mark, got %v", offsetCapableMarks)
	}
	if !markDrawsOffset("bar", "x_offset") || !markDrawsOffset("bar", "y_offset") {
		t.Fatalf("bar must dodge on both axes, got %v", offsetCapableMarks["bar"])
	}
}

// --- PRISM_SPEC_064: incoherent axis --------------------------------

func TestPrismOffsetAxisCoherentRejectsBothAxesBound(t *testing.T) {
	enc := groupedBarEncoding()
	enc.YOffset = offsetCh("channel")
	errs := (OffsetAxisCoherent{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{})
	requireCodes(t, errs, "PRISM_SPEC_064")
}

func TestPrismOffsetAxisCoherentRejectsContinuousBaseChannel(t *testing.T) {
	for _, ty := range []string{"quantitative", "temporal"} {
		enc := groupedBarEncoding()
		enc.X = pos("month", ty)
		errs := (OffsetAxisCoherent{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{})
		if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_064" {
			t.Errorf("x type %q: expected one PRISM_SPEC_064, got: %+v", ty, errs)
		}
	}
}

func TestPrismOffsetAxisCoherentRejectsUnboundBaseChannel(t *testing.T) {
	enc := groupedBarEncoding()
	enc.X = nil
	errs := (OffsetAxisCoherent{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{})
	requireCodes(t, errs, "PRISM_SPEC_064")
}

func TestPrismOffsetAxisCoherentRejectsNonBandScaleType(t *testing.T) {
	enc := groupedBarEncoding()
	enc.X.Scale = &spec.Scale{Type: "point"}
	errs := (OffsetAxisCoherent{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{})
	requireCodes(t, errs, "PRISM_SPEC_064")
}

func TestPrismOffsetAxisCoherentAcceptsExplicitBandScaleType(t *testing.T) {
	enc := groupedBarEncoding()
	enc.X.Scale = &spec.Scale{Type: "band"}
	if errs := (OffsetAxisCoherent{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismOffsetAxisCoherentStaysSilentOnInferredType(t *testing.T) {
	// No declared type and no scale.type: the family comes from the
	// executed column's kind, which validate never sees. The encoder
	// answers (PRISM_ENCODE_001); duplicating it here would report one
	// spec twice.
	enc := groupedBarEncoding()
	enc.X = &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "month"}}
	if errs := (OffsetAxisCoherent{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismOffsetAxisCoherentAcceptsYOffsetOnBandedY(t *testing.T) {
	enc := &spec.Encoding{
		Y:       pos("month", "ordinal"),
		X:       pos("visits", "quantitative"),
		YOffset: offsetCh("channel"),
	}
	if errs := (OffsetAxisCoherent{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

// --- PRISM_SPEC_065: explicit stack ---------------------------------

func TestPrismOffsetStackExclusiveRejectsExplicitStack(t *testing.T) {
	for _, offset := range []any{true, "zero", "normalize", "center"} {
		enc := groupedBarEncoding()
		enc.Y.Stack = offset
		errs := (OffsetStackExclusive{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{})
		if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_065" {
			t.Errorf("stack %v: expected one PRISM_SPEC_065, got: %+v", offset, errs)
		}
	}
}

func TestPrismOffsetStackExclusiveAcceptsImplicitStackShape(t *testing.T) {
	// The grouped-bar spec is character for character the shape that
	// would stack implicitly; ResolveStack yields, so no `stack` key
	// is written and nothing fires.
	if errs := (OffsetStackExclusive{}).Check(offsetSpec("bar", groupedBarEncoding()), validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("expected no errors, got: %+v", errs)
	}
}

func TestPrismOffsetStackExclusiveAcceptsDisabledStack(t *testing.T) {
	// `"stack": false` and `"stack": null` DISABLE stacking, so they
	// agree with the offset rather than contradicting it.
	enc := groupedBarEncoding()
	enc.Y.Stack = false
	if errs := (OffsetStackExclusive{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("stack false: expected no errors, got: %+v", errs)
	}

	enc = groupedBarEncoding()
	enc.Y.StackNull = true
	if errs := (OffsetStackExclusive{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("stack null: expected no errors, got: %+v", errs)
	}
}

func TestPrismOffsetStackExclusiveDecodesTriState(t *testing.T) {
	// Drive the real decoder so the tri-state is read the way an
	// author writes it, not the way a struct literal does.
	cases := []struct {
		name  string
		stack string
		want  int
	}{
		{"absent", "", 0},
		{"null", `, "stack": null`, 0},
		{"false", `, "stack": false`, 0},
		{"zero", `, "stack": "zero"`, 1},
	}
	for _, tc := range cases {
		doc := `{"$schema":"urn:prism:schema:v1:spec","mark":"bar","encoding":{` +
			`"x":{"field":"month","type":"ordinal"},` +
			`"y":{"field":"visits","type":"quantitative"` + tc.stack + `},` +
			`"x_offset":{"field":"channel","type":"nominal"}}}`
		s, err := spec.DecodeBytes([]byte(doc))
		if err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
		errs := (OffsetStackExclusive{}).Check(s, validate.EmptyLookup{})
		if len(errs) != tc.want {
			t.Errorf("%s: expected %d errors, got: %+v", tc.name, tc.want, errs)
		}
		for _, e := range errs {
			if e.Code != "PRISM_SPEC_065" {
				t.Errorf("%s: expected PRISM_SPEC_065, got %s", tc.name, e.Code)
			}
		}
	}
}

// --- PRISM_SPEC_066: same-axis span ---------------------------------

func TestPrismOffsetSpanExclusiveRejectsSameAxisSpan(t *testing.T) {
	enc := groupedBarEncoding()
	enc.X2 = pos("month_end", "ordinal")
	errs := (OffsetSpanExclusive{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{})
	requireCodes(t, errs, "PRISM_SPEC_066")
}

// TestPrismOffsetSpanExclusiveAcceptsOppositeAxisSpan pins the scope:
// a ranged AND dodged bar is well defined and must stay legal.
func TestPrismOffsetSpanExclusiveAcceptsOppositeAxisSpan(t *testing.T) {
	enc := groupedBarEncoding()
	enc.Y2 = pos("visits_end", "quantitative")
	if errs := (OffsetSpanExclusive{}).Check(offsetSpec("bar", enc), validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("y2 + x_offset must validate clean, got: %+v", errs)
	}

	mirrored := &spec.Encoding{
		Y:       pos("task", "ordinal"),
		X:       pos("start", "quantitative"),
		X2:      pos("end", "quantitative"),
		YOffset: offsetCh("team"),
	}
	if errs := (OffsetSpanExclusive{}).Check(offsetSpec("bar", mirrored), validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("x2 + y_offset must validate clean, got: %+v", errs)
	}
}

// --- composition walk ------------------------------------------------

// TestPrismOffsetRulesWalkCompositionChildren asserts every operator is
// reached, including one nested two levels deep — a leaf's binding is
// invisible to a parent-only check.
func TestPrismOffsetRulesWalkCompositionChildren(t *testing.T) {
	bad := func() *spec.Spec { return offsetSpec("tick", groupedBarEncoding()) }

	cases := []struct {
		name string
		s    *spec.Spec
	}{
		{"layer", &spec.Spec{Layer: []*spec.Spec{offsetSpec("bar", groupedBarEncoding()), bad()}}},
		{"concat", &spec.Spec{Concat: []*spec.Spec{bad()}}},
		{"hconcat", &spec.Spec{HConcat: []*spec.Spec{bad()}}},
		{"vconcat", &spec.Spec{VConcat: []*spec.Spec{bad()}}},
		{"facet-child", &spec.Spec{ChildSpec: bad()}},
		{"nested", &spec.Spec{Concat: []*spec.Spec{{Layer: []*spec.Spec{bad()}}}}},
	}
	for _, tc := range cases {
		errs := (OffsetMarkSupported{}).Check(tc.s, validate.EmptyLookup{})
		if len(errs) != 1 || errs[0].Code != "PRISM_SPEC_063" {
			t.Errorf("%s: expected one PRISM_SPEC_063, got: %+v", tc.name, errs)
		}
	}
}

func TestPrismOffsetRulesReportChildPath(t *testing.T) {
	s := &spec.Spec{Layer: []*spec.Spec{
		offsetSpec("bar", groupedBarEncoding()),
		offsetSpec("tick", groupedBarEncoding()),
	}}
	errs := (OffsetMarkSupported{}).Check(s, validate.EmptyLookup{})
	requireCodes(t, errs, "PRISM_SPEC_063")
	if got := errs[0].Context["Path"]; got != "layer[1]" {
		t.Fatalf("expected path layer[1], got %v", got)
	}
}

// --- registration ----------------------------------------------------

func TestPrismOffsetRulesRegistered(t *testing.T) {
	want := map[string]bool{
		"PRISM_SPEC_063": false,
		"PRISM_SPEC_064": false,
		"PRISM_SPEC_065": false,
		"PRISM_SPEC_066": false,
	}
	for _, r := range validate.NewDefaultSemanticValidator().Rules() {
		if _, ok := want[r.Code()]; ok {
			want[r.Code()] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Errorf("%s is not registered on the default semantic validator", code)
		}
	}
}
