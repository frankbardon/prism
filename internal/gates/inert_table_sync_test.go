// inert_table_sync_test.go keeps the runtime inert-field detector
// (encode/inert.go) honest against the two things it describes: the JSON
// Schema bundle that advertises the keys, and the typed-consumer analysis
// that says which of them are actually read.
//
// Why this exists. encode/inert.go's markDefOwners is an allowlist of
// *checked* keys: a property missing from the table is never reported, so its
// failure mode is silent under-reporting. E7-S1 chose that on purpose (a
// false positive trains authors to ignore warnings) and asked for this gate
// as the closer. Without it, a new `mark_def` property can land in the schema,
// be implemented by nobody, and warn about nothing.
//
// Three invariants are enforced:
//
//  1. COVERAGE — every `mark_def` property and every `encoding` channel the
//     schema advertises is classified: named in the detector's table, or
//     recorded here as honoured everywhere with a reason.
//  2. NO STALE KEYS — every key the detector names exists in the schema and
//     on the Go struct, so a renamed property cannot leave a table entry
//     pointing at nothing.
//  3. TRUTH — a property the detector calls dead really has no typed
//     consumer, and a property it credits to a mark really does. This is the
//     invariant that would have caught E7-S1's legend table going stale the
//     moment E3-S4 wired the legend keys in the same wave: the detector
//     warned "not read" about keys the encoder was obeying.
//
// What it CANNOT catch: the per-mark half of markDefOwners. The gate can
// verify that `corner_radius` is read *somewhere*; it cannot verify the owner
// list says bar / sparkbar / progress / winloss and not line. That mapping is
// still maintained by hand from the reading call site.
package gates

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// honouredMarkDefProps are the schema's `mark_def` properties that
// encode/inert.go intentionally leaves out of markDefOwners because they are
// honoured for every mark, so there is no per-mark ownership to record.
var honouredMarkDefProps = map[string]string{
	"type":           "the mark discriminator itself, read through spec.Mark.TypeName().",
	"fill":           "applyMarkDef copies it onto scene.Style for every mark.",
	"stroke":         "applyMarkDef copies it onto scene.Style for every mark.",
	"stroke_width":   "applyMarkDef copies it onto scene.Style for every mark.",
	"stroke_dash":    "E7-S4 made it a universal style property: applyMarkDef folds it into scene.Style.StrokeDash and render/svg emits stroke-dasharray, so no mark type renders it inert.",
	"opacity":        "applyMarkDef copies it onto scene.Style for every mark.",
	"fill_opacity":   "applyMarkDef copies it onto scene.Style for every mark.",
	"stroke_opacity": "applyMarkDef copies it onto scene.Style for every mark.",
	"font":           "applyMarkDef copies it onto scene.Style.FontFamily; inert only on a mark that draws no text, which is not worth a warning.",
	"font_weight":    "applyMarkDef normalises it onto scene.Style.FontWeight; inert only on a mark that draws no text.",
	"font_style":     "applyMarkDef copies it onto scene.Style.FontStyle; inert only on a mark that draws no text.",
	"clip":           "E2-S2 reads it on the owning spec to clip out-of-domain marks; it applies to the layer, not to one mark family.",
	"orient":         "E9-S1 / E9-S2 read it in the cartesian mark encoders, and a mark that cannot take it is rejected at validate (PRISM_SPEC_046 / PRISM_SPEC_053) rather than warned about.",
}

// honouredChannels are the schema's `encoding` channels that reach an
// encoder, so encode/inert.go's deadChannels does not list them.
var honouredChannels = map[string]string{
	"x":         "position; resolves a cartesian scale.",
	"y":         "position; resolves a cartesian scale.",
	"x2":        "span companion, bound to x's resolved scale (encode/span.go).",
	"y2":        "span companion, bound to y's resolved scale (encode/span.go).",
	"x_offset":  "E1-S4 builds a nested band scale from it (encode/offset.go) and rectAxisExtent hands the bar its sub-band.",
	"y_offset":  "E1-S4 builds a nested band scale from it (encode/offset.go) and rectAxisExtent hands the bar its sub-band.",
	"theta":     "polar angle, read by the arc / pie / donut encoders.",
	"radius":    "polar radius, read by the arc / pie / donut encoders.",
	"color":     "drives the palette and the legend.",
	"opacity":   "read by the heatmap encoder; every other mark is reported per-mark by encode/inert.go.",
	"detail":    "grouping-only channel; encode/marks/group.go partitions rows on it.",
	"order":     "E5-S4 injects a sort node from it.",
	"text":      "text mark / label content, read by encode/marks/text.go.",
	"tooltip":   "materialised onto the scene marks for the web component.",
	"row":       "facet row binding.",
	"column":    "facet column binding.",
	"columns":   "table mark column list.",
	"value":     "table / progress cell value binding.",
	"feature":   "geoshape feature-id binding.",
	"longitude": "geopoint lon binding.",
	"latitude":  "geopoint lat binding.",
	"source":    "tree / network edge source binding.",
	"target":    "tree / network edge target binding.",
}

// schemaProps reads the property names of one $def in a schema bundle file.
func schemaProps(t *testing.T, root, file, def string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "schema", "v1", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	var doc struct {
		Defs map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	d, ok := doc.Defs[def]
	if !ok {
		t.Fatalf("%s has no $def %q", file, def)
	}
	out := map[string]bool{}
	for name := range d.Properties {
		out[name] = true
	}
	if len(out) == 0 {
		t.Fatalf("%s#/$defs/%s declares no properties", file, def)
	}
	return out
}

// parseInertFile parses encode/inert.go so the gate can read the detector's
// own tables rather than a duplicate of them. Reading the source keeps the
// tables unexported: there is no test-only accessor on package encode to
// keep in step.
func parseInertFile(t *testing.T, root string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "encode", "inert.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse encode/inert.go: %v", err)
	}
	return f
}

// mapVarKeys returns the string keys of a package-level `var name = map[...]`
// composite literal.
func mapVarKeys(t *testing.T, f *ast.File, name string) map[string]bool {
	t.Helper()
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spc := range gd.Specs {
			vs, ok := spc.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || vs.Names[0].Name != name || len(vs.Values) != 1 {
				continue
			}
			cl, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				t.Fatalf("%s is not a composite literal", name)
			}
			return litKeys(t, cl)
		}
	}
	t.Fatalf("encode/inert.go declares no var %q — the gate is out of date with the detector", name)
	return nil
}

// mapLitKeysInFunc returns the string keys of the first map composite literal
// inside the named function.
func mapLitKeysInFunc(t *testing.T, f *ast.File, fn string) map[string]bool {
	t.Helper()
	var out map[string]bool
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != fn {
			continue
		}
		ast.Inspect(fd, func(n ast.Node) bool {
			if out != nil {
				return false
			}
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if _, isMap := cl.Type.(*ast.MapType); !isMap {
				return true
			}
			out = litKeys(t, cl)
			return false
		})
	}
	if out == nil {
		t.Fatalf("encode/inert.go has no map literal inside %s()", fn)
	}
	return out
}

func litKeys(t *testing.T, cl *ast.CompositeLit) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, el := range cl.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		bl, ok := kv.Key.(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			continue
		}
		key, err := strconv.Unquote(bl.Value)
		if err != nil {
			t.Fatalf("unquote %s: %v", bl.Value, err)
		}
		out[key] = true
	}
	return out
}

// markDefOwnersDead returns the markDefOwners keys whose owner list is empty,
// i.e. the properties the detector claims no mark reads at all.
func markDefOwnersDead(t *testing.T, f *ast.File) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spc := range gd.Specs {
			vs, ok := spc.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "markDefOwners" || len(vs.Values) != 1 {
				continue
			}
			cl := vs.Values[0].(*ast.CompositeLit)
			for _, el := range cl.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				bl, ok := kv.Key.(*ast.BasicLit)
				if !ok {
					continue
				}
				key, _ := strconv.Unquote(bl.Value)
				owners, ok := kv.Value.(*ast.CompositeLit)
				if ok && len(owners.Elts) == 0 {
					out[key] = true
				}
			}
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestPrismInertTablesMatchSchema(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	inert := parseInertFile(t, root)
	owners := mapVarKeys(t, inert, "markDefOwners")
	setKeys := mapLitKeysInFunc(t, inert, "markDefSet")
	channels := mapVarKeys(t, inert, "deadChannels")

	// --- mark_def coverage, both directions -----------------------------
	schemaMarkDef := schemaProps(t, root, "mark.schema.json", "mark_def")
	var unclassified, unknown []string
	for prop := range schemaMarkDef {
		if !owners[prop] && honouredMarkDefProps[prop] == "" {
			unclassified = append(unclassified, prop)
		}
	}
	for prop := range owners {
		if !schemaMarkDef[prop] {
			unknown = append(unknown, prop)
		}
	}
	sort.Strings(unclassified)
	sort.Strings(unknown)
	if len(unclassified) > 0 {
		t.Errorf("mark_def propert(ies) advertised by schema/v1/mark.schema.json that encode/inert.go "+
			"does not classify: %s\n\n"+
			"markDefOwners is an allowlist of CHECKED keys, so an unlisted property is silent: an author "+
			"can set it, pass validation, and get nothing — with no warning. Either add it to "+
			"markDefOwners with the mark types whose encoder reads it (an empty owner list means no mark "+
			"reads it), or add it to honouredMarkDefProps in this file with the reason it is honoured "+
			"everywhere.", strings.Join(unclassified, ", "))
	}
	if len(unknown) > 0 {
		t.Errorf("markDefOwners names propert(ies) the schema does not advertise: %s\n\n"+
			"Either the property was renamed in schema/v1/mark.schema.json and the table was not "+
			"updated, or the schema is missing a property the Go struct accepts — in which case a spec "+
			"using it is rejected by `prism validate` even though the encoder reads it.",
			strings.Join(unknown, ", "))
	}
	if diff := symmetricDiff(owners, setKeys); len(diff) > 0 {
		t.Errorf("markDefOwners and markDefSet disagree on: %s\n\n"+
			"markDefSet reports which properties a spec SET, and markDefOwners decides whether to warn "+
			"about them. A key in one and not the other is never reported (or is looked up and always "+
			"misses), which is the silent failure this gate exists to stop.", strings.Join(diff, ", "))
	}

	// --- encoding channel coverage --------------------------------------
	schemaChannels := schemaProps(t, root, "encoding.schema.json", "encoding")
	var unclassifiedCh, unknownCh []string
	for ch := range schemaChannels {
		if !channels[ch] && honouredChannels[ch] == "" {
			unclassifiedCh = append(unclassifiedCh, ch)
		}
	}
	for ch := range channels {
		if !schemaChannels[ch] {
			unknownCh = append(unknownCh, ch)
		}
	}
	sort.Strings(unclassifiedCh)
	sort.Strings(unknownCh)
	if len(unclassifiedCh) > 0 {
		t.Errorf("encoding channel(s) advertised by schema/v1/encoding.schema.json that are neither "+
			"reported inert nor recorded as honoured: %s\n\n"+
			"Add the channel to encode/inert.go's deadChannels (with the reason an author sees), or to "+
			"honouredChannels in this file naming the encoder that reads it.",
			strings.Join(unclassifiedCh, ", "))
	}
	if len(unknownCh) > 0 {
		t.Errorf("deadChannels names channel(s) the schema does not advertise: %s",
			strings.Join(unknownCh, ", "))
	}
	for prop, reason := range honouredMarkDefProps {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("honouredMarkDefProps[%q] carries no reason", prop)
		}
		if !schemaMarkDef[prop] {
			t.Errorf("honouredMarkDefProps[%q] names no schema property; drop the entry", prop)
		}
	}
	for ch, reason := range honouredChannels {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("honouredChannels[%q] carries no reason", ch)
		}
		if !schemaChannels[ch] {
			t.Errorf("honouredChannels[%q] names no schema channel; drop the entry", ch)
		}
	}
}

// TestPrismInertTablesMatchConsumers is invariant 3: what the detector says
// about a mark_def property has to agree with whether anything reads it.
func TestPrismInertTablesMatchConsumers(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	inert := parseInertFile(t, root)
	owners := mapVarKeys(t, inert, "markDefOwners")
	dead := markDefOwnersDead(t, inert)
	res := analyzeSpecConsumers(t)

	var lying, missing []string
	for prop := range owners {
		consumed, ok := res.consumedJSON("MarkDef", prop)
		if !ok {
			missing = append(missing, prop)
			continue
		}
		if dead[prop] && consumed {
			lying = append(lying, prop+" (claimed dead, but a consumer reads it)")
		}
		if !dead[prop] && !consumed {
			lying = append(lying, prop+" (credited to a mark encoder, but nothing outside spec/ reads it)")
		}
	}
	sort.Strings(lying)
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("markDefOwners names propert(ies) with no matching JSON tag on spec.MarkDef: %s",
			strings.Join(missing, ", "))
	}
	if len(lying) > 0 {
		t.Errorf("encode/inert.go disagrees with the typed-consumer analysis on %d mark_def "+
			"propert(ies):\n  %s\n\n"+
			"A warning that names a key the encoder obeys is worse than no warning: it teaches authors to "+
			"ignore the whole warning family. This is exactly how the legend table went stale — E7-S1 "+
			"declared five legend keys dead in the same wave E3-S4 wired them. Update markDefOwners to "+
			"match reality (an empty owner list means nothing reads it).",
			len(lying), strings.Join(lying, "\n  "))
	}

	// The axis, scale and legend blocks carry no inert table at all, which is
	// only correct while every property they advertise has a consumer. That
	// is the headline outcome of this effort, so pin it: a new property on
	// any of the three must arrive wired, or arrive with a detector entry.
	for _, blk := range []struct{ file, def, goType string }{
		{"axis.schema.json", "axis", "Axis"},
		{"scale.schema.json", "scale", "Scale"},
		{"legend.schema.json", "legend", "Legend"},
	} {
		props := schemaProps(t, root, blk.file, blk.def)
		var deadProps, absent []string
		for _, prop := range sortedKeys(props) {
			consumed, ok := res.consumedJSON(blk.goType, prop)
			if !ok {
				absent = append(absent, prop)
				continue
			}
			if !consumed {
				deadProps = append(deadProps, prop)
			}
		}
		if len(absent) > 0 {
			t.Errorf("schema/v1/%s advertises %s propert(ies) with no matching JSON tag on spec.%s: %s",
				blk.file, blk.def, blk.goType, strings.Join(absent, ", "))
		}
		if len(deadProps) > 0 {
			t.Errorf("schema/v1/%s advertises %s propert(ies) no consumer reads: %s\n\n"+
				"Every %s property was wired during vega-lite-parity, so a dead one is new. Wire it, or "+
				"give encode/inert.go a detector entry so an author is told it does nothing.",
				blk.file, blk.def, strings.Join(deadProps, ", "), blk.def)
		}
	}
}

func symmetricDiff(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k+" (in the first, not the second)")
		}
	}
	for k := range b {
		if !a[k] {
			out = append(out, k+" (in the second, not the first)")
		}
	}
	sort.Strings(out)
	return out
}
