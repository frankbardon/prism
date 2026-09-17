package encode

import (
	"strings"

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

// renameScene assigns a scene id and re-keys anything registered in
// Defs under the previous one — the plot clip, and a gradient
// legend's <linearGradient> — so a multi-cell grid never emits two
// elements sharing an id.
func renameScene(s *scene.Scene, id string) {
	old := s.ID
	s.ID = id
	renameSceneGradients(s, old, id)
	oldClip := s.ClipRef
	if oldClip == "" {
		return
	}
	next := plotClipID(id)
	if s.Defs != nil && s.Defs.Clips != nil {
		if r, ok := s.Defs.Clips[oldClip]; ok {
			delete(s.Defs.Clips, oldClip)
			s.Defs.Clips[next] = r
		}
	}
	s.ClipRef = next
}

// renameSceneGradients moves every legend gradient filed under the
// old scene id onto the new one, rewriting the swatch references that
// point at it. The match is on the scene-qualified prefix, so a
// layered scene's per-layer suffix ("…-layer-2") travels with it. A
// scene with no gradient legend is untouched.
func renameSceneGradients(s *scene.Scene, oldID, newID string) {
	if oldID == newID || s.Defs == nil || len(s.Defs.Gradients) == 0 {
		return
	}
	for i := range s.Legends {
		ch := s.Legends[i].Channel
		oldPrefix, newPrefix := LegendGradientID(oldID, ch), LegendGradientID(newID, ch)
		for j := range s.Legends[i].Entries {
			ref := s.Legends[i].Entries[j].Swatch.GradientID
			if !strings.HasPrefix(ref, oldPrefix) {
				continue
			}
			next := newPrefix + strings.TrimPrefix(ref, oldPrefix)
			if g, ok := s.Defs.Gradients[ref]; ok {
				delete(s.Defs.Gradients, ref)
				s.Defs.Gradients[next] = g
			}
			s.Legends[i].Entries[j].Swatch.GradientID = next
		}
	}
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
