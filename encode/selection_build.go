package encode

import (
	"sort"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// BuildSelections turns a spec.Selection map into post-resolve
// scene.Selection entries (D004 wiring; full hit-testing lands on the
// browser side per D077/D078). Returns nil when specSel is empty so
// SceneDoc round-trips without an empty `selections: []` artifact.
//
// Iteration order is sorted-by-selection-id for stable JSON output
// (cross-impl parity per D076 + IR contract per D011).
//
// Default Reactive mode is "client" (per D004 / D078): the JS port
// applies prism-selected / prism-deselected CSS classes without a
// server round-trip. Server-reactive mode is opt-in once the spec
// schema gains a `reactive` knob (deferred to P14 alongside the
// hardened /prism/scene endpoint per D081).
//
// Default On event:
//   - point     → click (default), hover (when p.On == "hover"),
//                 dblclick (when p.On == "dblclick").
//   - interval  → brush.
//
// Channels for interval selections derive from i.Encodings via
// channelFor — the validator (PRISM_SPEC_020) ensures interval
// encodings only list position channels.
func BuildSelections(specSel map[string]spec.Selection) []scene.Selection {
	if len(specSel) == 0 {
		return nil
	}
	names := make([]string, 0, len(specSel))
	for name := range specSel {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]scene.Selection, 0, len(names))
	for _, name := range names {
		sel := specSel[name]
		switch {
		case sel.Point != nil:
			out = append(out, scene.Selection{
				ID:        name,
				Kind:      scene.SelectionPoint,
				On:        pointEvent(sel.Point.On),
				Resolve:   scene.ResolveGlobal,
				Encodings: append([]string(nil), sel.Point.Encodings...),
				Reactive:  scene.ReactiveClient,
			})
		case sel.Interval != nil:
			out = append(out, scene.Selection{
				ID:        name,
				Kind:      scene.SelectionInterval,
				Channels:  channelsFromEncodings(sel.Interval.Encodings),
				On:        scene.EventBrush,
				Resolve:   scene.ResolveGlobal,
				Encodings: append([]string(nil), sel.Interval.Encodings...),
				Reactive:  scene.ReactiveClient,
			})
		}
	}
	return out
}

// pointEvent maps the spec-side On string to the scene event enum.
// Empty / unknown → click (the documented default per D004).
func pointEvent(on string) scene.SelectionEvent {
	switch on {
	case "hover":
		return scene.EventHover
	case "dblclick":
		return scene.EventDblclick
	default:
		return scene.EventClick
	}
}

// channelsFromEncodings turns the string encoding-channel list ("x",
// "y", "x2", "y2") into the typed scene.Channel enum. Unknown entries
// are skipped — the validator (PRISM_SPEC_020) rejects them upstream
// so this fallback is defensive only.
func channelsFromEncodings(encs []string) []scene.Channel {
	if len(encs) == 0 {
		return nil
	}
	out := make([]scene.Channel, 0, len(encs))
	for _, e := range encs {
		switch e {
		case "x":
			out = append(out, scene.ChannelX)
		case "y":
			out = append(out, scene.ChannelY)
		case "x2":
			out = append(out, scene.ChannelX2)
		case "y2":
			out = append(out, scene.ChannelY2)
		}
	}
	return out
}
