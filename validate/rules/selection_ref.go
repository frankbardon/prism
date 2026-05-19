package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// SelectionRef implements PRISM_SPEC_004: every reference to a named
// selection must resolve to a selection declared in the spec's
// "selection" block.
//
// Selection references in v1 appear as:
//   - The substring "selection:<name>" inside a filter expression (the
//     convention adopted for Pulse-expression-driven specs).
//   - Future: explicit references in condition encodings (deferred to v2).
type SelectionRef struct{}

// Code returns PRISM_SPEC_004.
func (SelectionRef) Code() string { return "PRISM_SPEC_004" }

var selectionRefRegex = regexp.MustCompile(`(?i)selection\s*[:=]\s*([a-z_][a-z0-9_]*)`)

// Check scans filter expressions for selection references and reports any
// that do not match a declared selection.
func (SelectionRef) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	declared := map[string]bool{}
	for name := range s.Selection {
		declared[name] = true
	}

	var out []*errors.AppError
	for _, t := range s.Transform {
		if t.Filter == nil {
			continue
		}
		for _, m := range selectionRefRegex.FindAllStringSubmatch(t.Filter.Filter, -1) {
			name := m[1]
			if declared[name] {
				continue
			}
			out = append(out, errors.New("PRISM_SPEC_004",
				fmt.Sprintf("Selection reference %q in filter does not resolve to a declared selection.", name),
				map[string]any{
					"Selection": name,
					"Available": joinSortedKeys(declared),
				},
			))
		}
	}
	return out
}

func joinSortedKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
