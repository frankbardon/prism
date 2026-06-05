package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/theme"
	"github.com/frankbardon/prism/validate"
)

// ThemeSchemeKnown implements PRISM_SPEC_030: every scheme reference
// in the spec (scale.scheme on a color/fill/stroke channel, plus
// theme.range.*.scheme) must resolve to a registered scheme — either
// in the global d3-scale-chromatic catalogue or in theme.schemes.
//
// Unknown schemes are non-fatal at the encoder (they fall through
// to the next tier of the palette cascade) but signal a typo or a
// missing custom-scheme registration.
type ThemeSchemeKnown struct{}

// Code returns PRISM_SPEC_030.
func (ThemeSchemeKnown) Code() string { return "PRISM_SPEC_030" }

// Check walks every scheme reference and emits one diagnostic per
// unknown name.
func (ThemeSchemeKnown) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	customSchemes := collectCustomSchemes(s)
	var out []*errors.AppError
	walkSpecForSchemes(s, customSchemes, "", &out)
	return out
}

// collectCustomSchemes pulls every key declared under theme.schemes
// into a set so spec-defined schemes don't trip the rule.
func collectCustomSchemes(s *spec.Spec) map[string]bool {
	out := make(map[string]bool)
	if s.Theme == nil || s.Theme.Schemes == nil {
		return out
	}
	for k := range s.Theme.Schemes {
		out[k] = true
	}
	return out
}

func walkSpecForSchemes(s *spec.Spec, custom map[string]bool, prefix string, out *[]*errors.AppError) {
	if s == nil {
		return
	}
	checkSchemeRef(s.Theme, custom, prefix, out)
	checkEncodingSchemes(s.Encoding, custom, prefix, out)
	for i, layer := range s.Layer {
		walkSpecForSchemes(layer, custom, fmt.Sprintf("%slayer[%d].", prefix, i), out)
	}
	for i, child := range s.Concat {
		walkSpecForSchemes(child, custom, fmt.Sprintf("%sconcat[%d].", prefix, i), out)
	}
	for i, child := range s.HConcat {
		walkSpecForSchemes(child, custom, fmt.Sprintf("%shconcat[%d].", prefix, i), out)
	}
	for i, child := range s.VConcat {
		walkSpecForSchemes(child, custom, fmt.Sprintf("%svconcat[%d].", prefix, i), out)
	}
	if s.ChildSpec != nil {
		walkSpecForSchemes(s.ChildSpec, custom, prefix+"spec.", out)
	}
}

func checkSchemeRef(t *spec.ThemeOverride, custom map[string]bool, prefix string, out *[]*errors.AppError) {
	if t == nil {
		return
	}
	if t.Scheme != "" && !theme.IsSchemeRegistered(nil, t.Scheme) && !custom[t.Scheme] {
		*out = append(*out, schemeError(t.Scheme, prefix+"theme.scheme"))
	}
	if t.Range == nil {
		return
	}
	rs := []struct {
		slot *spec.RangeSlot
		name string
	}{
		{t.Range.Category, "category"},
		{t.Range.Ordinal, "ordinal"},
		{t.Range.Ramp, "ramp"},
		{t.Range.Heatmap, "heatmap"},
		{t.Range.Diverging, "diverging"},
		{t.Range.Symbol, "symbol"},
		{t.Range.Cyclic, "cyclic"},
	}
	for _, r := range rs {
		if r.slot == nil || r.slot.Scheme == "" {
			continue
		}
		if !theme.IsSchemeRegistered(nil, r.slot.Scheme) && !custom[r.slot.Scheme] {
			*out = append(*out, schemeError(r.slot.Scheme, prefix+"theme.range."+r.name+".scheme"))
		}
	}
}

func checkEncodingSchemes(e *spec.Encoding, custom map[string]bool, prefix string, out *[]*errors.AppError) {
	if e == nil {
		return
	}
	type chref struct {
		path   string
		scheme string
	}
	var refs []chref
	if e.Color != nil && e.Color.Scale != nil && e.Color.Scale.Scheme != "" {
		refs = append(refs, chref{"encoding.color.scale.scheme", e.Color.Scale.Scheme})
	}
	if e.Fill != nil && e.Fill.Scale != nil && e.Fill.Scale.Scheme != "" {
		refs = append(refs, chref{"encoding.fill.scale.scheme", e.Fill.Scale.Scheme})
	}
	if e.Stroke != nil && e.Stroke.Scale != nil && e.Stroke.Scale.Scheme != "" {
		refs = append(refs, chref{"encoding.stroke.scale.scheme", e.Stroke.Scale.Scheme})
	}
	for _, r := range refs {
		if theme.IsSchemeRegistered(nil, r.scheme) || custom[r.scheme] {
			continue
		}
		*out = append(*out, schemeError(r.scheme, prefix+r.path))
	}
}

func schemeError(name, path string) *errors.AppError {
	return errors.New(
		"PRISM_SPEC_030",
		fmt.Sprintf("Unknown color scheme %q (at %s).", name, path),
		map[string]any{"Scheme": name, "Path": path},
	)
}
