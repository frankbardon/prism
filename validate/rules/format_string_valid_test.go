package rules

import (
	"testing"

	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

func tableColumnFormatSpec(formats ...string) *spec.Spec {
	cols := make([]spec.TableColumn, 0, len(formats))
	for i, f := range formats {
		c := spec.TableColumn{}
		c.Field = "f" + string(rune('0'+i))
		c.Type = "quantitative"
		c.Format = f
		cols = append(cols, c)
	}
	return &spec.Spec{
		Schema:   "urn:prism:schema:v1:spec",
		Mark:     &spec.Mark{Shorthand: "table"},
		Encoding: &spec.Encoding{Columns: cols},
	}
}

// TestFormatStringValidWalksTableColumns pins the E7-S4 coverage gap:
// the rule did not walk encoding.columns[] at all, which is how the
// unsupported `$,.0f` (the subset has no currency prefix) reached two
// committed fixtures unchallenged.
func TestFormatStringValidWalksTableColumns(t *testing.T) {
	errs := (FormatStringValid{}).Check(tableColumnFormatSpec("$,.0f"), validate.EmptyLookup{})
	if len(errs) != 1 {
		t.Fatalf("want one error for an unparseable column format, got %+v", errs)
	}
	if errs[0].Code != "PRISM_SPEC_011" {
		t.Errorf("code = %q, want PRISM_SPEC_011", errs[0].Code)
	}
	if where, _ := errs[0].Context["Where"].(string); where != "columns[0].format" {
		t.Errorf("Where = %q, want columns[0].format", where)
	}
}

// TestFormatStringValidAcceptsSupportedColumnFormats keeps the rule
// from over-rejecting: every specifier the encode/format subset
// parses must survive, including an empty (absent) one.
func TestFormatStringValidAcceptsSupportedColumnFormats(t *testing.T) {
	s := tableColumnFormatSpec("", ",.0f", ".1%", ".2s", "%Y-%m")
	if errs := (FormatStringValid{}).Check(s, validate.EmptyLookup{}); len(errs) != 0 {
		t.Fatalf("supported column formats rejected: %+v", errs)
	}
}

// TestFormatStringValidReportsEveryBadColumn asserts the walk does not
// stop at the first offender — an author fixing a multi-column table
// should see all of them at once.
func TestFormatStringValidReportsEveryBadColumn(t *testing.T) {
	errs := (FormatStringValid{}).Check(tableColumnFormatSpec(",.0f", "$,.0f", "£d"), validate.EmptyLookup{})
	if len(errs) != 2 {
		t.Fatalf("want two errors, got %+v", errs)
	}
	if where, _ := errs[0].Context["Where"].(string); where != "columns[1].format" {
		t.Errorf("first Where = %q, want columns[1].format", where)
	}
	if where, _ := errs[1].Context["Where"].(string); where != "columns[2].format" {
		t.Errorf("second Where = %q, want columns[2].format", where)
	}
}
