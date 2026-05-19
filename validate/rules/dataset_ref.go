package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// DatasetRef implements PRISM_SPEC_005: every named dataset reference
// (in data.name, transform.data, transform.join.with, transform.union)
// must resolve to a dataset declared in the spec's "datasets" map, in
// the registered SchemaLookup, or in an earlier transform's "as" output.
type DatasetRef struct{}

// Code returns PRISM_SPEC_005.
func (DatasetRef) Code() string { return "PRISM_SPEC_005" }

// Check walks the spec collecting dataset references and reports any that
// do not resolve. Recurses into child layer/concat/facet/repeat specs.
func (DatasetRef) Check(s *spec.Spec, schemas validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	declared := collectDeclaredDatasets(s, schemas)
	return checkSpec(s, declared)
}

func checkSpec(s *spec.Spec, declared map[string]bool) []*errors.AppError {
	if s == nil {
		return nil
	}
	// Local "as" outputs become available datasets for later transforms.
	available := copyStringSet(declared)

	var out []*errors.AppError
	emit := func(name, site string) {
		out = append(out, errors.New("PRISM_SPEC_005",
			fmt.Sprintf("Dataset reference %q (%s) does not resolve to a declared dataset.", name, site),
			map[string]any{
				"Dataset":   name,
				"Site":      site,
				"Available": joinSortedKeys(available),
			},
		))
	}

	if s.Data != nil && s.Data.Name != "" && !available[s.Data.Name] {
		// data.name must be declared in datasets or registered externally.
		emit(s.Data.Name, "data.name")
	}
	// After checking, treat any spec-bound name as available downstream.
	if s.Data != nil && s.Data.Name != "" {
		available[s.Data.Name] = true
	}

	for i, t := range s.Transform {
		site := fmt.Sprintf("transform[%d]", i)
		var alias string
		switch {
		case t.Filter != nil:
			alias = t.Filter.Data
		case t.Calculate != nil:
			alias = t.Calculate.Data
		case t.Aggregate != nil:
			alias = t.Aggregate.Data
		case t.Bin != nil:
			alias = t.Bin.Data
		case t.Window != nil:
			alias = t.Window.Data
		case t.Join != nil:
			alias = t.Join.Data
			if t.Join.With != "" && !available[t.Join.With] {
				emit(t.Join.With, site+".with")
			}
		case t.Union != nil:
			alias = t.Union.Data
			for _, u := range t.Union.Union {
				if !available[u] {
					emit(u, site+".union")
				}
			}
		case t.Pivot != nil:
			alias = t.Pivot.Data
		case t.Unpivot != nil:
			alias = t.Unpivot.Data
		case t.Sample != nil:
			alias = t.Sample.Data
		case t.Sort != nil:
			alias = t.Sort.Data
		case t.Limit != nil:
			alias = t.Limit.Data
		}
		if alias != "" && !available[alias] {
			emit(alias, site+".data")
		}

		// Add this transform's "as" output for subsequent transforms.
		if as := transformAs(t); as != "" {
			available[as] = true
		}
	}

	for _, child := range s.Layer {
		out = append(out, checkSpec(child, declared)...)
	}
	for _, child := range s.Concat {
		out = append(out, checkSpec(child, declared)...)
	}
	for _, child := range s.HConcat {
		out = append(out, checkSpec(child, declared)...)
	}
	for _, child := range s.VConcat {
		out = append(out, checkSpec(child, declared)...)
	}
	if s.ChildSpec != nil {
		out = append(out, checkSpec(s.ChildSpec, declared)...)
	}
	return out
}

func transformAs(t spec.Transform) string {
	switch {
	case t.Filter != nil:
		return t.Filter.As
	case t.Calculate != nil:
		// Calculate.As is a column, not a dataset alias.
		return ""
	case t.Aggregate != nil:
		return t.Aggregate.As
	case t.Bin != nil:
		// Bin.As is a column.
		return ""
	case t.Window != nil:
		return t.Window.As
	case t.Join != nil:
		return t.Join.As
	case t.Union != nil:
		return t.Union.As
	case t.Pivot != nil:
		return t.Pivot.As
	case t.Sample != nil:
		return t.Sample.As
	case t.Sort != nil:
		return t.Sort.As
	case t.Limit != nil:
		return t.Limit.As
	}
	return ""
}

func collectDeclaredDatasets(s *spec.Spec, schemas validate.SchemaLookup) map[string]bool {
	out := map[string]bool{}
	walk(s, func(n *spec.Spec) {
		for name, ds := range n.Datasets {
			out[name] = true
			if ds != nil && ds.Name != "" {
				out[ds.Name] = true
			}
		}
	})
	// Any dataset registered externally via SchemaLookup also counts.
	// We have no enumerate-all API on SchemaLookup yet, so this is a
	// no-op for EmptyLookup; StaticLookup tests register the same names
	// in both datasets/spec.Selection and the lookup.
	if static, ok := schemas.(*validate.StaticLookup); ok && static != nil {
		for name := range static.Schemas {
			out[name] = true
		}
	}
	return out
}

func walk(s *spec.Spec, fn func(*spec.Spec)) {
	if s == nil {
		return
	}
	fn(s)
	for _, c := range s.Layer {
		walk(c, fn)
	}
	for _, c := range s.Concat {
		walk(c, fn)
	}
	for _, c := range s.HConcat {
		walk(c, fn)
	}
	for _, c := range s.VConcat {
		walk(c, fn)
	}
	walk(s.ChildSpec, fn)
}

func copyStringSet(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Ensure sort import is used (the joinSortedKeys helper lives in
// selection_ref.go, but if it ever moves we want to keep this file
// self-sufficient under build tools that don't yet load other files).
var _ = sort.Strings
var _ = strings.Join
