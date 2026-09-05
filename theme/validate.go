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
	return nil
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
