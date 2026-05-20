// Package validatorutil shares the SchemaLookup construction logic the
// CLI's `prism validate` command and the Twirp `Validate` RPC both
// need. The function used to live as a private helper in
// `cmd/prism/cmd_validate.go`; P14's Twirp service forced the move
// when `rpc/` started needing the same wiring without pulling in
// `cmd/prism` (circular).
//
// BuildLookup walks the spec's data + datasets bindings and registers
// each under both an in-memory StaticLookup (so inline values feed
// semantic rules) and a PulseLookup (so on-disk .pulse files feed
// field-existence + scale-compat rules). When any Pulse-backed
// dataset is present the result is a CompositeLookup (Pulse first,
// Static fallback); pure-inline specs get the StaticLookup alone.
package validatorutil

import (
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/resolve"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// BuildLookup builds a SchemaLookup for the spec. fs may be nil; the
// default is afero.NewOsFs(). The function never returns nil — a spec
// with no data still gets an empty StaticLookup so callers do not
// nil-check.
func BuildLookup(s *spec.Spec, fs afero.Fs) validate.SchemaLookup {
	if fs == nil {
		fs = afero.NewOsFs()
	}
	staticLookup := validate.NewStaticLookup()
	pulseLookup := validate.NewPulseLookup(resolve.New(nil), fs)
	usedPulse := false

	registerStatic := func(name string, ds *spec.Data) {
		if ds == nil {
			return
		}
		shim := &validate.PulseSchemaShim{Name: name}
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
				Name: f.Name, Type: pulseStorageToMeasure(f.Type),
			})
		}
		if len(shim.Fields) == 0 {
			return
		}
		staticLookup.Register(name, shim)
	}

	registerPulse := func(name string, ds *spec.Data) {
		if ds == nil || ds.Source == "" {
			return
		}
		if name != "" {
			pulseLookup.Register(name, ds.Source)
			usedPulse = true
		}
		base := strings.TrimSuffix(filepath.Base(ds.Source), filepath.Ext(ds.Source))
		if base != "" && base != name {
			pulseLookup.Register(base, ds.Source)
			usedPulse = true
		}
		pulseLookup.Register(ds.Source, ds.Source)
		usedPulse = true
	}

	walk := func(name string, ds *spec.Data) {
		registerStatic(name, ds)
		registerPulse(name, ds)
	}

	if s != nil {
		if s.Data != nil {
			walk(s.Data.Name, s.Data)
		}
		for name, ds := range s.Datasets {
			walk(name, ds)
		}
	}

	if !usedPulse {
		return staticLookup
	}
	return validate.NewCompositeLookup(pulseLookup, staticLookup)
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

// pulseStorageToMeasure folds Pulse FieldSpec.Type tokens
// (int/float/string/...) into a measure-type bucket. Matches the
// CLI-side helper byte-for-byte (unknowns fall through to nominal).
func pulseStorageToMeasure(storage string) string {
	switch strings.ToLower(storage) {
	case "int", "int8", "int16", "int32", "int64", "float", "float32", "float64":
		return "quantitative"
	case "date", "datetime", "duration":
		return "temporal"
	default:
		return "nominal"
	}
}
