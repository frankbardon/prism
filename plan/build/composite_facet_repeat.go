package build

import (
	"fmt"
	"sort"
	"strings"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
)

// buildFacetComposite builds the shared-upstream sub-DAG for a
// faceted spec (D054). The returned CompositeDAG carries exactly
// one ChildDAG — the parent's source / transform pipeline up to the
// facet boundary. The encoder partitions the resulting Table by
// `(row_value, col_value)` tuples and emits one SceneCell per
// partition; the builder does not yet know how many partitions
// exist (that depends on the data).
//
// Rows / Cols on the returned CompositeDAG are placeholder zeros;
// the encoder fills them in once it has run the partition step.
func buildFacetComposite(parent *spec.Spec, opts Options) (*plan.CompositeDAG, error) {
	if parent.ChildSpec == nil {
		return nil, prismerrors.New(
			"PRISM_PLAN_002",
			"Facet block declares no child spec (`spec` is missing).",
			map[string]any{"Kind": "composition:facet-missing-spec", "Phase": "P09"},
		)
	}
	if parent.Facet.Row == nil && parent.Facet.Column == nil {
		return nil, prismerrors.New(
			"PRISM_PLAN_002",
			"Facet block declares neither row nor column channel.",
			map[string]any{"Kind": "composition:facet-empty", "Phase": "P09"},
		)
	}

	// Build the shared upstream: take the parent spec but strip the
	// facet + child spec so Build sees a plain "datasets + transform"
	// pipeline. The leaf encoding lives on the child spec and runs
	// per-partition at encode time, so we do not run it here.
	upstream := *parent
	upstream.Facet = nil
	upstream.ChildSpec = nil
	upstream.Mark = nil
	upstream.Encoding = nil
	upstream.Title = nil

	dag, tip, err := Build(&upstream, opts)
	if err != nil {
		return nil, err
	}

	// The encoder needs the child spec carried verbatim so it can
	// dispatch the per-cell encode (flat or recursive composite).
	// Merge parent datasets / data into the child for downstream
	// data-resolution symmetry with concat / layer.
	mergedChild := mergeParentDatasets(parent, parent.ChildSpec)

	out := &plan.CompositeDAG{
		Kind:    plan.CompositeFacet,
		Rows:    0, // encoder fills in
		Cols:    0, // encoder fills in
		Resolve: parent.Resolve,
		Children: []plan.ChildDAG{
			{DAG: dag, Tip: tip, Spec: mergedChild},
		},
	}
	return out, nil
}

// buildRepeatComposite builds one sub-DAG per repeat cell after
// applying the field-name substitution to the child spec (D055 /
// D056). Row-major ordering: outer loop on row, inner on column.
// Pure-row repeat → 1 column dimension; pure-col repeat → 1 row
// dimension.
func buildRepeatComposite(parent *spec.Spec, opts Options) (*plan.CompositeDAG, error) {
	if parent.ChildSpec == nil {
		return nil, prismerrors.New(
			"PRISM_PLAN_002",
			"Repeat block declares no child spec (`spec` is missing).",
			map[string]any{"Kind": "composition:repeat-missing-spec", "Phase": "P09"},
		)
	}
	if len(parent.Repeat.Row) == 0 && len(parent.Repeat.Column) == 0 {
		return nil, prismerrors.New(
			"PRISM_PLAN_002",
			"Repeat block declares neither row nor column field list.",
			map[string]any{"Kind": "composition:repeat-empty", "Phase": "P09"},
		)
	}
	// Reject `layer` form of repeat (overlay-style); v1 supports only
	// row + column.
	if len(parent.Repeat.Layer) > 0 {
		return nil, prismerrors.New(
			"PRISM_PLAN_002",
			"Repeat.layer (overlay) is not supported in v1; use repeat.row / repeat.column.",
			map[string]any{"Kind": "composition:repeat-layer", "Phase": "P10"},
		)
	}

	rows := parent.Repeat.Row
	cols := parent.Repeat.Column
	if len(rows) == 0 {
		rows = []string{""} // single-row scaffold
	}
	if len(cols) == 0 {
		cols = []string{""} // single-col scaffold
	}

	out := &plan.CompositeDAG{
		Kind:    plan.CompositeRepeat,
		Rows:    len(rows),
		Cols:    len(cols),
		Resolve: parent.Resolve,
	}

	declared := repeatAxesDeclared(parent.Repeat)

	for _, rowField := range rows {
		for _, colField := range cols {
			bindings := map[string]string{}
			if rowField != "" {
				bindings["row"] = rowField
			}
			if colField != "" {
				bindings["column"] = colField
			}

			substituted, err := applyRepeatSubstitution(parent.ChildSpec, bindings, declared)
			if err != nil {
				return nil, err
			}
			merged := mergeParentDatasets(parent, substituted)

			dag, tip, err := Build(merged, opts)
			if err != nil {
				return nil, err
			}
			out.Children = append(out.Children, plan.ChildDAG{
				DAG:  dag,
				Tip:  tip,
				Spec: merged,
			})
		}
	}
	return out, nil
}

// repeatAxesDeclared returns the sorted list of axes the parent
// repeat block declares ("row", "column"). Used for PRISM_SPEC_012's
// Declared context.
func repeatAxesDeclared(r *spec.Repeat) []string {
	var out []string
	if len(r.Row) > 0 {
		out = append(out, "row")
	}
	if len(r.Column) > 0 {
		out = append(out, "column")
	}
	return out
}

// applyRepeatSubstitution deep-clones child and walks its encoding
// channels rewriting any FieldRef whose axis is bound in `bindings`
// into a bare Field. Unknown axes raise PRISM_SPEC_012. Transform
// chains are scanned for forward-looking substitution forms and
// rejected per D055 with PRISM_PLAN_002.
func applyRepeatSubstitution(child *spec.Spec, bindings map[string]string, declared []string) (*spec.Spec, error) {
	out := deepCloneSpec(child)
	if out.Encoding != nil {
		if err := substituteEncoding(out.Encoding, bindings, declared); err != nil {
			return nil, err
		}
	}
	// Recursive: if the child is itself a composite carrying its own
	// layers / panels / inner spec, descend so substitutions inside
	// nested specs also resolve.
	for _, layer := range out.Layer {
		if layer != nil && layer.Encoding != nil {
			if err := substituteEncoding(layer.Encoding, bindings, declared); err != nil {
				return nil, err
			}
		}
	}
	for _, panel := range out.HConcat {
		if panel != nil && panel.Encoding != nil {
			if err := substituteEncoding(panel.Encoding, bindings, declared); err != nil {
				return nil, err
			}
		}
	}
	for _, panel := range out.VConcat {
		if panel != nil && panel.Encoding != nil {
			if err := substituteEncoding(panel.Encoding, bindings, declared); err != nil {
				return nil, err
			}
		}
	}
	for _, panel := range out.Concat {
		if panel != nil && panel.Encoding != nil {
			if err := substituteEncoding(panel.Encoding, bindings, declared); err != nil {
				return nil, err
			}
		}
	}
	if out.ChildSpec != nil && out.ChildSpec.Encoding != nil {
		if err := substituteEncoding(out.ChildSpec.Encoding, bindings, declared); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// substituteEncoding rewrites every channel on enc whose FieldRef is
// bound in bindings into the bare-field form. Channels referencing
// an undeclared axis raise PRISM_SPEC_012.
func substituteEncoding(enc *spec.Encoding, bindings map[string]string, declared []string) error {
	positions := []*spec.PositionChannel{
		enc.X, enc.Y, enc.X2, enc.Y2, enc.Theta, enc.Radius,
	}
	for _, ch := range positions {
		if ch == nil {
			continue
		}
		if err := substitutePositionChannel(ch, bindings, declared); err != nil {
			return err
		}
	}
	marks := []*spec.MarkChannel{
		enc.Color, enc.Fill, enc.Stroke, enc.Opacity, enc.Size, enc.Shape,
	}
	for _, ch := range marks {
		if ch == nil {
			continue
		}
		if err := substituteMarkChannel(ch, bindings, declared); err != nil {
			return err
		}
	}
	return nil
}

func substitutePositionChannel(ch *spec.PositionChannel, bindings map[string]string, declared []string) error {
	if ch.FieldRef == nil {
		return nil
	}
	resolved, err := resolveRepeatRef(ch.FieldRef, bindings, declared)
	if err != nil {
		return err
	}
	ch.Field = resolved
	ch.FieldRef = nil
	return nil
}

func substituteMarkChannel(ch *spec.MarkChannel, bindings map[string]string, declared []string) error {
	if ch.FieldRef == nil {
		return nil
	}
	resolved, err := resolveRepeatRef(ch.FieldRef, bindings, declared)
	if err != nil {
		return err
	}
	ch.Field = resolved
	ch.FieldRef = nil
	return nil
}

func resolveRepeatRef(ref *spec.RepeatRef, bindings map[string]string, declared []string) (string, error) {
	value, ok := bindings[ref.Axis]
	if !ok {
		sort.Strings(declared)
		declaredStr := strings.Join(declared, ", ")
		if declaredStr == "" {
			declaredStr = "(none)"
		}
		return "", prismerrors.New(
			"PRISM_SPEC_012",
			fmt.Sprintf("Repeat substitution {repeat: %q} references axis %q but the parent repeat block declares only %s.",
				ref.Axis, ref.Axis, declaredStr),
			map[string]any{
				"Ref":      fmt.Sprintf("{repeat: %q}", ref.Axis),
				"Axis":     ref.Axis,
				"Declared": declaredStr,
			},
		)
	}
	return value, nil
}

// deepCloneSpec produces a deep copy of s suitable for per-cell
// mutation. Channel slices + pointers are reallocated; immutable
// leaves (strings, ints) are copied by value via struct assignment.
func deepCloneSpec(s *spec.Spec) *spec.Spec {
	if s == nil {
		return nil
	}
	out := *s
	if s.Encoding != nil {
		out.Encoding = cloneEncoding(s.Encoding)
	}
	if s.ChildSpec != nil {
		out.ChildSpec = deepCloneSpec(s.ChildSpec)
	}
	if len(s.Layer) > 0 {
		out.Layer = make([]*spec.Spec, len(s.Layer))
		for i, c := range s.Layer {
			out.Layer[i] = deepCloneSpec(c)
		}
	}
	if len(s.HConcat) > 0 {
		out.HConcat = make([]*spec.Spec, len(s.HConcat))
		for i, c := range s.HConcat {
			out.HConcat[i] = deepCloneSpec(c)
		}
	}
	if len(s.VConcat) > 0 {
		out.VConcat = make([]*spec.Spec, len(s.VConcat))
		for i, c := range s.VConcat {
			out.VConcat[i] = deepCloneSpec(c)
		}
	}
	if len(s.Concat) > 0 {
		out.Concat = make([]*spec.Spec, len(s.Concat))
		for i, c := range s.Concat {
			out.Concat[i] = deepCloneSpec(c)
		}
	}
	return &out
}

func cloneEncoding(enc *spec.Encoding) *spec.Encoding {
	if enc == nil {
		return nil
	}
	out := *enc
	out.X = cloneposition(enc.X)
	out.Y = cloneposition(enc.Y)
	out.X2 = cloneposition(enc.X2)
	out.Y2 = cloneposition(enc.Y2)
	out.Theta = cloneposition(enc.Theta)
	out.Radius = cloneposition(enc.Radius)
	out.Color = cloneMarkChannel(enc.Color)
	out.Fill = cloneMarkChannel(enc.Fill)
	out.Stroke = cloneMarkChannel(enc.Stroke)
	out.Opacity = cloneMarkChannel(enc.Opacity)
	out.Size = cloneMarkChannel(enc.Size)
	out.Shape = cloneMarkChannel(enc.Shape)
	return &out
}

func cloneposition(ch *spec.PositionChannel) *spec.PositionChannel {
	if ch == nil {
		return nil
	}
	out := *ch
	if ch.FieldRef != nil {
		ref := *ch.FieldRef
		out.FieldRef = &ref
	}
	return &out
}

func cloneMarkChannel(ch *spec.MarkChannel) *spec.MarkChannel {
	if ch == nil {
		return nil
	}
	out := *ch
	if ch.FieldRef != nil {
		ref := *ch.FieldRef
		out.FieldRef = &ref
	}
	return &out
}
