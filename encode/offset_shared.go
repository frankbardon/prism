package encode

import (
	"fmt"
	"reflect"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// Offset scales across composition children (E3-S1).
//
// `x_offset` / `y_offset` resolve the same way every other scale in a
// composition does: SHARED by default for the children that bind the
// channel, with an `independent` opt-out through the `resolve` block.
//
// Shared is the load-bearing default. Two layers each binding
// `x_offset` to `series` but resolving their own sub-band domains
// would divide one band slot differently — a layer that sees two
// series gives each half the slot, a layer that sees three gives each
// a third — and the bars would not line up. That reads as a rendering
// fault rather than a configuration one, which is the worst kind of
// bug to ship. It is also the rule `x` and `y` already follow, so
// offset behaving differently would itself be the surprise.
//
// What is shared is the sub-band DOMAIN — the ordered category list
// and the band-geometry knobs the slot is divided with — not a scale
// object. An offset scale's range is the parent band's own
// BandWidth(), which is a property of the position scale in the cell
// being drawn, so it can only be read per child. Sharing the domain
// is what makes the sub-bands comparable; sharing the range would be
// meaningless.
//
// The scale itself is still built in exactly one place,
// resolveOffsetBinding's NewBandScale call, whether the categories
// came from this file or from the child's own table. The flat path
// and the composite path therefore cannot drift apart on a padding
// knob.

// OffsetDomain is one shared offset resolution, handed from a
// composition parent down to the children that draw with it.
//
// Axis is the position channel ("x" or "y") whose band slot the
// offset subdivides; a child whose own binding names the other axis
// ignores the domain and resolves its own.
type OffsetDomain struct {
	// Axis is "x" or "y" — the position channel being subdivided.
	Axis string
	// Categories is the resolved sub-band order, already run through
	// the full precedence chain (scale.domain > sort naming
	// categories > sort direction > distinct values ascending).
	Categories []string
	// Opts carries the merged band-geometry knobs (padding, align,
	// reverse, round) with the offset padding defaults seeded in.
	Opts ScaleOpts
}

// offsetBlock is one composition child's contribution to a shared
// offset scale: the child's own offset channel plus a human label
// used in conflict diagnostics ("layer-1").
type offsetBlock struct {
	Label string
	Ch    *spec.OffsetChannel
}

// offsetChannelFor returns the encoding's binding for an offset
// channel, or nil when the channel is unbound.
func offsetChannelFor(enc *spec.Encoding, channel scene.Channel) *spec.OffsetChannel {
	if enc == nil {
		return nil
	}
	switch channel {
	case scene.ChannelXOffset:
		return enc.XOffset
	case scene.ChannelYOffset:
		return enc.YOffset
	}
	return nil
}

// isOffsetChannel reports whether channel names an offset sub-band
// scale rather than a position scale.
func isOffsetChannel(channel scene.Channel) bool {
	return channel == scene.ChannelXOffset || channel == scene.ChannelYOffset
}

// offsetAxisName maps an offset channel onto the position channel it
// subdivides, in the "x" / "y" spelling spec.OffsetBinding uses.
func offsetAxisName(channel scene.Channel) string {
	if channel == scene.ChannelYOffset {
		return "y"
	}
	return "x"
}

// offsetBlocksFrom collects the offset bindings on one channel across
// composition children, in child order. Children that do not bind the
// channel contribute nothing, so "first specified wins" means the
// lowest-indexed child that actually binds it.
func offsetBlocksFrom(channel scene.Channel, children []labelledEncoding) []offsetBlock {
	var out []offsetBlock
	for _, c := range children {
		ch := offsetChannelFor(c.Enc, channel)
		if ch == nil || ch.Field == "" {
			continue
		}
		out = append(out, offsetBlock{Label: c.Label, Ch: ch})
	}
	return out
}

// mergeSharedOffsetChannel folds N per-child offset blocks into one,
// applying the first-specified-wins rule property by property.
//
// This is the rule mergeSharedAxisSpec already established for a
// shared axis, and it is followed here for the same reason: a
// disagreement between children is never settled silently. The first
// child (in declaration order) to specify `sort`, or any property of
// the offset channel's own `scale` block, supplies the value; a later
// child specifying the SAME property with a DIFFERENT value is
// ignored and raises PRISM_WARN_OFFSET_CONFIG_CONFLICT naming the
// channel, the property and both values.
//
// `field` and `type` are taken from the first binding child and never
// conflict-reported. They do not decide how the slot is divided — the
// values every child contributes are unioned regardless of which
// column they came from, the same way a shared x scale unions layers
// that bind different fields — and each child still validates its own
// declared type where it resolves its own binding.
//
// The `scale` fold walks spec.Scale reflectively, so a knob added to
// the scale block is covered without a second registration site.
func mergeSharedOffsetChannel(channel scene.Channel, blocks []offsetBlock) (*spec.OffsetChannel, []scene.Warning) {
	var out *spec.OffsetChannel
	var warnings []scene.Warning

	scaleOut := &spec.Scale{}
	scaleVal := reflect.ValueOf(scaleOut).Elem()
	scaleTyp := scaleVal.Type()
	scaleWinners := make([]string, scaleTyp.NumField())
	scaleSpecified := false
	sortWinner := ""

	for _, b := range blocks {
		if b.Ch == nil || b.Ch.Field == "" {
			continue
		}
		if out == nil {
			out = &spec.OffsetChannel{Field: b.Ch.Field, Type: b.Ch.Type}
		}
		if b.Ch.Sort != nil {
			switch {
			case sortWinner == "":
				out.Sort = b.Ch.Sort
				sortWinner = b.Label
			case !reflect.DeepEqual(out.Sort, b.Ch.Sort):
				warnings = append(warnings, offsetConflictWarning(
					channel, "sort", b.Label, sortWinner, out.Sort, b.Ch.Sort))
			}
		}
		if b.Ch.Scale == nil {
			continue
		}
		src := reflect.ValueOf(b.Ch.Scale).Elem()
		for i := 0; i < scaleTyp.NumField(); i++ {
			f := src.Field(i)
			if f.IsZero() {
				// Property unspecified on this child (nil pointer,
				// empty string or nil interface).
				continue
			}
			if scaleWinners[i] == "" {
				scaleVal.Field(i).Set(f)
				scaleWinners[i] = b.Label
				scaleSpecified = true
				continue
			}
			if reflect.DeepEqual(scaleVal.Field(i).Interface(), f.Interface()) {
				continue
			}
			warnings = append(warnings, offsetConflictWarning(
				channel, specPropertyName(scaleTyp.Field(i)), b.Label, scaleWinners[i],
				specDisplayValue(scaleVal.Field(i)), specDisplayValue(f)))
		}
	}
	if out == nil {
		return nil, warnings
	}
	if scaleSpecified {
		out.Scale = scaleOut
	}
	return out, warnings
}

// offsetSharedScaleOpts is the ScaleOpts half of a shared offset
// resolution: the merged block's own band-geometry knobs with the
// offset padding defaults seeded in.
//
// It runs the fold early and DISCARDS its warnings, exactly as
// sharedAxisOrient does — sharedOffsetDomain runs the same fold when
// it resolves the categories and is the single reporter, so
// PRISM_WARN_OFFSET_CONFIG_CONFLICT is emitted once.
func offsetSharedScaleOpts(channel scene.Channel, blocks []offsetBlock) ScaleOpts {
	merged, _ := mergeSharedOffsetChannel(channel, blocks)
	if merged == nil {
		return ScaleOpts{}
	}
	return offsetScaleOpts(merged)
}

// sharedOffsetDomain resolves ONE sub-band order for every child that
// binds the channel, from the merged offset block and the union of
// the children's offset values (collected in child order).
//
// It does not route through encode/resolve.Unify the way
// resolveSharedScale does for a position channel. Unify's categorical
// arm returns the FIRST-SEEN union across layers, and an offset scale
// orders its sub-bands by the distinct values ASCENDING when no
// `sort` and no `scale.domain` say otherwise — an order that does not
// move when rows move. Running the union through Unify would hand
// offsetCategories a domain already permuted into row order and
// quietly undo that guarantee. The union here is a plain
// concatenation; offsetCategories owns the ordering, once, for the
// flat path and the shared path alike.
func sharedOffsetDomain(channel scene.Channel, blocks []offsetBlock, values []any, opts ScaleOpts) (*OffsetDomain, []scene.Warning, error) {
	merged, warnings := mergeSharedOffsetChannel(channel, blocks)
	if merged == nil || len(values) == 0 {
		return nil, warnings, nil
	}
	cats, err := offsetCategories(values, merged, opts, string(channel))
	if err != nil {
		return nil, warnings, err
	}
	return &OffsetDomain{
		Axis:       offsetAxisName(channel),
		Categories: cats,
		Opts:       opts,
	}, warnings, nil
}

// offsetConflictWarning builds the PRISM_WARN_OFFSET_CONFIG_CONFLICT
// warning emitted when two children disagree on a shared offset
// scale's configuration. Shaped to match axisConflictWarning so the
// two read identically in a warnings block.
func offsetConflictWarning(channel scene.Channel, property, loser, winner string, kept, ignored any) scene.Warning {
	return scene.Warning{
		Code:  scene.WarnOffsetConfigConflict,
		Layer: loser,
		Message: fmt.Sprintf(
			"shared %s scale: %s already set %s to %v; %s requests %v and is ignored (first specified wins).",
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
