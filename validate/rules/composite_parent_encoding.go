package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// CompositeParentEncoding implements PRISM_SPEC_054: a spec node may
// not carry an `encoding` block at the same level as a composition
// operator.
//
// Composition in Prism inherits only `data`, `datasets` and `$schema`
// from a parent down to its children (plan/build/composite.go's
// mergeParentDatasets) — `mark`, `transform`, `title` and `encoding`
// are not inherited, because each layer / panel is a self-contained
// chart. The composite encoder reads `child.Spec.Encoding` and nothing
// else, so a parent-level block is read by no code path at all: the
// rendered bytes are identical to the same spec with the block
// removed.
//
// Vega-Lite does inherit a parent encoding into layer children, which
// is exactly why the block looks like the way to share axis config
// across layers. Rejecting it is the settled choice for v1 — it stops
// the spec from lying without foreclosing real inheritance later.
type CompositeParentEncoding struct{}

// Code returns PRISM_SPEC_054.
func (CompositeParentEncoding) Code() string { return "PRISM_SPEC_054" }

// Check walks the whole spec tree — including composites nested inside
// other composites — and emits one error per offending node.
func (CompositeParentEncoding) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, b := range walkCompositeEncodings(s) {
		out = append(out, errors.New("PRISM_SPEC_054",
			fmt.Sprintf(`An "encoding" block on %s sits beside %q, where nothing reads it.`,
				b.Display(), b.Operator),
			map[string]any{
				"Path":     b.Display(),
				"Operator": b.Operator,
				"Child":    compositeChildKey(b.Operator),
			},
		))
	}
	return out
}

// compositeEncodingBinding is one spec node that pairs an `encoding`
// block with a composition operator. Path is a dotted slug naming the
// node ("" for the root, "layer[1]", "concat[0].vconcat[2]", …).
type compositeEncodingBinding struct {
	Path     string
	Operator string
}

// Display renders Path for error text, naming the root explicitly
// rather than printing an empty slug.
func (b compositeEncodingBinding) Display() string {
	if b.Path == "" {
		return "the spec root"
	}
	return b.Path
}

// walkCompositeEncodings collects every node in the spec tree that
// carries both an `encoding` block and a composition operator.
//
// The walk mirrors the child ordering the other composition-aware
// rules use (walkAxisOrients, walkSpanBindings) — layer, concat,
// hconcat, vconcat, then the single `spec` child — but recurses, so a
// `layer` nested inside a `concat` panel is reached too.
func walkCompositeEncodings(s *spec.Spec) []compositeEncodingBinding {
	if s == nil {
		return nil
	}
	var out []compositeEncodingBinding
	if s.Encoding != nil {
		if op := compositeOperator(s); op != "" {
			out = append(out, compositeEncodingBinding{Path: "", Operator: op})
		}
	}
	for _, child := range compositeChildNodes("", s) {
		for _, b := range walkCompositeEncodings(child.Spec) {
			out = append(out, compositeEncodingBinding{
				Path:     joinSpecPath(child.Path, b.Path),
				Operator: b.Operator,
			})
		}
	}
	return out
}

// compositeChildEntry is one child spec plus the path slug naming it
// relative to its parent.
type compositeChildEntry struct {
	Path string
	Spec *spec.Spec
}

// compositeChildNodes returns every child spec node reachable from s,
// each tagged with its path slug. prefix is prepended to each slug so
// callers can build absolute paths in one pass.
//
// The `spec` key — the single child of a `facet` or `repeat` parent —
// is a child like any other: its own encoding is the chart that gets
// drawn, so it is walked but never itself reported unless it is in
// turn a composite carrying an encoding.
func compositeChildNodes(prefix string, s *spec.Spec) []compositeChildEntry {
	if s == nil {
		return nil
	}
	var out []compositeChildEntry
	add := func(slug string, child *spec.Spec) {
		if child == nil {
			return
		}
		out = append(out, compositeChildEntry{Path: joinSpecPath(prefix, slug), Spec: child})
	}
	for i, c := range s.Layer {
		add(prefixf("layer[%d]", i), c)
	}
	for i, c := range s.Concat {
		add(prefixf("concat[%d]", i), c)
	}
	for i, c := range s.HConcat {
		add(prefixf("hconcat[%d]", i), c)
	}
	for i, c := range s.VConcat {
		add(prefixf("vconcat[%d]", i), c)
	}
	add("spec", s.ChildSpec)
	return out
}

// joinSpecPath concatenates two dotted path slugs, tolerating an empty
// half on either side.
func joinSpecPath(prefix, suffix string) string {
	switch {
	case prefix == "":
		return suffix
	case suffix == "":
		return prefix
	}
	return prefix + "." + suffix
}

// compositeOperator names the composition operator declared directly
// on s, or "" for a leaf. The probe order matches
// plan/build.IsComposite; the schema's top-level `oneOf` already
// guarantees at most one is present.
func compositeOperator(s *spec.Spec) string {
	if s == nil {
		return ""
	}
	switch {
	case len(s.Layer) > 0:
		return "layer"
	case len(s.Concat) > 0:
		return "concat"
	case len(s.HConcat) > 0:
		return "hconcat"
	case len(s.VConcat) > 0:
		return "vconcat"
	case s.Facet != nil:
		return "facet"
	case s.Repeat != nil:
		return "repeat"
	}
	return ""
}

// compositeChildKey names the spec key holding the children an
// encoding block should move into. `facet` and `repeat` hold their
// single child under `spec`; every other operator holds an array
// under its own name.
func compositeChildKey(operator string) string {
	switch operator {
	case "facet", "repeat":
		return "spec"
	default:
		return operator
	}
}
