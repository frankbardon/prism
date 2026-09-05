package encode

import (
	"testing"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// newRegionTable builds a tiny 2-row table with a string "region"
// column, mirroring newGeoTable's pattern (geoshape_e2e_test.go), for
// unit-testing applyCategoryStyles in isolation from the full
// spec->plan->encode pipeline.
func newRegionTable(t *testing.T) *table.Table {
	t.Helper()
	rows := []string{"west", "east"}
	sch := &table.Schema{Fields: []table.Field{{Name: "region", Type: table.FieldTypeCategoricalU8}}}
	tbl, err := table.NewTable(sch, map[string]table.Column{"region": table.StringColumn(rows)}, len(rows), "test-hash")
	if err != nil {
		t.Fatalf("table build: %v", err)
	}
	return tbl
}

func markForRow(row int64) scene.Mark {
	return scene.Mark{Datum: &scene.Datum{RowID: row}}
}

// TestCategoryStyleFieldsAt_EnumeratesBoundFields covers the field
// enumeration helper: every channel with a non-empty Field
// contributes exactly once, field-less channels are skipped, and a
// field bound on two channels only appears once.
func TestCategoryStyleFieldsAt_EnumeratesBoundFields(t *testing.T) {
	enc := &spec.Encoding{
		X:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "region"}},
		Y:     &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "score"}},
		Color: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{Field: "region"}}, // duplicate of X's field
		Shape: &spec.MarkChannel{ChannelCommon: spec.ChannelCommon{}},                // field-less: value-only channel
	}
	got := categoryStyleFieldsAt(enc)
	want := []string{"region", "score"}
	if len(got) != len(want) {
		t.Fatalf("categoryStyleFieldsAt = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("categoryStyleFieldsAt[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestCategoryStyleFieldsAt_Nil covers the nil-encoding guard.
func TestCategoryStyleFieldsAt_Nil(t *testing.T) {
	if got := categoryStyleFieldsAt(nil); got != nil {
		t.Fatalf("categoryStyleFieldsAt(nil) = %v, want nil", got)
	}
}

// TestApplyCategoryStyleToMark_Match resolves a matching field value
// and asserts the mark's style picks up the matched MarkStyle's fill.
func TestApplyCategoryStyleToMark_Match(t *testing.T) {
	tbl := newRegionTable(t)
	m := markForRow(1) // row 1 == "east"
	values := map[string]*theme.MarkStyle{"east": {Fill: "#22c55e"}}

	applyCategoryStyleToMark(&m, tbl, "region", values, &theme.Theme{})

	if m.Style.Fill == nil || m.Style.Fill.Hex() != "#22c55e" {
		t.Errorf("Style.Fill = %v, want #22c55e", m.Style.Fill)
	}
}

// TestApplyCategoryStyleToMark_NoMatch leaves the mark's style
// completely untouched when the row's field value has no entry.
func TestApplyCategoryStyleToMark_NoMatch(t *testing.T) {
	tbl := newRegionTable(t)
	existingFill, _ := scene.ColorFromHex("#3b82f6")
	m := markForRow(0) // row 0 == "west"
	m.Style.Fill = existingFill
	values := map[string]*theme.MarkStyle{"east": {Fill: "#22c55e"}}

	applyCategoryStyleToMark(&m, tbl, "region", values, &theme.Theme{})

	if m.Style.Fill != existingFill {
		t.Errorf("Style.Fill = %v, want unchanged %v (no matching entry)", m.Style.Fill, existingFill)
	}
}

// TestApplyCategoryStyleToMark_OnlySetsMatchedFields proves the merge
// only moves fields the CategoryStyle entry actually sets: a stroke
// -only entry must not disturb an already-resolved fill.
func TestApplyCategoryStyleToMark_OnlySetsMatchedFields(t *testing.T) {
	tbl := newRegionTable(t)
	existingFill, _ := scene.ColorFromHex("#3b82f6")
	m := markForRow(1) // row 1 == "east"
	m.Style.Fill = existingFill
	values := map[string]*theme.MarkStyle{"east": {Stroke: "#000000"}}

	applyCategoryStyleToMark(&m, tbl, "region", values, &theme.Theme{})

	if m.Style.Fill != existingFill {
		t.Errorf("Style.Fill = %v, want unchanged %v (entry only set stroke)", m.Style.Fill, existingFill)
	}
	if m.Style.Stroke == nil || m.Style.Stroke.Hex() != "#000000" {
		t.Errorf("Style.Stroke = %v, want #000000", m.Style.Stroke)
	}
}

// TestApplyCategoryStyles_NoOpGuards covers the early-return guards:
// nil encoding, nil theme, empty CategoryStyles, and an empty mark
// list must all be safe no-ops.
func TestApplyCategoryStyles_NoOpGuards(t *testing.T) {
	tbl := newRegionTable(t)
	enc := &spec.Encoding{X: &spec.PositionChannel{ChannelCommon: spec.ChannelCommon{Field: "region"}}}
	th := &theme.Theme{CategoryStyles: map[string]map[string]*theme.MarkStyle{
		"region": {"east": {Fill: "#22c55e"}},
	}}
	m := markForRow(1)
	markList := []scene.Mark{m}

	// nil encoding
	applyCategoryStyles(nil, tbl, th, markList)
	if markList[0].Style.Fill != nil {
		t.Errorf("nil enc: Style.Fill = %v, want nil", markList[0].Style.Fill)
	}
	// nil theme
	applyCategoryStyles(enc, tbl, nil, markList)
	if markList[0].Style.Fill != nil {
		t.Errorf("nil theme: Style.Fill = %v, want nil", markList[0].Style.Fill)
	}
	// empty CategoryStyles
	applyCategoryStyles(enc, tbl, &theme.Theme{}, markList)
	if markList[0].Style.Fill != nil {
		t.Errorf("empty CategoryStyles: Style.Fill = %v, want nil", markList[0].Style.Fill)
	}
	// empty mark list must not panic
	applyCategoryStyles(enc, tbl, th, nil)

	// Sanity: the same theme/enc/table DOES apply when marks are
	// actually present, proving the guards above short-circuited for
	// the stated reason and not some unrelated bug.
	applyCategoryStyles(enc, tbl, th, markList)
	if markList[0].Style.Fill == nil || markList[0].Style.Fill.Hex() != "#22c55e" {
		t.Errorf("Style.Fill = %v, want #22c55e once guards are satisfied", markList[0].Style.Fill)
	}
}

// TestStringifyCategoryValue covers the string / non-string branches.
func TestStringifyCategoryValue(t *testing.T) {
	if got := stringifyCategoryValue("east"); got != "east" {
		t.Errorf("stringifyCategoryValue(%q) = %q, want %q", "east", got, "east")
	}
	if got := stringifyCategoryValue(90.0); got != "90" {
		t.Errorf("stringifyCategoryValue(90.0) = %q, want %q", got, "90")
	}
}
