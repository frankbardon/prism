package encode

import (
	"strings"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// usedMarkKeys collects the theme.Marks keys a finished document
// actually needs, so the emitted CSS carries variables for those
// families alone.
//
// A theme declares a MarkStyle per mark family it knows about, and the
// variable block emitted every one of them regardless of what the
// scene drew — a bar chart shipped tokens for arc, tree, geoshape and
// a dozen more. The tokens were inert (nothing carried the matching
// class) but they were bytes in every document, and each new Marks key
// widened every golden in the repo.
//
// Keys come from the SPEC's mark names, not from scene.Mark.Type: the
// scene type is the geometry a mark draws (a bar is a "rect"), while
// theme.Marks is keyed by the mark family an author names. A
// multi-shape family that stamps scene.Mark.Class contributes that
// class too, which is how a progress track's own tokens survive.
func usedMarkKeys(doc *scene.SceneDoc, s *spec.Spec) map[string]bool {
	used := make(map[string]bool, 8)
	collectSpecMarkNames(s, used)
	if doc == nil {
		return used
	}
	for i := range doc.Grid.Cells {
		collectMarkClasses(&doc.Grid.Cells[i].Scene, used)
	}
	return used
}

// collectSpecMarkNames walks a spec and its composition children,
// recording every mark family named anywhere in the tree.
func collectSpecMarkNames(s *spec.Spec, used map[string]bool) {
	if s == nil {
		return
	}
	if s.Mark != nil && s.Mark.TypeName() != "" {
		used[s.Mark.TypeName()] = true
	}
	for _, child := range s.Layer {
		collectSpecMarkNames(child, used)
	}
	for _, child := range s.Concat {
		collectSpecMarkNames(child, used)
	}
	for _, child := range s.HConcat {
		collectSpecMarkNames(child, used)
	}
	for _, child := range s.VConcat {
		collectSpecMarkNames(child, used)
	}
	collectSpecMarkNames(s.ChildSpec, used)
}

// collectMarkClasses adds the explicit classes a multi-shape family
// stamped on its marks (progress-track), naming theme keys the spec's
// mark name alone does not reach.
func collectMarkClasses(sc *scene.Scene, used map[string]bool) {
	if sc == nil {
		return
	}
	for _, layer := range sc.Layers {
		for _, m := range layer.Marks {
			if m.Class == "" {
				continue
			}
			// The renderer spells a multi-shape class with hyphens
			// ("prism-mark-progress-track") while theme.Marks keys
			// are snake_case ("progress_track"), so record both
			// spellings rather than guessing which side to rewrite.
			key := strings.TrimPrefix(m.Class, "prism-mark-")
			used[key] = true
			used[strings.ReplaceAll(key, "-", "_")] = true
		}
	}
}

// finalizeAutoDarkCSS, which re-emits for the auto-dark registry.
func narrowDocCSS(doc *scene.SceneDoc, s *spec.Spec, opts EncodeOpts) {
	if doc == nil || doc.Theme == nil || s == nil || doc.Theme.CSS == "" {
		return
	}
	// Re-resolve rather than thread the theme through every
	// document-building path: resolveThemeFull is deterministic and
	// pure, and this runs once per encode at the top of the tree.
	_, full, err := resolveThemeFull(opts, s.Theme)
	if err != nil || full == nil || len(full.Marks) == 0 {
		return
	}
	used := usedMarkKeys(doc, s)
	drop := make([]string, 0, len(full.Marks))
	for name := range full.Marks {
		if !used[name] {
			drop = append(drop, "--prism-mark-"+name+"-")
		}
	}
	doc.Theme.CSS = dropVarDecls(doc.Theme.CSS, drop)
}

// dropVarDecls removes every `--name...:value;` declaration whose name
// starts with one of the given prefixes.
//
// It filters the emitted text rather than re-rendering the variable
// block, because the block also carries the auto-dark resolved
// mark-color vars (E4-S3) and the prefers-color-scheme chrome rule,
// neither of which the theme alone can reproduce. Values are
// theme-authored colors, lengths and dash lists — none contains a
// semicolon — so scanning to the next `;` is exact.
func dropVarDecls(css string, prefixes []string) string {
	if css == "" || len(prefixes) == 0 {
		return css
	}
	var b strings.Builder
	b.Grow(len(css))
	for i := 0; i < len(css); {
		matched := ""
		for _, p := range prefixes {
			if strings.HasPrefix(css[i:], p) {
				matched = p
				break
			}
		}
		if matched == "" {
			b.WriteByte(css[i])
			i++
			continue
		}
		end := strings.IndexByte(css[i:], ';')
		if end < 0 {
			b.WriteString(css[i:])
			break
		}
		i += end + 1
	}
	return b.String()
}
