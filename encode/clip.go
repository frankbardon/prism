package encode

import (
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// plotClipPrefix namespaces the generated clipPath id. A scene id is
// already unique across the grid (encode_composite / encode_facet /
// encode_repeat rename every cell), so prefixing it yields an id that
// is unique across the whole SVG document.
const plotClipPrefix = "prism-clip-"

// plotClipID is the Defs.Clips key for a scene's plot-region clip.
func plotClipID(sceneID string) string { return plotClipPrefix + sceneID }

// wantsPlotClip decides whether a scene's mark container is bounded by
// the plot rect.
//
// Default ("auto"): the clip is armed only when a position scale in
// scope pins an explicit `scale.domain`. That is the sole way a mark
// can land outside the plot rect — without an explicit domain the
// domain is derived from the very rows being drawn, so every mark sits
// inside by construction and arming a clip would only risk shaving a
// stroke or a glyph that legitimately overhangs the edge.
//
// `mark_def.clip` overrides the default in both directions: true always
// clips, false never does. Across a layer composite (one plot rect
// shared by every layer) a single `clip: true` wins; otherwise a
// `clip: false` on any layer disarms the clip.
func wantsPlotClip(s *spec.Spec) bool {
	if s == nil {
		return false
	}
	scope := clipScope(s)
	var sawFalse bool
	for _, sub := range scope {
		c := markClipFlag(sub)
		if c == nil {
			continue
		}
		if *c {
			return true
		}
		sawFalse = true
	}
	if sawFalse {
		return false
	}
	for _, sub := range scope {
		if encodingPinsPositionDomain(sub.Encoding) {
			return true
		}
	}
	return false
}

// clipScope is the set of specs that share one plot rect: the spec
// itself plus its layer children. Concat / facet / repeat children are
// NOT in scope — each of those becomes its own scene with its own plot
// rect and its own clip decision.
func clipScope(s *spec.Spec) []*spec.Spec {
	out := make([]*spec.Spec, 0, 1+len(s.Layer))
	out = append(out, s)
	for _, child := range s.Layer {
		if child != nil {
			out = append(out, child)
		}
	}
	return out
}

// markClipFlag reads the tri-state `mark_def.clip`.
func markClipFlag(s *spec.Spec) *bool {
	if s == nil || s.Mark == nil || s.Mark.Def == nil {
		return nil
	}
	return s.Mark.Def.Clip
}

// encodingPinsPositionDomain reports whether any cartesian position
// channel carries an explicit `scale.domain`. Span channels (x2 / y2)
// resolve no scale of their own, but an author may still spell the
// domain there, so they are checked too.
func encodingPinsPositionDomain(enc *spec.Encoding) bool {
	if enc == nil {
		return false
	}
	for _, ch := range []*spec.PositionChannel{enc.X, enc.Y, enc.X2, enc.Y2} {
		if ch != nil && ch.Scale != nil && ch.Scale.Domain != nil {
			return true
		}
	}
	return false
}

// armPlotClip registers the plot rect as a Defs.Clips entry and points
// the scene's mark container at it. A no-op when on is false, which is
// what keeps every scene that predates E2-S2 byte-identical.
func armPlotClip(s *scene.Scene, on bool) {
	if !on || s == nil {
		return
	}
	id := plotClipID(s.ID)
	if s.Defs == nil {
		s.Defs = &scene.Defs{}
	}
	if s.Defs.Clips == nil {
		s.Defs.Clips = map[string]scene.Rect{}
	}
	s.Defs.Clips[id] = s.Plot
	s.ClipRef = id
}

// renameScene assigns a scene id and re-keys any plot clip registered
// under the previous one, so a multi-cell grid never emits two clipPath
// elements sharing an id.
func renameScene(s *scene.Scene, id string) {
	old := s.ClipRef
	s.ID = id
	if old == "" {
		return
	}
	next := plotClipID(id)
	if s.Defs != nil && s.Defs.Clips != nil {
		if r, ok := s.Defs.Clips[old]; ok {
			delete(s.Defs.Clips, old)
			s.Defs.Clips[next] = r
		}
	}
	s.ClipRef = next
}

// offsetClips shifts every registered clip rect by (dx, dy) alongside
// the rest of the scene's absolute coordinates.
func offsetClips(s *scene.Scene, dx, dy float64) {
	if s.Defs == nil || len(s.Defs.Clips) == 0 {
		return
	}
	for id, r := range s.Defs.Clips {
		r.X += dx
		r.Y += dy
		s.Defs.Clips[id] = r
	}
}
