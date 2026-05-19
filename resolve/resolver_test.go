package resolve_test

import (
	"context"
	"io"
	"testing"

	"github.com/frankbardon/pulse"
	"github.com/frankbardon/pulse/encoding"
	"github.com/frankbardon/pulse/synth"
	"github.com/spf13/afero"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/resolve"
)

// tinySpec returns a 100-row synth spec with three fields used by every
// subtest. Keep this small — these tests run on every CI invocation.
func tinySpec(rows int) *synth.Spec {
	return &synth.Spec{
		RowCount: rows,
		Fields: []synth.FieldSpec{
			{
				Name:         "brand_id",
				Type:         "categorical_u8",
				Distribution: "weighted_categorical",
				Params: map[string]any{
					"values":  []any{"alpha", "beta", "gamma"},
					"weights": []any{0.5, 0.3, 0.2},
				},
			},
			{
				Name:         "score",
				Type:         "f64",
				Distribution: "normal",
				Params:       map[string]any{"mean": 0.5, "std": 0.1},
			},
			{
				Name:         "age",
				Type:         "u8",
				Distribution: "normal",
				Params:       map[string]any{"mean": 35, "std": 11, "min": 18, "max": 95},
			},
		},
	}
}

// writeCohort synthesises a .pulse onto fs at path. Returns the pulse
// instance so callers can use it for additional ops if needed.
func writeCohort(t *testing.T, fs afero.Fs, path string, spec *synth.Spec) {
	t.Helper()
	p, err := pulse.New(pulse.Options{FS: fs})
	if err != nil {
		t.Fatalf("pulse.New: %v", err)
	}
	if _, err := p.Synth(context.Background(), spec, path, pulse.SynthOptions{Seed: 42}); err != nil {
		t.Fatalf("Synth(%s): %v", path, err)
	}
}

func TestPrismResolverPathForms(t *testing.T) {
	t.Run("local_path", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeCohort(t, fs, "tiny.pulse", tinySpec(100))

		r := resolve.New(nil)
		rc, schema, err := r.Resolve("tiny.pulse", fs)
		if err != nil {
			t.Fatalf("Resolve(tiny.pulse): %v", err)
		}
		defer rc.Close()
		if schema == nil {
			t.Fatalf("schema is nil")
		}
		gotFields := fieldNames(schema)
		if !equalSlice(gotFields, []string{"brand_id", "score", "age"}) {
			t.Fatalf("schema fields = %v, want [brand_id score age]", gotFields)
		}
		// Drain the payload to ensure the reader is usable.
		if _, err := io.Copy(io.Discard, rc); err != nil {
			t.Fatalf("drain payload: %v", err)
		}
	})

	t.Run("archive_anchor", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeCohort(t, fs, "shard_0.pulse", tinySpec(50))
		writeCohort(t, fs, "shard_1.pulse", tinySpec(50))

		p, err := pulse.New(pulse.Options{FS: fs})
		if err != nil {
			t.Fatalf("pulse.New: %v", err)
		}
		if err := p.CreateShardArchive(context.Background(), "archive.pulse",
			[]string{"shard_0.pulse", "shard_1.pulse"}); err != nil {
			t.Fatalf("CreateShardArchive: %v", err)
		}

		r := resolve.New(nil)
		rc, schema, err := r.Resolve("archive.pulse#shard_0.pulse", fs)
		if err != nil {
			t.Fatalf("Resolve archive anchor: %v", err)
		}
		defer rc.Close()
		if schema == nil {
			t.Fatalf("schema is nil")
		}
		if got := fieldNames(schema); !equalSlice(got, []string{"brand_id", "score", "age"}) {
			t.Fatalf("anchor schema fields = %v, want [brand_id score age]", got)
		}
	})

	t.Run("cohort_id", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeCohort(t, fs, "tiny.pulse", tinySpec(100))

		reg := resolve.MapRegistry{"tiny": "tiny.pulse"}
		r := resolve.New(reg)
		rc, schema, err := r.Resolve("cohort:tiny", fs)
		if err != nil {
			t.Fatalf("Resolve cohort:tiny: %v", err)
		}
		defer rc.Close()
		if schema == nil || len(schema.Fields) != 3 {
			t.Fatalf("cohort:id schema malformed: %v", schema)
		}
	})

	t.Run("gcs_unavailable", func(t *testing.T) {
		r := resolve.New(nil)
		_, _, err := r.Resolve("gs://bucket/x.pulse", afero.NewMemMapFs())
		ae, ok := err.(*prismerrors.AppError)
		if !ok {
			t.Fatalf("expected *AppError, got %T (%v)", err, err)
		}
		if ae.Code != "PRISM_RESOLVE_GCS_UNAVAILABLE" {
			t.Fatalf("got %s, want PRISM_RESOLVE_GCS_UNAVAILABLE", ae.Code)
		}
	})

	t.Run("malformed_ref", func(t *testing.T) {
		r := resolve.New(nil)

		_, _, err := r.Resolve("", afero.NewMemMapFs())
		if ae, ok := err.(*prismerrors.AppError); !ok || ae.Code != "PRISM_RESOLVE_005" {
			t.Fatalf("empty ref: got %v, want PRISM_RESOLVE_005", err)
		}

		_, _, err = r.Resolve("cohort:", afero.NewMemMapFs())
		if ae, ok := err.(*prismerrors.AppError); !ok || ae.Code != "PRISM_RESOLVE_005" {
			t.Fatalf("cohort: ref: got %v, want PRISM_RESOLVE_005", err)
		}
	})

	t.Run("cohort_id_not_registered", func(t *testing.T) {
		r := resolve.New(nil)
		_, _, err := r.Resolve("cohort:unknown", afero.NewMemMapFs())
		ae, ok := err.(*prismerrors.AppError)
		if !ok || ae.Code != "PRISM_RESOLVE_004" {
			t.Fatalf("got %v, want PRISM_RESOLVE_004", err)
		}
	})

	t.Run("local_path_missing", func(t *testing.T) {
		r := resolve.New(nil)
		_, _, err := r.Resolve("nope.pulse", afero.NewMemMapFs())
		ae, ok := err.(*prismerrors.AppError)
		if !ok || ae.Code != "PRISM_RESOLVE_002" {
			t.Fatalf("got %v, want PRISM_RESOLVE_002", err)
		}
	})

	t.Run("archive_missing_shard", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		writeCohort(t, fs, "shard_0.pulse", tinySpec(20))
		p, err := pulse.New(pulse.Options{FS: fs})
		if err != nil {
			t.Fatalf("pulse.New: %v", err)
		}
		if err := p.CreateShardArchive(context.Background(), "archive.pulse",
			[]string{"shard_0.pulse"}); err != nil {
			t.Fatalf("CreateShardArchive: %v", err)
		}
		r := resolve.New(nil)
		_, _, err = r.Resolve("archive.pulse#missing.pulse", fs)
		ae, ok := err.(*prismerrors.AppError)
		if !ok {
			t.Fatalf("expected *AppError, got %T (%v)", err, err)
		}
		if ae.Code != "PRISM_RESOLVE_003" {
			t.Fatalf("got %s, want PRISM_RESOLVE_003 (msg=%s)", ae.Code, ae.Message)
		}
	})
}

// fieldNames extracts ordered field names from a Pulse schema.
func fieldNames(s *encoding.Schema) []string {
	out := make([]string, 0, len(s.Fields))
	for _, f := range s.Fields {
		out = append(out, f.Name)
	}
	return out
}

// equalSlice mirrors the helper in table_test.go (kept local to avoid
// exporting test-only utilities).
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
