package errors

import (
	"bytes"
	"regexp"
	"sort"
	"text/template"

	"testing"
)

// referenceRender reproduces the pre-E6-S2 rendering path: text/template
// with Option("missingkey=zero"). It is the oracle the hand-rolled
// interpolator in codes.go must match byte-for-byte on the host. It is
// intentionally confined to the test binary — the shipping code no
// longer depends on text/template (it crashes TinyGo via
// reflect.Value.MethodByName).
func referenceRender(body string, ctx map[string]any) string {
	tpl, err := template.New("ref").Option("missingkey=zero").Parse(body)
	if err != nil {
		return body
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ctx); err != nil {
		return body
	}
	return buf.String()
}

var placeholderRE = regexp.MustCompile(`\{\{-?\s*\.([A-Za-z_][A-Za-z0-9_]*)`)

// placeholders extracts every distinct `.Ident` field reference used in a
// template body (covers both plain {{.X}} and {{if .X}} guards).
func placeholders(body string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range placeholderRE.FindAllStringSubmatch(body, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
}

// TestRenderTemplateMatchesTextTemplate asserts the non-reflective
// interpolator produces output byte-identical to the previous
// text/template + missingkey=zero renderer for every message and fixup
// in the catalog, under three context regimes:
//
//   - full:    every placeholder supplied with a distinct string value
//   - empty:   no keys at all (exercises the "<no value>" missing path)
//   - partial: every second placeholder omitted (mixed present/missing)
//
// This is the E6-S2 host byte-identity gate: if it passes, validate
// goldens and `prism errors lookup` output cannot have changed.
func TestRenderTemplateMatchesTextTemplate(t *testing.T) {
	regimes := []struct {
		name string
		ctx  func(fields []string) map[string]any
	}{
		{
			name: "full",
			ctx: func(fields []string) map[string]any {
				m := map[string]any{}
				for _, f := range fields {
					m[f] = "val_" + f
				}
				return m
			},
		},
		{
			name: "empty",
			ctx:  func(fields []string) map[string]any { return map[string]any{} },
		},
		{
			name: "partial",
			ctx: func(fields []string) map[string]any {
				m := map[string]any{}
				for i, f := range fields {
					if i%2 == 0 {
						m[f] = "val_" + f
					}
				}
				return m
			},
		},
	}

	for _, code := range CodesSorted() {
		meta := Codes[code]
		bodies := append([]string{meta.Message}, meta.Fixups...)
		for bi, body := range bodies {
			fields := placeholders(body)
			for _, reg := range regimes {
				ctx := reg.ctx(fields)
				got := renderTemplate("t", body, ctx)
				want := referenceRender(body, ctx)
				if got != want {
					t.Errorf("%s body#%d regime=%s mismatch\n body:  %q\n got:   %q\n want:  %q",
						code, bi, reg.name, body, got, want)
				}
			}
		}
	}
}

// TestRenderTemplateConditionalPermutations pins the one catalog template
// that uses {{if}} guards (PRISM_SPEC_032) across every present/absent
// permutation of its two conditional fields, plus non-string truthiness
// edge cases (empty string, zero, false all read as absent).
func TestRenderTemplateConditionalPermutations(t *testing.T) {
	body := Codes["PRISM_SPEC_032"].Message
	cases := []map[string]any{
		{"Axis": "rows"},
		{"Axis": "rows", "Aggregate": "sum"},
		{"Axis": "rows", "Field": "rev"},
		{"Axis": "rows", "Aggregate": "sum", "Field": "rev"},
		{"Axis": "rows", "Aggregate": ""},            // empty string → falsy
		{"Axis": "rows", "Aggregate": 0},             // zero → falsy
		{"Axis": "rows", "Aggregate": false},         // false → falsy
		{"Axis": "rows", "Aggregate": []string{}},    // empty slice → falsy
		{"Axis": "rows", "Aggregate": []string{"x"}}, // non-empty slice → truthy
		{"Axis": "rows", "Aggregate": nil},           // explicit nil → falsy
		{},                                           // all missing
	}
	for i, ctx := range cases {
		got := renderTemplate("t", body, ctx)
		want := referenceRender(body, ctx)
		if got != want {
			t.Errorf("case %d ctx=%v mismatch\n got:  %q\n want: %q", i, ctx, got, want)
		}
	}
}

// TestFormatValueTypes confirms the interpolator formats the scalar and
// slice value types the catalog details maps actually carry identically
// to text/template.
func TestFormatValueTypes(t *testing.T) {
	body := `{{.V}}`
	vals := []any{
		"str",
		42,
		int64(7),
		3.14,
		true,
		[]string{"a", "b"},
		nil,
	}
	for _, v := range vals {
		ctx := map[string]any{"V": v}
		got := renderTemplate("t", body, ctx)
		want := referenceRender(body, ctx)
		if got != want {
			t.Errorf("value %#v: got %q want %q", v, got, want)
		}
	}
}
