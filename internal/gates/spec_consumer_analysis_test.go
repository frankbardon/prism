// spec_consumer_analysis_test.go carries the typed-reference analysis the
// dead-spec-field gate and the inert-table sync gate both run on.
//
// The analysis answers exactly one question per field: does any non-test Go
// file outside package spec/ contain a *type-checked* reference to it? The
// emphasis on type-checked is the whole point. A textual search for
// ".FillOpacity" cannot tell spec.MarkDef.FillOpacity from
// theme.MarkStyle.FillOpacity — identically named fields are everywhere in
// this repo (CLAUDE.md's Gotchas call it out, and it already produced one
// wrong conclusion during the vega-lite-parity audit). So instead of matching
// text, the analysis type-checks every package and compares the *types.Var
// object each selector resolves to against the field objects of the imported
// spec package. Two fields with the same name are two different objects, and
// a promoted field through an embedded struct is attributed to the struct
// that actually declares it.
//
// Mechanics: `go list -deps -export -json` hands us the compiler's export
// data for every dependency, so imports resolve from the build cache instead
// of being re-type-checked from source. That keeps the whole analysis around
// a second, which matters because internal/gates runs on every CI build.
package gates

import (
	"encoding/json"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	specPkgPath   = "github.com/frankbardon/prism/spec"
	prismModePath = "github.com/frankbardon/prism"
)

// reporterFiles are non-test files that mention spec fields in order to
// *report* on them rather than to act on them. A reference from one of these
// is not a consumer: encode/inert.go names every field it believes dead, so
// counting it would make the gate assert nothing at all.
var reporterFiles = map[string]bool{
	filepath.Join("encode", "inert.go"): true,
}

// specField is one exported, JSON-tagged field on a spec/ struct.
type specField struct {
	Struct string // Go type name, e.g. "MarkDef"
	Field  string // Go field name, e.g. "StrokeDash"
	JSON   string // wire key, e.g. "stroke_dash"
}

func (f specField) key() string { return f.Struct + "." + f.Field }

// specConsumers is the result of one analysis run.
type specConsumers struct {
	// fields is every exported, JSON-tagged spec field, keyed "Struct.Field".
	fields map[string]specField
	// consumed reports, per "Struct.Field" key, whether a type-checked
	// reference exists outside spec/ and outside reporterFiles.
	consumed map[string]bool
	// byJSON indexes fields by "Struct" + wire key so a schema-driven gate
	// can go from a JSON property name back to the Go field.
	byJSON map[string]specField
}

// consumedJSON reports whether the field of structName carrying the given
// wire key has a consumer. The second result is false when no such field
// exists, which is itself a finding for a schema-sync gate.
func (c *specConsumers) consumedJSON(structName, jsonKey string) (bool, bool) {
	f, ok := c.byJSON[structName+"|"+jsonKey]
	if !ok {
		return false, false
	}
	return c.consumed[f.key()], true
}

type goListPkg struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	Export     string
}

// listPkgs runs `go list -deps -export -json` and indexes the result by
// import path. extraEnv lets the caller retarget the platform, which is how
// the js/wasm entry point — invisible to a default-platform listing, and a
// genuine consumer of the spec types — gets covered.
func listPkgs(t *testing.T, root string, args, extraEnv []string) map[string]goListPkg {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list", "-deps", "-export", "-json"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("go list %v: %v\n%s", args, err, stderr)
	}
	pkgs := map[string]goListPkg{}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for {
		var p goListPkg
		err := dec.Decode(&p)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode `go list` output: %v", err)
		}
		pkgs[p.ImportPath] = p
	}
	return pkgs
}

// analyzeSpecConsumers type-checks the module and reports which spec fields
// are referenced from outside spec/. It covers two platforms: the host
// listing (`./...`) and the js/wasm entry, which is build-gated out of the
// host graph.
func analyzeSpecConsumers(t *testing.T) *specConsumers {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}

	targets := []struct {
		name string
		args []string
		env  []string
	}{
		{name: "host", args: []string{"./..."}},
		{name: "wasm", args: []string{"./cmd/prismwasm"}, env: []string{"GOOS=js", "GOARCH=wasm"}},
	}

	res := &specConsumers{
		fields:   map[string]specField{},
		consumed: map[string]bool{},
		byJSON:   map[string]specField{},
	}
	fieldsLoaded := false

	for _, tgt := range targets {
		pkgs := listPkgs(t, root, tgt.args, tgt.env)
		fset := token.NewFileSet()
		// One importer per platform: export data is platform-specific, and
		// the importer caches packages so every consumer shares a single
		// *types.Package for spec (pointer identity across packages is what
		// makes field attribution work).
		imp := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
			p, ok := pkgs[path]
			if !ok || p.Export == "" {
				return nil, os.ErrNotExist
			}
			return os.Open(p.Export)
		})
		specPkg, err := imp.Import(specPkgPath)
		if err != nil {
			t.Fatalf("%s: import %s: %v", tgt.name, specPkgPath, err)
		}
		owner := specFieldObjects(specPkg, res, !fieldsLoaded)
		fieldsLoaded = true

		for path, p := range pkgs {
			if !strings.HasPrefix(path, prismModePath) || path == specPkgPath || len(p.GoFiles) == 0 {
				continue
			}
			files := make([]*ast.File, 0, len(p.GoFiles))
			for _, name := range p.GoFiles {
				af, err := parser.ParseFile(fset, filepath.Join(p.Dir, name), nil, 0)
				if err != nil {
					t.Fatalf("%s: parse %s: %v", tgt.name, name, err)
				}
				files = append(files, af)
			}
			info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
			var checkErrs []string
			conf := types.Config{
				Importer: imp,
				Error:    func(e error) { checkErrs = append(checkErrs, e.Error()) },
			}
			_, _ = conf.Check(path, fset, files, info)
			if len(checkErrs) > 0 {
				// A package that does not type-check yields incomplete Uses
				// data, which would silently under-report consumers and so
				// over-report dead fields. Never let that pass quietly.
				t.Fatalf("%s: type-checking %s failed (%d errors); the dead-field gate "+
					"cannot trust an incomplete result. First: %s",
					tgt.name, path, len(checkErrs), checkErrs[0])
			}
			for id, obj := range info.Uses {
				v, ok := obj.(*types.Var)
				if !ok || !v.IsField() {
					continue
				}
				key, ok := owner[v]
				if !ok {
					continue
				}
				rel, err := filepath.Rel(root, fset.Position(id.Pos()).Filename)
				if err == nil && reporterFiles[rel] {
					continue
				}
				res.consumed[key] = true
			}
		}
	}
	if len(res.fields) == 0 {
		t.Fatal("no spec fields discovered — the analysis is not doing anything")
	}
	return res
}

// specFieldObjects maps every field object of every struct in the spec
// package to its "Struct.Field" key, and (when record is set) populates the
// wire-field inventory on res.
func specFieldObjects(specPkg *types.Package, res *specConsumers, record bool) map[*types.Var]string {
	owner := map[*types.Var]string{}
	scope := specPkg.Scope()
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			key := name + "." + f.Name()
			owner[f] = key
			if !record {
				continue
			}
			jsonName := strings.Split(reflect.StructTag(st.Tag(i)).Get("json"), ",")[0]
			// Unexported fields are not a spec surface. An embedded struct
			// carries no wire key of its own — its fields are enumerated
			// under the type that declares them. `json:"-"` is an internal
			// binding target, not a wire key, and a field with no tag at all
			// is decoded by a hand-written UnmarshalJSON (the union types).
			if !f.Exported() || f.Embedded() || jsonName == "" || jsonName == "-" {
				continue
			}
			sf := specField{Struct: name, Field: f.Name(), JSON: jsonName}
			res.fields[key] = sf
			res.byJSON[name+"|"+jsonName] = sf
		}
	}
	return owner
}
