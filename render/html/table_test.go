package html_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	"github.com/frankbardon/prism/encode"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/render"
	prismhtml "github.com/frankbardon/prism/render/html"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// tableBasicSpecJSON is a table-mark spec with two plain text/number
// columns and no sub-mark column — the simplest possible table (E1-S4
// acceptance criterion 1: header + body rows, no sub-marks).
const tableBasicSpecJSON = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "values": [
      {"name": "Acme", "revenue": 120},
      {"name": "Globex", "revenue": 80}
    ]
  },
  "mark": {"type": "table"},
  "encoding": {
    "columns": [
      {"field": "name", "type": "nominal", "title": "Account"},
      {"field": "revenue", "type": "quantitative"}
    ]
  }
}`

// tableSparklineSpecJSON is a table-mark spec whose "trend" column
// binds a "sparkline" sub-mark — E1-S4 acceptance criterion 2: the
// <td> for that column must contain a well-formed inline
// <svg>...</svg> produced by re-invoking render/svg's real emitters,
// not hand-rolled markup.
const tableSparklineSpecJSON = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "values": [
      {"name": "Acme", "revenue": 120, "trend": [10, 12, 9, 14, 20]},
      {"name": "Globex", "revenue": 80, "trend": [5, 6, 4, 7, 9]}
    ]
  },
  "mark": {"type": "table"},
  "encoding": {
    "columns": [
      {"field": "name", "type": "nominal", "title": "Account"},
      {"field": "revenue", "type": "quantitative"},
      {"field": "trend", "type": "quantitative", "mark": "sparkline", "title": "Trend"}
    ]
  }
}`

// tablePaginatedSpecJSON declares a small page_size (2) against five
// rows — E1-S5 acceptance criterion 3: a table with more rows than
// page_size must render pagination controls in the markup (the
// client, static/vendor/prism/prism-table.mjs, does the actual
// slicing; Go's job is only to decide whether the controls are
// needed and to render every row so the client has the full set to
// slice from).
const tablePaginatedSpecJSON = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "values": [
      {"name": "Acme", "revenue": 120},
      {"name": "Globex", "revenue": 80},
      {"name": "Initech", "revenue": 60},
      {"name": "Umbrella", "revenue": 200},
      {"name": "Soylent", "revenue": 40}
    ]
  },
  "mark": {"type": "table", "page_size": 2},
  "encoding": {
    "columns": [
      {"field": "name", "type": "nominal", "title": "Account"},
      {"field": "revenue", "type": "quantitative"}
    ]
  }
}`

// TestPrismHTMLTablePagination asserts pagination controls render
// only when row count exceeds page_size, that every row still lands
// in the markup (client-side slicing needs the full set, not a
// server-paginated subset), and that the page-size + indicator values
// reflect the spec's declared page_size.
func TestPrismHTMLTablePagination(t *testing.T) {
	got, err := renderTableSpec(t, tablePaginatedSpecJSON)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(got)

	if !strings.Contains(s, `data-prism-page-size="2"`) {
		t.Errorf("expected data-prism-page-size=\"2\" on <table>:\n%s", truncate(got, 1200))
	}
	if !strings.Contains(s, "data-prism-table-pagination") {
		t.Errorf("expected pagination controls for 5 rows / page_size 2:\n%s", truncate(got, 1200))
	}
	if !strings.Contains(s, "Page 1 of 3") {
		t.Errorf("expected page indicator \"Page 1 of 3\" (ceil(5/2)):\n%s", truncate(got, 1200))
	}
	for _, name := range []string{"Acme", "Globex", "Initech", "Umbrella", "Soylent"} {
		if !strings.Contains(s, name) {
			t.Errorf("expected all 5 rows pre-rendered (client slices, server does not paginate); missing %q:\n%s", name, truncate(got, 1200))
		}
	}
	if !strings.Contains(s, "data-prism-page-prev") || !strings.Contains(s, "data-prism-page-next") {
		t.Errorf("expected prev/next pagination buttons:\n%s", truncate(got, 1200))
	}
}

// TestPrismHTMLTableNoPaginationWhenUnderPageSize asserts the basic
// (2-row, default page_size 25) fixture renders no pagination
// controls — the negative case for acceptance criterion 3.
func TestPrismHTMLTableNoPaginationWhenUnderPageSize(t *testing.T) {
	got, err := renderTableSpec(t, tableBasicSpecJSON)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(string(got), "data-prism-table-pagination") {
		t.Errorf("did not expect pagination controls for 2 rows / default page_size 25:\n%s", truncate(got, 1200))
	}
}

// TestPrismHTMLTableGoldensStable golden-tests the <table> rendering
// path for both a plain table and a table with an embedded sparkline
// column. Mirrors TestPrismHTMLGoldensStable's shape; set
// UPDATE_GOLDENS=1 to regenerate.
func TestPrismHTMLTableGoldensStable(t *testing.T) {
	fixtures := map[string]string{
		"table_basic.html":     tableBasicSpecJSON,
		"table_sparkline.html": tableSparklineSpecJSON,
	}
	update := os.Getenv("UPDATE_GOLDENS") == "1"
	for goldenName, body := range fixtures {
		goldenName, body := goldenName, body
		t.Run(goldenName, func(t *testing.T) {
			got, err := renderTableSpec(t, body)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			goldenPath := filepath.Join(repoRoot(t), "render", "html", "testdata", "htmls", goldenName)
			if update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden %s: %v", goldenPath, err)
				}
				t.Logf("wrote golden %s (%d bytes)", goldenPath, len(got))
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (%s): %v.\nRun `UPDATE_GOLDENS=1 go test ./render/html/...` to create.", goldenPath, err)
			}
			if !bytes.Equal(want, got) {
				t.Errorf("HTML does not match golden %s.\n--- golden ---\n%s\n--- got ---\n%s",
					goldenPath, truncate(want, 1200), truncate(got, 1200))
			}
		})
	}
}

// TestPrismHTMLTableStructure asserts the basic-table output is a
// well-formed HTML shell containing a semantic <table> with a header
// row built from column titles and one body row per data row — no
// sub-mark, no embedded <svg>.
func TestPrismHTMLTableStructure(t *testing.T) {
	got, err := renderTableSpec(t, tableBasicSpecJSON)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(got)
	for _, want := range []string{
		"<!doctype html>", "<html>", "</html>",
		`<table class="prism-html-table"`,
		"<thead>",
		`<th data-prism-field="name">Account</th>`,
		`<th data-prism-field="revenue">revenue</th>`,
		"<tbody>", "Acme", "Globex", "120", "80",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, truncate(got, 1200))
		}
	}
	if strings.Contains(s, "<svg") {
		t.Errorf("no-sub-mark table output should not contain an <svg>:\n%s", truncate(got, 1200))
	}
}

// TestPrismHTMLTableSparklineCell asserts the sparkline column's
// <td> carries a well-formed inline <svg>...</svg> fragment (E1-S4
// acceptance criteria 2-4): the fragment parses as valid XML, and its
// numeric attributes are pinned to render.RenderPrecision (3
// decimals) — proving it came through render/svg's real emitters
// (render.FormatFloat), not hand-rolled coordinates.
func TestPrismHTMLTableSparklineCell(t *testing.T) {
	got, err := renderTableSpec(t, tableSparklineSpecJSON)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(got)

	frags := extractSVGFragments(s)
	if len(frags) == 0 {
		t.Fatalf("expected at least one inline <svg> fragment in sparkline column cells:\n%s", truncate(got, 1200))
	}
	// Two data rows, one sparkline column each => 2 fragments.
	if len(frags) != 2 {
		t.Errorf("got %d inline <svg> fragments, want 2:\n%s", len(frags), truncate(got, 1200))
	}

	decimals := regexp.MustCompile(`\d+\.(\d+)`)
	for i, frag := range frags {
		if err := assertWellFormedXML(frag); err != nil {
			t.Errorf("fragment %d is not well-formed XML: %v\n%s", i, err, frag)
		}
		for _, m := range decimals.FindAllStringSubmatch(frag, -1) {
			if len(m[1]) > 3 {
				t.Errorf("fragment %d has a coordinate with >3 decimal places (%q) — expected render.FormatFloat's 3-decimal quantisation:\n%s", i, m[0], frag)
			}
		}
	}
}

// extractSVGFragments returns every <svg ...>...</svg> top-level
// fragment found in s, in order.
func extractSVGFragments(s string) []string {
	var frags []string
	rest := s
	for {
		start := strings.Index(rest, "<svg")
		if start < 0 {
			break
		}
		end := strings.Index(rest[start:], "</svg>")
		if end < 0 {
			break
		}
		end += start + len("</svg>")
		frags = append(frags, rest[start:end])
		rest = rest[end:]
	}
	return frags
}

// assertWellFormedXML walks every token in frag via encoding/xml,
// failing on the first parse error — the "xml.Unmarshal on the
// fragment" smoke check CLAUDE.md's acceptance criteria calls for.
func assertWellFormedXML(frag string) error {
	dec := xml.NewDecoder(strings.NewReader(frag))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func renderTableSpec(t *testing.T, body string) ([]byte, error) {
	t.Helper()
	s, err := spec.DecodeBytes([]byte(body))
	if err != nil {
		return nil, err
	}
	dag, tipID, err := build.Build(s, build.Options{
		FS:       afero.NewOsFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
	})
	if err != nil {
		return nil, err
	}
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{})
	if err != nil {
		return nil, err
	}
	if len(res.Errors) > 0 {
		t.Fatalf("execute: %d node errors: %v", len(res.Errors), res.Errors)
	}
	doc, err := encode.Encode(s, res.Tables, tipID, encode.EncodeOpts{Width: 800, Height: 600})
	if err != nil {
		return nil, err
	}
	return prismhtml.New().Render(doc, render.RenderOpts{Format: "html", Width: 800, Height: 600})
}
