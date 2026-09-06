package marks

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
)

// ColorVarRegistry accumulates distinct (light, dark) resolved
// mark-color pairs encountered during a single Encode pass when the
// active theme has DarkVariant set (E4-S3 — auto light/dark in one
// SVG). Each distinct pair is assigned a stable CSS custom-property
// name ("prism-resolved-N", no leading "--") in first-encounter
// order, so the same input spec + theme pairing always produces the
// same names across runs — required for golden-fixture stability.
//
// A nil *ColorVarRegistry means auto-dark is inactive for this encode
// call (the active theme has no DarkVariant, or this call is not the
// theme-resolution owner — see encode.Encode's isThemeOwner). Every
// mark-color call site treats nil as "bake the literal light color",
// exactly the pre-E4-S3 behavior — see scene.Style.FillVar/StrokeVar.
type ColorVarRegistry struct {
	order []resolvedPair
	index map[resolvedPair]int
}

// resolvedPair is the dedup key: the light/dark CSS color strings
// that together define one prism-resolved-N variable's two values.
type resolvedPair struct {
	light string
	dark  string
}

// NewColorVarRegistry returns an empty, ready-to-use registry.
func NewColorVarRegistry() *ColorVarRegistry {
	return &ColorVarRegistry{index: make(map[resolvedPair]int)}
}

// Resolve registers the (light, dark) color pair — deduping on the
// pair's CSS string values — and returns its stable variable name. A
// nil dark falls back to light (both values identical; still safe to
// register and dedupe against a later call that supplies a real dark
// counterpart for the same light color... note: because dedup keys on
// the (light, dark) pair jointly, a later call with a different dark
// value for the same light color intentionally gets its own distinct
// variable name — two mark instances that happen to share a light
// color but resolve to different dark counterparts are genuinely
// different resolved pairs).
//
// Returns "" when light is nil (nothing to resolve) or the receiver
// is nil (defensive; callers already gate on ColorRegistry != nil).
func (r *ColorVarRegistry) Resolve(light, dark *scene.Color) string {
	if r == nil || light == nil {
		return ""
	}
	d := dark
	if d == nil {
		d = light
	}
	key := resolvedPair{light: light.CSS(), dark: d.CSS()}
	if i, ok := r.index[key]; ok {
		return varName(i)
	}
	i := len(r.order)
	r.order = append(r.order, key)
	r.index[key] = i
	return varName(i)
}

func varName(i int) string {
	return fmt.Sprintf("prism-resolved-%d", i)
}

// ResolvedVar is one (name, light hex, dark hex) triple ready to hand
// to theme.ResolvedColorVar for CSS emission.
type ResolvedVar struct {
	Name  string
	Light string
	Dark  string
}

// Pairs returns every registered pair in first-seen order. Returns
// nil for a nil receiver (no entries to emit).
func (r *ColorVarRegistry) Pairs() []ResolvedVar {
	if r == nil {
		return nil
	}
	out := make([]ResolvedVar, len(r.order))
	for i, k := range r.order {
		out[i] = ResolvedVar{Name: varName(i), Light: k.light, Dark: k.dark}
	}
	return out
}
