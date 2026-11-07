package marks

import (
	"fmt"
	"hash/fnv"

	"github.com/frankbardon/prism/encode/scene"
)

// ColorVarRegistry accumulates distinct (light, dark) resolved
// mark-color pairs encountered during a single Encode pass when the
// active theme has DarkVariant set (E4-S3 — auto light/dark in one
// SVG). Each distinct pair is assigned a CSS custom-property name
// derived from its two values ("prism-resolved-<light>-<dark>", no
// leading "--"; see resolvedPair.varName) and emitted in
// first-encounter order, so the same input spec + theme pairing always
// produces the same CSS — required for golden-fixture stability.
//
// One registry serves a whole document: a composition's top-level
// encoder creates it and hands it to every cell and layer through
// EncodeOpts (encode/autodark.go), so layered and concatenated marks
// and legend swatches are paired too, and the owner emits the CSS
// once.
//
// A nil *ColorVarRegistry means auto-dark is inactive for this encode
// call (the active theme has no DarkVariant). Every
// mark-color call site treats nil as "bake the literal light color",
// exactly the pre-E4-S3 behavior — see scene.Style.FillVar/StrokeVar.
type ColorVarRegistry struct {
	order []resolvedPair
	index map[resolvedPair]int
}

// resolvedPair is the dedup key: the light/dark CSS color strings
// that together define one prism-resolved-* variable's two values.
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
	if _, ok := r.index[key]; !ok {
		r.index[key] = len(r.order)
		r.order = append(r.order, key)
	}
	return key.varName()
}

// varName names the pair by its two values — "prism-resolved-" plus
// the light and the dark hex digits ("prism-resolved-4c78a8-4269d0"),
// or a hash of both CSS strings for a colour that is not opaque hex.
// A name that depends only on the values is what lets several Prism
// SVGs sit inline in one document: every <style> block that declares
// a name declares it with the same pair, so whichever block the page
// applies last, no chart picks up another chart's colours. (A
// first-seen counter, the E4-S3 scheme, restarted in every SVG.)
func (p resolvedPair) varName() string {
	if l, ok := opaqueHexDigits(p.light); ok {
		if d, ok := opaqueHexDigits(p.dark); ok {
			return "prism-resolved-" + l + "-" + d
		}
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(p.light + "|" + p.dark))
	return fmt.Sprintf("prism-resolved-%08x", h.Sum32())
}

// opaqueHexDigits returns the six digits of a "#rrggbb" colour.
func opaqueHexDigits(css string) (string, bool) {
	if len(css) != 7 || css[0] != '#' {
		return "", false
	}
	for _, c := range css[1:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return "", false
		}
	}
	return css[1:], true
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
		out[i] = ResolvedVar{Name: k.varName(), Light: k.light, Dark: k.dark}
	}
	return out
}
