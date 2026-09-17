// spec_strict_decode_test.go gates that every hand-written decoder in spec/
// rejects unknown keys.
//
// package spec's doc comment promises "Decoding is strict: unknown fields
// fail", and spec.Decode does set DisallowUnknownFields. But Go does NOT
// propagate that setting into a custom UnmarshalJSON: the method receives raw
// bytes and any json.Unmarshal it calls is unconditionally lenient. E7-S2
// found the consequence by probe — an unknown key written inside
// `encoding.x`, `.color`, `.tooltip`, `.order`, `.detail` or their hand-
// decoded `axis` / `legend` sub-blocks was silently dropped — and recommended
// a gate as the durable form of the fix, because the hole reappears every
// time someone adds a decoder.
//
// The JSON Schema (`additionalProperties: false` on each $def) catches these
// keys for anyone running `prism validate`, so the silent drop was only ever
// reachable by a library caller using spec.Decode alone. That is still a
// supported entry point, hence the fix and hence this gate.
//
// Two halves, and both matter:
//
//   - COVERAGE: every `func (T) UnmarshalJSON` in spec/ has a probe here. A
//     new decoder fails the build until someone writes one, so the gate
//     cannot silently stop covering the package.
//   - BEHAVIOUR: each probe decodes a valid document (the positive control —
//     without it a malformed probe would "pass" for the wrong reason) and
//     then the same document with one unknown key added, which must fail.
package gates

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/frankbardon/prism/spec"
)

// strictProbe drives one decoder with a good document and a good document
// plus one unknown key.
type strictProbe struct {
	// valid must decode cleanly; unknown must not.
	valid   string
	unknown string
	decode  func([]byte) error
	// note explains a probe whose shape is not obvious.
	note string
}

// noObjectForm records decoders that accept no JSON object at all, so there
// is no object in which an unknown key could hide. Their probe still runs;
// this map only records that a rejection proves "not an object" rather than
// "unknown key".
var noObjectForm = map[string]string{
	"Dimension": "accepts a number or a sizing token string only; an object of any shape is rejected outright.",
}

// strictDecodeProbes maps a spec type with a custom UnmarshalJSON to its
// probe. Every such type must appear here — TestPrismSpecDecodersAreStrict
// enumerates them from the source and fails on an unprobed one.
func strictDecodeProbes() map[string]strictProbe {
	pred := `{"op": "eq", "field": "a", "value": 1}`
	return map[string]strictProbe{
		"CalcExpr": {
			valid:   `{"op": "add", "operands": [{"field": "a"}, {"field": "b"}]}`,
			unknown: `{"op": "add", "operands": [{"field": "a"}, {"field": "b"}], "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.CalcExpr).UnmarshalJSON(b) },
		},
		"Data": {
			valid:   `{"values": [{"a": 1}]}`,
			unknown: `{"values": [{"a": 1}], "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.Data).UnmarshalJSON(b) },
		},
		"Condition": {
			valid:   `{"test": ` + pred + `, "value": "red"}`,
			unknown: `{"test": ` + pred + `, "value": "red", "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.Condition).UnmarshalJSON(b) },
		},
		"PositionChannel": {
			valid:   `{"field": "a", "type": "quantitative"}`,
			unknown: `{"field": "a", "type": "quantitative", "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.PositionChannel).UnmarshalJSON(b) },
		},
		"MarkChannel": {
			valid:   `{"field": "a", "type": "nominal"}`,
			unknown: `{"field": "a", "type": "nominal", "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.MarkChannel).UnmarshalJSON(b) },
		},
		"TooltipChannel": {
			valid:   `{"field": "a", "type": "nominal"}`,
			unknown: `{"field": "a", "type": "nominal", "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.TooltipChannel).UnmarshalJSON(b) },
		},
		"OrderChannel": {
			valid:   `{"field": "a", "type": "quantitative"}`,
			unknown: `{"field": "a", "type": "quantitative", "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.OrderChannel).UnmarshalJSON(b) },
		},
		"DetailChannel": {
			valid:   `{"field": "a", "type": "nominal"}`,
			unknown: `{"field": "a", "type": "nominal", "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.DetailChannel).UnmarshalJSON(b) },
		},
		"Predicate": {
			valid:   pred,
			unknown: `{"op": "eq", "field": "a", "value": 1, "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.Predicate).UnmarshalJSON(b) },
		},
		"Mark": {
			valid:   `{"type": "bar"}`,
			unknown: `{"type": "bar", "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.Mark).UnmarshalJSON(b) },
		},
		"Selection": {
			valid:   `{"type": "point", "fields": ["a"]}`,
			unknown: `{"type": "point", "fields": ["a"], "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.Selection).UnmarshalJSON(b) },
		},
		"Transform": {
			valid:   `{"filter": ` + pred + `}`,
			unknown: `{"filter": ` + pred + `, "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.Transform).UnmarshalJSON(b) },
		},
		"Dimension": {
			valid:   `240`,
			unknown: `{"step": 10, "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.Dimension).UnmarshalJSON(b) },
			note:    "number / token only; see noObjectForm.",
		},
		"Padding": {
			valid:   `{"top": 1}`,
			unknown: `{"top": 1, "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.Padding).UnmarshalJSON(b) },
		},
		"TextOrTextObj": {
			valid:   `{"text": "hi"}`,
			unknown: `{"text": "hi", "zzz_unknown": 1}`,
			decode:  func(b []byte) error { return new(spec.TextOrTextObj).UnmarshalJSON(b) },
		},
	}
}

// nestedStrictProbes cover the blocks a channel decoder decodes BY HAND.
// Axis and Legend have no UnmarshalJSON of their own, so the source scan
// never sees them — but they are decoded from raw bytes inside
// PositionChannel / MarkChannel, which is exactly where the leniency hid.
var nestedStrictProbes = map[string]strictProbe{
	"Axis (inside PositionChannel)": {
		valid:   `{"field": "a", "type": "quantitative", "axis": {"title": "T"}}`,
		unknown: `{"field": "a", "type": "quantitative", "axis": {"title": "T", "zzz_unknown": 1}}`,
		decode:  func(b []byte) error { return new(spec.PositionChannel).UnmarshalJSON(b) },
	},
	"Legend (inside MarkChannel)": {
		valid:   `{"field": "a", "type": "nominal", "legend": {"title": "T"}}`,
		unknown: `{"field": "a", "type": "nominal", "legend": {"title": "T", "zzz_unknown": 1}}`,
		decode:  func(b []byte) error { return new(spec.MarkChannel).UnmarshalJSON(b) },
	},
	"TooltipChannel (array form)": {
		valid:   `[{"field": "a", "type": "nominal"}]`,
		unknown: `[{"field": "a", "type": "nominal", "zzz_unknown": 1}]`,
		decode:  func(b []byte) error { return new(spec.TooltipChannel).UnmarshalJSON(b) },
	},
	"OrderChannel (array form)": {
		valid:   `[{"field": "a", "type": "quantitative"}]`,
		unknown: `[{"field": "a", "type": "quantitative", "zzz_unknown": 1}]`,
		decode:  func(b []byte) error { return new(spec.OrderChannel).UnmarshalJSON(b) },
	},
	"DetailChannel (array form)": {
		valid:   `[{"field": "a", "type": "nominal"}]`,
		unknown: `[{"field": "a", "type": "nominal", "zzz_unknown": 1}]`,
		decode:  func(b []byte) error { return new(spec.DetailChannel).UnmarshalJSON(b) },
	},
	"Condition (array form)": {
		valid:   `[{"test": {"op": "eq", "field": "a", "value": 1}, "value": "red"}]`,
		unknown: `[{"test": {"op": "eq", "field": "a", "value": 1}, "value": "red", "zzz_unknown": 1}]`,
		decode:  func(b []byte) error { return new(spec.Condition).UnmarshalJSON(b) },
	},
	"Selection (interval variant)": {
		valid:   `{"type": "interval", "encodings": ["x"]}`,
		unknown: `{"type": "interval", "encodings": ["x"], "zzz_unknown": 1}`,
		decode:  func(b []byte) error { return new(spec.Selection).UnmarshalJSON(b) },
	},
}

// customUnmarshalers scans spec/'s non-test sources for UnmarshalJSON methods
// and returns the receiver type names.
func customUnmarshalers(t *testing.T, root string) []string {
	t.Helper()
	dir := filepath.Join(root, "spec")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read spec/: %v", err)
	}
	fset := token.NewFileSet()
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse spec/%s: %v", name, err)
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Name.Name != "UnmarshalJSON" || fd.Recv == nil || len(fd.Recv.List) != 1 {
				continue
			}
			typ := fd.Recv.List[0].Type
			if star, ok := typ.(*ast.StarExpr); ok {
				typ = star.X
			}
			if id, ok := typ.(*ast.Ident); ok {
				out = append(out, id.Name)
			}
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatal("found no UnmarshalJSON methods in spec/ — the scan is broken")
	}
	return out
}

func TestPrismSpecDecodersAreStrict(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	probes := strictDecodeProbes()

	// Coverage: a new hand-written decoder must arrive with a probe.
	for _, typ := range customUnmarshalers(t, root) {
		if _, ok := probes[typ]; !ok {
			t.Errorf("spec.%s declares a custom UnmarshalJSON with no probe in strictDecodeProbes.\n\n"+
				"A custom UnmarshalJSON does not inherit spec.Decode's DisallowUnknownFields — it has to "+
				"re-arm strictness itself (spec.strictUnmarshal). Add a probe: a valid document and the "+
				"same document with one unknown key, which must be rejected.", typ)
		}
	}
	for typ := range probes {
		found := false
		for _, name := range customUnmarshalers(t, root) {
			if name == typ {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("strictDecodeProbes has an entry for spec.%s, which no longer declares "+
				"UnmarshalJSON; drop the probe", typ)
		}
	}

	run := func(name string, p strictProbe) {
		t.Helper()
		if err := p.decode([]byte(p.valid)); err != nil {
			t.Errorf("%s: the probe's VALID document was rejected (%v).\n\n"+
				"The positive control failed, so the unknown-key half proves nothing — fix the probe "+
				"document, not the assertion.", name, err)
			return
		}
		if err := p.decode([]byte(p.unknown)); err == nil {
			hint := ""
			if why := noObjectForm[name]; why != "" {
				hint = " (" + why + ")"
			}
			t.Errorf("%s: an unknown key decoded silently%s.\n\n"+
				"Route the decode through spec.strictUnmarshal so the key is rejected. Leaving it lenient "+
				"means a typo in a spec field name is dropped without a word for any caller using "+
				"spec.Decode without the JSON-Schema shape stage.", name, hint)
		}
	}

	for _, name := range sortedProbeNames(probes) {
		run(name, probes[name])
	}
	for _, name := range sortedProbeNames(nestedStrictProbes) {
		run(name, nestedStrictProbes[name])
	}
}

func sortedProbeNames(m map[string]strictProbe) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
