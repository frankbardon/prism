package theme

import (
	"fmt"
	"sort"

	prismerrors "github.com/frankbardon/prism/errors"
)

// Validate checks theme-level structural invariants that JSON Schema
// shape-checking alone cannot express: every style block's Filter
// reference (Mark, per-type Marks entries, named Style entries, Axis,
// Legend, Title, View) must name a key present in t.Filters.
//
// This is an intentional departure from RangeSlot.Resolve's silent-
// fallback behavior — an unresolved Filter name fails loudly with
// PRISM_THEME_FILTER_UNKNOWN rather than silently rendering without
// the filter. Called by Register and LoadFile/LoadBytes so both the
// built-in registry and externally loaded theme JSON are covered;
// this story is model + validation only — the actual SVG <filter>
// element / filter="" attribute emission lands in E1-S2.
func (t *Theme) Validate() error {
	if t == nil {
		return nil
	}
	if err := t.checkFilterRef("mark", markStyleFilter(t.Mark)); err != nil {
		return err
	}
	for _, name := range sortedMarkStyleKeys(t.Marks) {
		if err := t.checkFilterRef("marks."+name, markStyleFilter(t.Marks[name])); err != nil {
			return err
		}
	}
	for _, name := range sortedMarkStyleKeys(t.Style) {
		if err := t.checkFilterRef("style."+name, markStyleFilter(t.Style[name])); err != nil {
			return err
		}
	}
	if t.Axis != nil {
		if err := t.checkFilterRef("axis", t.Axis.Filter); err != nil {
			return err
		}
	}
	if t.Legend != nil {
		if err := t.checkFilterRef("legend", t.Legend.Filter); err != nil {
			return err
		}
	}
	if t.Title != nil {
		if err := t.checkFilterRef("title", t.Title.Filter); err != nil {
			return err
		}
	}
	if t.View != nil {
		if err := t.checkFilterRef("view", t.View.Filter); err != nil {
			return err
		}
	}
	if err := t.validateGradients(); err != nil {
		return err
	}
	if err := t.validatePatterns(); err != nil {
		return err
	}
	return nil
}

// validateGradients checks structural sanity of every entry in
// t.Gradients. There is no cross-reference to check yet — Fill/
// Stroke/Background resolution against url(#name) lands in E3-S2, at
// which point an unresolved reference gets its own fail-loud check
// mirroring checkFilterRef.
func (t *Theme) validateGradients() error {
	for _, name := range sortedGradientNames(t.Gradients) {
		if reason := gradientDefIssue(t.Gradients[name]); reason != "" {
			return prismerrors.New(
				"PRISM_THEME_GRADIENT_INVALID",
				fmt.Sprintf("theme.gradients.%s is invalid: %s.", name, reason),
				map[string]any{"Name": name, "Reason": reason},
			)
		}
	}
	return nil
}

// gradientDefIssue returns a non-empty reason string when g fails a
// structural sanity check, or "" when it is well-formed.
func gradientDefIssue(g GradientDef) string {
	switch g.Type {
	case "linear", "radial":
	default:
		return fmt.Sprintf("type must be \"linear\" or \"radial\", got %q", g.Type)
	}
	if len(g.Stops) < 2 {
		return fmt.Sprintf("must declare at least 2 stops, got %d", len(g.Stops))
	}
	for i, s := range g.Stops {
		if s.Offset < 0 || s.Offset > 1 {
			return fmt.Sprintf("stops[%d].offset %v is out of range [0, 1]", i, s.Offset)
		}
		if s.Color == "" {
			return fmt.Sprintf("stops[%d].color is empty", i)
		}
	}
	if g.Type == "radial" && g.Radius != nil && *g.Radius <= 0 {
		return fmt.Sprintf("radius must be positive, got %v", *g.Radius)
	}
	return ""
}

// validatePatterns checks structural sanity of every entry in
// t.Patterns. There is no cross-reference to check yet — see
// validateGradients.
func (t *Theme) validatePatterns() error {
	for _, name := range sortedPatternNames(t.Patterns) {
		if reason := patternDefIssue(t.Patterns[name]); reason != "" {
			return prismerrors.New(
				"PRISM_THEME_PATTERN_INVALID",
				fmt.Sprintf("theme.patterns.%s is invalid: %s.", name, reason),
				map[string]any{"Name": name, "Reason": reason},
			)
		}
	}
	return nil
}

// patternDefIssue returns a non-empty reason string when p fails a
// structural sanity check, or "" when it is well-formed.
func patternDefIssue(p PatternDef) string {
	hasType := p.Type != ""
	hasContent := p.Content != ""
	switch {
	case hasType && hasContent:
		return `set either "type" or "content", not both`
	case !hasType && !hasContent:
		return `must set either "type" (built-in catalogue name) or "content" (raw SVG)`
	case hasType && !IsBuiltinPatternType(p.Type):
		return fmt.Sprintf("type %q is not a built-in pattern (want one of %v)", p.Type, PatternTypes)
	}
	if p.Spacing != nil && *p.Spacing <= 0 {
		return fmt.Sprintf("spacing must be positive, got %v", *p.Spacing)
	}
	if p.Size != nil && *p.Size <= 0 {
		return fmt.Sprintf("size must be positive, got %v", *p.Size)
	}
	return ""
}

func sortedGradientNames(m map[string]GradientDef) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedPatternNames(m map[string]PatternDef) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// checkFilterRef returns PRISM_THEME_FILTER_UNKNOWN when filter is
// non-empty and does not name a key in t.Filters. block is the
// dotted theme-path used in the error message (e.g. "marks.bar").
func (t *Theme) checkFilterRef(block, filter string) error {
	if filter == "" {
		return nil
	}
	if _, ok := t.Filters[filter]; ok {
		return nil
	}
	return prismerrors.New(
		"PRISM_THEME_FILTER_UNKNOWN",
		fmt.Sprintf("theme.%s.filter references undefined filter %q.", block, filter),
		map[string]any{
			"Block":     block,
			"Filter":    filter,
			"Available": sortedFilterNames(t.Filters),
		},
	)
}

func markStyleFilter(m *MarkStyle) string {
	if m == nil {
		return ""
	}
	return m.Filter
}

func sortedMarkStyleKeys(m map[string]*MarkStyle) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedFilterNames(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
