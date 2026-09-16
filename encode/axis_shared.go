package encode

import (
	"fmt"
	"reflect"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// sharedAxisBlock is one composition child's contribution to a shared
// position axis: the child's own `axis` block plus a human label used
// in conflict diagnostics.
type sharedAxisBlock struct {
	// Label identifies the child in warning messages, e.g. "layer-1".
	Label string
	// Axis is the child's channel.axis block. Nil means the child
	// specified no axis config at all.
	Axis *spec.Axis
}

// labelledEncoding pairs one composition child's encoding with the
// label used to identify it in conflict diagnostics.
type labelledEncoding struct {
	Label string
	Enc   *spec.Encoding
}

// sharedAxisBlocksFrom collects the axis blocks bound to one position
// channel across composition children, in child order. Children that
// do not bind the channel contribute nothing.
func sharedAxisBlocksFrom(channel scene.Channel, children []labelledEncoding) []sharedAxisBlock {
	var out []sharedAxisBlock
	for _, c := range children {
		ch := positionChannelFor(c.Enc, channel)
		if ch == nil || ch.Axis == nil {
			continue
		}
		out = append(out, sharedAxisBlock{Label: c.Label, Axis: ch.Axis})
	}
	return out
}

// positionChannelFor returns the encoding's channel binding for a
// position channel, or nil when the channel is unbound.
func positionChannelFor(enc *spec.Encoding, channel scene.Channel) *spec.PositionChannel {
	if enc == nil {
		return nil
	}
	switch channel {
	case scene.ChannelX:
		return enc.X
	case scene.ChannelY:
		return enc.Y
	}
	return nil
}

// sharedAxisOpts resolves the AxisOpts for a *shared* position axis
// from every contributing child's `axis` block.
//
// Resolution rule (documented in docs/src/concepts/composition.md):
// **first specified wins, per property**. Each property of the axis
// block is taken from the first child, in declaration order, that
// specifies it; a later child specifying the *same* property with a
// *different* value is ignored and raises
// PRISM_WARN_AXIS_CONFIG_CONFLICT naming the channel, the property,
// and both values. Conflicts are therefore never silently resolved.
//
// fallbackTitle supplies the axis title when no child sets
// `axis.title` — it is the same field-name fallback a shared axis used
// before per-channel config was honoured. An explicit
// `"title": false` still suppresses the title.
func sharedAxisOpts(channel scene.Channel, blocks []sharedAxisBlock, fallbackTitle string) (AxisOpts, []scene.Warning) {
	merged, warnings := mergeSharedAxisSpec(channel, blocks)
	// Synthesise a position channel so the shared path resolves through
	// exactly the same reader as the flat path. Any field added to
	// axisOptsFor is honoured under shared scales for free.
	ch := &spec.PositionChannel{Axis: merged}
	ch.Field = fallbackTitle
	return axisOptsFor(ch), warnings
}

// sharedAxisOrient resolves the `orient` of a *shared* position axis
// from every contributing child's axis block, first-specified-wins —
// the same fold sharedAxisOpts applies, so the side the padding
// reserves and the side the axis renders on agree.
//
// The layout has to know the side before the axis can be built (the
// axis is anchored to the plot rect the padding produces), so this
// runs the fold early and discards its warnings; sharedAxisOpts runs
// it again at axis-build time and is the one that reports conflicts,
// which keeps PRISM_WARN_AXIS_CONFIG_CONFLICT emitted exactly once.
func sharedAxisOrient(channel scene.Channel, blocks []sharedAxisBlock) string {
	merged, _ := mergeSharedAxisSpec(channel, blocks)
	if merged == nil {
		return ""
	}
	return merged.Orient
}

// sharedAxisPlacement applies the folded orient of both position
// channels to p, leaving the hidden flags the caller already set
// untouched.
func sharedAxisPlacement(p AxisPlacement, children []labelledEncoding) AxisPlacement {
	p.X = AxisPositionFor(scene.ChannelX,
		sharedAxisOrient(scene.ChannelX, sharedAxisBlocksFrom(scene.ChannelX, children)))
	p.Y = AxisPositionFor(scene.ChannelY,
		sharedAxisOrient(scene.ChannelY, sharedAxisBlocksFrom(scene.ChannelY, children)))
	return p
}

// mergeSharedAxisSpec folds N per-child axis blocks into one, applying
// the first-specified-wins rule property by property. The merge walks
// spec.Axis reflectively so a property added to the spec type is
// covered without a second registration site; encode/axis_shared_test.go
// pins that invariant.
func mergeSharedAxisSpec(channel scene.Channel, blocks []sharedAxisBlock) (*spec.Axis, []scene.Warning) {
	var specified bool
	var warnings []scene.Warning

	out := &spec.Axis{}
	outVal := reflect.ValueOf(out).Elem()
	typ := outVal.Type()
	winners := make([]string, typ.NumField())

	for _, b := range blocks {
		if b.Axis == nil {
			continue
		}
		src := reflect.ValueOf(b.Axis).Elem()
		for i := 0; i < typ.NumField(); i++ {
			f := src.Field(i)
			if f.IsZero() {
				// Property unspecified on this child (nil pointer,
				// empty string, nil slice or nil interface).
				continue
			}
			if winners[i] == "" {
				outVal.Field(i).Set(f)
				winners[i] = b.Label
				specified = true
				continue
			}
			if reflect.DeepEqual(outVal.Field(i).Interface(), f.Interface()) {
				continue
			}
			warnings = append(warnings, axisConflictWarning(
				channel, axisPropertyName(typ.Field(i)), b.Label, winners[i],
				axisDisplayValue(outVal.Field(i)), axisDisplayValue(f)))
		}
	}
	if !specified {
		return nil, warnings
	}
	return out, warnings
}

// axisPropertyName returns the wire (snake_case) name of an axis
// property, falling back to the Go field name.
func axisPropertyName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' {
			tag = tag[:i]
			break
		}
	}
	if tag == "" || tag == "-" {
		return f.Name
	}
	return tag
}

// axisDisplayValue renders an axis property for diagnostics,
// dereferencing the optional-pointer fields so a warning reports
// `false` rather than a pointer address.
func axisDisplayValue(v reflect.Value) any {
	if v.Kind() == reflect.Pointer && !v.IsNil() {
		return v.Elem().Interface()
	}
	return v.Interface()
}

// axisConflictWarning builds the PRISM_WARN_AXIS_CONFIG_CONFLICT
// warning emitted when two children disagree on a shared axis
// property.
func axisConflictWarning(channel scene.Channel, property, loser, winner string, kept, ignored any) scene.Warning {
	return scene.Warning{
		Code:  scene.WarnAxisConfigConflict,
		Layer: loser,
		Message: fmt.Sprintf(
			"shared %s axis: %s already set %s to %v; %s requests %v and is ignored (first specified wins).",
			channel, winner, property, kept, loser, ignored),
		Details: map[string]any{
			"Channel":  string(channel),
			"Property": property,
			"Resolve":  "shared",
			"Winner":   winner,
			"Kept":     kept,
			"Ignored":  ignored,
			"Loser":    loser,
		},
	}
}
