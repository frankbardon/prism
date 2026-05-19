// Package resolve carries cross-layer scale + axis resolution. The
// encoder consults it for composite (layer) charts to decide whether
// each channel resolves once across layers ("shared") or once per
// layer ("independent"). Defaults match Vega-Lite: x/y shared, color/
// size/shape/opacity independent.
//
// Shared-scale type compatibility is checked by Unify in domain.go;
// incompatible combinations raise PRISM_PLAN_005.
//
// Per design/04-multi-source.md, axis resolution defaults to follow
// scale resolution: if scale is shared but axis isn't explicitly set,
// the axis is also shared. The spec can override either independently.
package resolve

import (
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// Mode is the per-channel resolution discriminator. Two values only;
// kept as a typed string for spec-parity with `resolve.scale.<ch>`.
type Mode string

const (
	// ModeShared resolves the channel once across all layers (domain
	// union for numeric/temporal, ordered category union for band/
	// ordinal/point).
	ModeShared Mode = "shared"
	// ModeIndependent resolves the channel once per layer (each layer
	// gets its own scale + axis).
	ModeIndependent Mode = "independent"
)

// ChannelResolution carries the scale + axis decision for one channel.
// Axis defaults to follow Scale when the spec leaves it unset.
type ChannelResolution struct {
	Scale Mode
	Axis  Mode
}

// Defaults returns the Vega-Lite-parity default resolution map. The
// returned map is freshly allocated; callers may mutate.
//
// Defaults:
//   - x / y: Scale shared, Axis shared (one set of axes across layers)
//   - color / size / shape / opacity: Scale independent, Axis
//     independent (per-layer legends; "axis" is read as "legend" for
//     these channels via design/04-multi-source.md).
func Defaults() map[scene.Channel]ChannelResolution {
	return map[scene.Channel]ChannelResolution{
		scene.ChannelX:       {Scale: ModeShared, Axis: ModeShared},
		scene.ChannelY:       {Scale: ModeShared, Axis: ModeShared},
		scene.ChannelX2:      {Scale: ModeShared, Axis: ModeShared},
		scene.ChannelY2:      {Scale: ModeShared, Axis: ModeShared},
		scene.ChannelColor:   {Scale: ModeIndependent, Axis: ModeIndependent},
		scene.ChannelSize:    {Scale: ModeIndependent, Axis: ModeIndependent},
		scene.ChannelShape:   {Scale: ModeIndependent, Axis: ModeIndependent},
		scene.ChannelOpacity: {Scale: ModeIndependent, Axis: ModeIndependent},
	}
}

// FromSpec overlays a `spec.Resolve` block on top of Defaults().
// When the spec sets `resolve.scale.<ch>` but not `resolve.axis.<ch>`,
// axis follows scale (design/04-multi-source.md). When only axis is
// set, scale stays at its default — letting users keep shared scales
// but independent axes (the "twin-axis with same domain" case).
//
// Unknown / empty mode strings are ignored (defaults win) so a
// half-filled spec block does not silently flip behaviour.
func FromSpec(r *spec.Resolve) map[scene.Channel]ChannelResolution {
	out := Defaults()
	if r == nil {
		return out
	}
	if r.Scale != nil {
		applyChannelMap(out, r.Scale, true)
	}
	if r.Axis != nil {
		applyChannelMap(out, r.Axis, false)
	}
	// Legend defaults are handled in the encoder for now (color/size/
	// shape/opacity); resolve.legend belongs on a future LegendMode if
	// the encoder grows richer per-channel legend wiring.
	return out
}

func applyChannelMap(out map[scene.Channel]ChannelResolution, m *spec.ResolveChannelMap, scaleSide bool) {
	pairs := []struct {
		ch    scene.Channel
		value string
	}{
		{scene.ChannelX, m.X},
		{scene.ChannelY, m.Y},
		{scene.ChannelX2, m.X2},
		{scene.ChannelY2, m.Y2},
		{scene.ChannelColor, m.Color},
		{scene.ChannelSize, m.Size},
		{scene.ChannelShape, m.Shape},
		{scene.ChannelOpacity, m.Opacity},
	}
	for _, p := range pairs {
		mode, ok := parseMode(p.value)
		if !ok {
			continue
		}
		cur := out[p.ch]
		if scaleSide {
			cur.Scale = mode
			// Axis follows scale when the spec did not set axis
			// explicitly. Callers that flip only axis later (second
			// call into applyChannelMap with scaleSide=false) override
			// this. The "axis defaults to scale" rule lives in
			// design/04-multi-source.md.
			cur.Axis = mode
		} else {
			cur.Axis = mode
		}
		out[p.ch] = cur
	}
}

func parseMode(s string) (Mode, bool) {
	switch Mode(s) {
	case ModeShared:
		return ModeShared, true
	case ModeIndependent:
		return ModeIndependent, true
	}
	return "", false
}
