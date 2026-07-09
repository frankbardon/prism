package resolve_test

import (
	"context"
	"testing"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
)

// TestPrismResolverPathForms exercises the Pulse-free inline resolution
// seam (ResolveInline). The byte Resolve loader was removed in epic E4,
// so refs no longer stream `.pulse` bytes; instead ResolveInline
// canonicalises cohort:<id> indirection through the Registry and
// delegates to a DataResolver. This test pins the ref-form
// classification and the happy-path dataset handoff.
func TestPrismResolverPathForms(t *testing.T) {
	ctx := context.Background()

	rows := []map[string]any{
		{"brand_id": "alpha", "score": 0.5},
		{"brand_id": "beta", "score": 0.7},
	}
	data := resolve.MapDataResolver{
		"cohort_a": {Values: rows, Fields: []spec.FieldSpec{
			{Name: "brand_id", Type: "categorical_u8"},
			{Name: "score", Type: "f64"},
		}},
	}

	t.Run("plain_ref", func(t *testing.T) {
		r := resolve.NewWithData(nil, data)
		ds, err := r.ResolveInline(ctx, "cohort_a")
		if err != nil {
			t.Fatalf("ResolveInline(cohort_a): %v", err)
		}
		if len(ds.Fields) != 2 {
			t.Fatalf("fields = %d, want 2", len(ds.Fields))
		}
		if got := fieldNames(ds); !equalSlice(got, []string{"brand_id", "score"}) {
			t.Fatalf("fields = %v, want [brand_id score]", got)
		}
		if len(ds.Values) != 2 {
			t.Fatalf("values = %d, want 2", len(ds.Values))
		}
	})

	t.Run("cohort_id", func(t *testing.T) {
		reg := resolve.MapRegistry{"tiny": "cohort_a"}
		r := resolve.NewWithData(reg, data)
		ds, err := r.ResolveInline(ctx, "cohort:tiny")
		if err != nil {
			t.Fatalf("ResolveInline(cohort:tiny): %v", err)
		}
		if len(ds.Fields) != 2 {
			t.Fatalf("cohort:id fields = %d, want 2", len(ds.Fields))
		}
	})

	t.Run("gcs_unavailable", func(t *testing.T) {
		r := resolve.NewWithData(nil, data)
		_, err := r.ResolveInline(ctx, "gs://bucket/x")
		assertCode(t, err, "PRISM_RESOLVE_GCS_UNAVAILABLE")
	})

	t.Run("malformed_ref", func(t *testing.T) {
		r := resolve.NewWithData(nil, data)
		_, err := r.ResolveInline(ctx, "")
		assertCode(t, err, "PRISM_RESOLVE_005")
		_, err = r.ResolveInline(ctx, "cohort:")
		assertCode(t, err, "PRISM_RESOLVE_005")
	})

	t.Run("cohort_id_not_registered", func(t *testing.T) {
		r := resolve.NewWithData(nil, data)
		_, err := r.ResolveInline(ctx, "cohort:unknown")
		assertCode(t, err, "PRISM_RESOLVE_004")
	})

	t.Run("ref_unresolved_no_data_resolver", func(t *testing.T) {
		r := resolve.New(nil)
		_, err := r.ResolveInline(ctx, "anything")
		assertCode(t, err, "PRISM_RESOLVE_REF_UNRESOLVED")
	})

	t.Run("ref_unresolved_unknown_ref", func(t *testing.T) {
		r := resolve.NewWithData(nil, data)
		_, err := r.ResolveInline(ctx, "not_in_map")
		assertCode(t, err, "PRISM_RESOLVE_REF_UNRESOLVED")
	})
}

// assertCode fails unless err is an *AppError carrying want.
func assertCode(t *testing.T, err error, want string) {
	t.Helper()
	ae, ok := err.(*prismerrors.AppError)
	if !ok {
		t.Fatalf("expected *AppError, got %T (%v)", err, err)
	}
	if ae.Code != want {
		t.Fatalf("code = %s, want %s (msg=%s)", ae.Code, want, ae.Message)
	}
}

// fieldNames extracts ordered field names from an inline Dataset.
func fieldNames(ds *resolve.Dataset) []string {
	out := make([]string, 0, len(ds.Fields))
	for _, f := range ds.Fields {
		out = append(out, f.Name)
	}
	return out
}

// equalSlice reports element-wise string slice equality.
func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
