// Package validatorutil shares the SchemaLookup construction logic the
// CLI's `prism validate` command and the Twirp `Validate` RPC both
// need. The function used to live as a private helper in
// `cmd/prism/cmd_validate.go`; P14's Twirp service forced the move
// when `rpc/` started needing the same wiring without pulling in
// `cmd/prism` (circular).
//
// BuildLookup walks the spec's data + datasets bindings and registers
// each under both an in-memory StaticLookup (so inline values feed
// semantic rules) and a DatasetLookup (so on-disk .pulse files feed
// field-existence + scale-compat rules). When any Pulse-backed
// dataset is present the result is a CompositeLookup (Pulse first,
// Static fallback); pure-inline specs get the StaticLookup alone.
package validatorutil

import (
	"path/filepath"
	"strings"

	"github.com/frankbardon/prism/internal/vfs"
	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// BuildLookup builds a SchemaLookup for the spec. fs may be nil; the
// default is afero.NewOsFs(). The function never returns nil — a spec
// with no data still gets an empty StaticLookup so callers do not
// nil-check.
func BuildLookup(s *spec.Spec, fs vfs.Fs) validate.SchemaLookup {
	if fs == nil {
		fs = vfs.OsFs()
	}
	staticLookup := validate.NewStaticLookup()
	datasetLookup := validate.NewDatasetLookup(resolve.New(nil), fs)
	usedDataset := false

	registerStatic := func(name string, ds *spec.Data) {
		if ds == nil {
			return
		}
		shim := &validate.SchemaShim{Name: name}
		if len(ds.Values) > 0 {
			seen := map[string]bool{}
			for _, row := range ds.Values {
				for k, v := range row {
					if seen[k] {
						continue
					}
					seen[k] = true
					shim.Fields = append(shim.Fields, validate.FieldShim{
						Name: k, Type: inferMeasureType(v),
					})
				}
			}
		}
		for _, f := range ds.Fields {
			shim.Fields = append(shim.Fields, validate.FieldShim{
				Name: f.Name, Type: storageToMeasure(f.Type),
			})
		}
		if len(shim.Fields) == 0 {
			return
		}
		staticLookup.Register(name, shim)
	}

	registerDataset := func(name string, ds *spec.Data) {
		if ds == nil || ds.Source == "" {
			return
		}
		if name != "" {
			datasetLookup.Register(name, ds.Source)
			usedDataset = true
		}
		base := strings.TrimSuffix(filepath.Base(ds.Source), filepath.Ext(ds.Source))
		if base != "" && base != name {
			datasetLookup.Register(base, ds.Source)
			usedDataset = true
		}
		datasetLookup.Register(ds.Source, ds.Source)
		usedDataset = true
	}

	walk := func(name string, ds *spec.Data) {
		registerStatic(name, ds)
		registerDataset(name, ds)
	}

	if s != nil {
		if s.Data != nil {
			walk(s.Data.Name, s.Data)
		}
		for name, ds := range s.Datasets {
			walk(name, ds)
		}
	}

	if !usedDataset {
		return staticLookup
	}
	return validate.NewCompositeLookup(datasetLookup, staticLookup)
}

// inferMeasureType maps a Go scalar value to a Prism measure-type
// bucket. Mirrors the cmd-side helper byte-for-byte; kept private
// here so callers do not couple to the bucket strings.
func inferMeasureType(v any) string {
	switch v.(type) {
	case float64, float32, int, int64, int32:
		return "quantitative"
	case bool:
		return "nominal"
	case string:
		return "nominal"
	default:
		return ""
	}
}

// storageToMeasure folds storage-type tokens
// (int/float/string/...) into a measure-type bucket. Matches the
// CLI-side helper byte-for-byte (unknowns fall through to nominal).
func storageToMeasure(storage string) string {
	switch strings.ToLower(storage) {
	case "int", "int8", "int16", "int32", "int64", "float", "float32", "float64":
		return "quantitative"
	case "date", "datetime", "duration":
		return "temporal"
	default:
		return "nominal"
	}
}
