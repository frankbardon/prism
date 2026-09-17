package encode_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// axisOptsBypassAllowlist names the only functions permitted to hand a
// bare DefaultAxisOpts(...) to BuildAxisWithOpts. BuildAxis is the
// documented default-opts convenience wrapper — it *is* the default
// path, so it is not a bypass.
var axisOptsBypassAllowlist = map[string]string{
	"BuildAxis": "the public convenience wrapper whose contract is `uses DefaultAxisOpts`",
}

// TestPrismAxisCallSitesResolvePerChannelOpts pins the E3-S5 fix: every
// BuildAxisWithOpts call in the encoder must resolve the spec's
// per-channel `axis` block instead of passing DefaultAxisOpts. Five
// call sites (shared layer x/y, shared facet x/y, histogram y) silently
// discarded axis config under the *default* resolve mode; this gate
// stops a sixth from being added unnoticed.
func TestPrismAxisCallSitesResolvePerChannelOpts(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no encode sources found")
	}
	checked := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			enclosing := fn.Name.Name
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !isIdent(call.Fun, "BuildAxisWithOpts") {
					return true
				}
				checked++
				if len(call.Args) != 5 {
					t.Errorf("%s: BuildAxisWithOpts takes %d args, want 5",
						fset.Position(call.Pos()), len(call.Args))
					return true
				}
				inner, ok := call.Args[4].(*ast.CallExpr)
				if !ok || !isIdent(inner.Fun, "DefaultAxisOpts") {
					return true
				}
				if reason, allowed := axisOptsBypassAllowlist[enclosing]; allowed {
					t.Logf("%s: DefaultAxisOpts allowed in %s (%s)",
						fset.Position(call.Pos()), enclosing, reason)
					return true
				}
				t.Errorf("%s: %s passes DefaultAxisOpts to BuildAxisWithOpts; "+
					"resolve the channel's axis block (axisOptsFor / axisOptsForTitled / "+
					"sharedAxisOpts) so `axis` config survives composition (E3-S5)",
					fset.Position(call.Pos()), enclosing)
				return true
			})
		}
	}
	if checked == 0 {
		t.Fatal("found no BuildAxisWithOpts call sites; the gate is not watching anything")
	}
}

func isIdent(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}
