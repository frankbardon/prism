package build_test

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile/inmem"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/plan/build"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

const refSpecJSON = `{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {"ref": "current_window"},
  "mark": "bar",
  "encoding": {
    "x": {"field": "label", "type": "nominal"},
    "y": {"field": "value", "type": "quantitative"}
  }
}`

func TestPrismDAGBuildRefResolves(t *testing.T) {
	s, err := spec.DecodeBytes([]byte(refSpecJSON))
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	resolver := resolve.MapDataResolver{
		"current_window": {
			Values: []map[string]any{
				{"label": "alpha", "value": 1.0},
				{"label": "beta", "value": 2.0},
			},
		},
	}
	dag, tip, err := build.Build(s, build.Options{
		FS:           afero.NewMemMapFs(),
		Resolver:     resolve.New(nil),
		Backend:      inmem.New(),
		DataResolver: resolver,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if dag == nil {
		t.Fatal("DAG nil")
	}
	if tip == "" {
		t.Fatal("tip empty")
	}

	// Execute the DAG and confirm the resolver-provided values flow
	// through end-to-end.
	res, err := plan.Execute(context.Background(), dag, plan.ExecOpts{Workers: 1})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(res.Errors) > 0 {
		t.Fatalf("Execute errors: %v", res.Errors)
	}
	tbl, ok := res.Tables[tip]
	if !ok || tbl == nil {
		t.Fatal("tip table missing")
	}
	if tbl.NumRows() != 2 {
		t.Errorf("tip rows = %d want 2", tbl.NumRows())
	}
}

func TestPrismDAGBuildRefMissingResolver(t *testing.T) {
	s, err := spec.DecodeBytes([]byte(refSpecJSON))
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	_, _, err = build.Build(s, build.Options{
		FS:       afero.NewMemMapFs(),
		Resolver: resolve.New(nil),
		Backend:  inmem.New(),
		// DataResolver intentionally nil.
	})
	if err == nil {
		t.Fatal("expected PRISM_RESOLVE_REF_UNRESOLVED")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if ae.Code != "PRISM_RESOLVE_REF_UNRESOLVED" {
		t.Errorf("Code = %q want PRISM_RESOLVE_REF_UNRESOLVED", ae.Code)
	}
}

func TestPrismDAGBuildRefUnknownRef(t *testing.T) {
	s, err := spec.DecodeBytes([]byte(refSpecJSON))
	if err != nil {
		t.Fatalf("DecodeBytes: %v", err)
	}
	_, _, err = build.Build(s, build.Options{
		FS:           afero.NewMemMapFs(),
		Resolver:     resolve.New(nil),
		Backend:      inmem.New(),
		DataResolver: resolve.MapDataResolver{},
	})
	if err == nil {
		t.Fatal("expected PRISM_RESOLVE_REF_UNRESOLVED for missing ref")
	}
	var ae *prismerrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if ae.Code != "PRISM_RESOLVE_REF_UNRESOLVED" {
		t.Errorf("Code = %q want PRISM_RESOLVE_REF_UNRESOLVED", ae.Code)
	}
}
