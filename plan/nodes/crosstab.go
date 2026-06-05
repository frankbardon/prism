package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/pulse"
	"github.com/frankbardon/pulse/encoding"
	pulsetypes "github.com/frankbardon/pulse/types"
	"github.com/spf13/afero"

	"github.com/frankbardon/prism/compile"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// CrosstabNode runs a Pulse v0.13+ Request with a Crosstab section
// against the source cohort and materialises the long-form rows into
// a typed *table.Table the heatmap encoder consumes. The node is its
// own leaf — it opens the .pulse cohort directly rather than reading
// an upstream Table, since Pulse has no in-memory cohort constructor
// (see compile/inmem/backend.go § Pulse v0.8.4 note).
//
// v1 ships shape=long only. shape=matrix returns Pulse's structured
// payload on Response.Crosstab; the heatmap encoder consumes
// flat rows today, so the wire shape stays the same regardless of
// margins / normalisation.
//
// Grouper support is currently restricted to GROUP_CATEGORY (one
// Field per axis entry). Date / range / quantile groupers land
// behind a follow-up that wires their params.
type CrosstabNode struct {
	id        plan.NodeID
	ref       string
	fs        afero.Fs
	body      spec.CrosstabBody
	outSchema *encoding.Schema
	cellAs    string
}

// NewCrosstab constructs a CrosstabNode. inSchema is the source
// cohort's schema — used to compute the row / column grouper output
// types so the downstream encoder sees the right column kinds. The
// caller (plan/build) typically obtains it via SourceNode.OutputSchema().
func NewCrosstab(id plan.NodeID, ref string, fs afero.Fs, inSchema *encoding.Schema, body spec.CrosstabBody) (*CrosstabNode, error) {
	if inSchema == nil {
		return nil, fmt.Errorf("crosstab: nil input schema")
	}
	cellAs := body.Cell.As
	if cellAs == "" {
		cellAs = "_value"
	}
	out, err := deriveCrosstabSchema(inSchema, body, cellAs)
	if err != nil {
		return nil, err
	}
	return &CrosstabNode{
		id:        id,
		ref:       ref,
		fs:        fs,
		body:      body,
		outSchema: out,
		cellAs:    cellAs,
	}, nil
}

// DeriveCrosstabID hashes the source ref together with the canonical
// body shape so two equivalent crosstab nodes hash identically.
func DeriveCrosstabID(ref string, body spec.CrosstabBody) plan.NodeID {
	h := sha256.New()
	h.Write([]byte(ref))
	h.Write([]byte{0})
	h.Write([]byte(crosstabBodyKey(body)))
	return plan.NodeID("crosstab:" + hex.EncodeToString(h.Sum(nil)[:8]))
}

// ID implements plan.Node.
func (n *CrosstabNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node. Crosstab is a leaf — Pulse processes
// the cohort directly.
func (n *CrosstabNode) Inputs() []plan.NodeID { return nil }

// Schema implements plan.Node. Pre-computed at construction.
func (n *CrosstabNode) Schema(_ []*encoding.Schema) (*encoding.Schema, error) {
	return n.outSchema, nil
}

// Fingerprint implements plan.Node.
func (n *CrosstabNode) Fingerprint() string {
	return fingerprintFor("CrosstabNode", n.ref, crosstabBodyKey(n.body))
}

// Ref returns the source ref so plan-visualisation tooling can show
// the underlying cohort. Mirrors SourceNode.Ref().
func (n *CrosstabNode) Ref() string { return n.ref }

// FS returns the afero filesystem this node was constructed with.
func (n *CrosstabNode) FS() afero.Fs { return n.fs }

// Body returns the crosstab body for renderer + test inspection.
func (n *CrosstabNode) Body() spec.CrosstabBody { return n.body }

// Kind implements plan.Labeled.
func (n *CrosstabNode) Kind() string { return "CrosstabNode" }

// Summary implements plan.Labeled. "rows: a,b | cols: c | cell: sum(x)".
func (n *CrosstabNode) Summary() string {
	rows := make([]string, len(n.body.Rows))
	for i, g := range n.body.Rows {
		rows[i] = g.Field
	}
	cols := make([]string, len(n.body.Columns))
	for i, g := range n.body.Columns {
		cols[i] = g.Field
	}
	return fmt.Sprintf("rows: %s | cols: %s | cell: %s(%s)",
		strings.Join(rows, ","), strings.Join(cols, ","),
		n.body.Cell.Aggregate, n.body.Cell.Field)
}

// Execute implements plan.Node. Opens a fresh pulse.Pulse against the
// node's afero.Fs, runs Process with a Request that carries only the
// Crosstab section + Shape=long, and materialises Response.Data into
// a typed *table.Table.
func (n *CrosstabNode) Execute(ctx context.Context, _ []*table.Table) (*table.Table, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if n.fs == nil {
		return nil, fmt.Errorf("crosstab: fs is nil")
	}
	p, err := pulse.New(pulse.Options{FS: n.fs})
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse failed to open %s for crosstab: %v.", n.ref, err),
			map[string]any{"Ref": n.ref, "Reason": err.Error()},
			err,
		)
	}
	req, err := buildCrosstabRequest(n.ref, n.body, n.cellAs)
	if err != nil {
		return nil, err
	}
	resp, err := p.Process(ctx, req)
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_PLAN_CROSSTAB_PROCESS",
			fmt.Sprintf("Pulse failed to run crosstab on %s: %v.", n.ref, err),
			map[string]any{"Ref": n.ref, "Reason": err.Error()},
			err,
		)
	}
	return tableFromCrosstabResponse(resp, n.outSchema, n.id)
}

// crosstabBodyKey returns a stable canonical key for fingerprinting.
func crosstabBodyKey(b spec.CrosstabBody) string {
	var sb strings.Builder
	sb.WriteString("rows=")
	for _, g := range b.Rows {
		sb.WriteString(g.Field)
		sb.WriteByte(',')
	}
	sb.WriteString(";cols=")
	for _, g := range b.Columns {
		sb.WriteString(g.Field)
		sb.WriteByte(',')
	}
	sb.WriteString(";cell=")
	sb.WriteString(b.Cell.Aggregate)
	sb.WriteByte('(')
	sb.WriteString(b.Cell.Field)
	sb.WriteByte(')')
	if b.Margins != nil {
		fmt.Fprintf(&sb, ";margins=%v/%v/%v", b.Margins.Rows, b.Margins.Columns, b.Margins.Grand)
	}
	if b.Normalize != "" {
		sb.WriteString(";norm=")
		sb.WriteString(b.Normalize)
	}
	if b.Shape != "" {
		sb.WriteString(";shape=")
		sb.WriteString(b.Shape)
	}
	return sb.String()
}

// buildCrosstabRequest translates a spec.CrosstabBody into a fully-
// populated pulse Request. Shape is forced to "long" so the response
// rows land on Response.Data, regardless of the spec author's choice
// (the matrix shape lands behind a future encoder).
func buildCrosstabRequest(ref string, body spec.CrosstabBody, cellAs string) (*pulsetypes.Request, error) {
	rows, err := translateGroupers(body.Rows, "rows")
	if err != nil {
		return nil, err
	}
	cols, err := translateGroupers(body.Columns, "columns")
	if err != nil {
		return nil, err
	}
	cell, err := translateCrosstabCell(body.Cell, cellAs)
	if err != nil {
		return nil, err
	}
	spec := &pulsetypes.CrosstabSpec{
		Rows:    rows,
		Columns: cols,
		Cell:    cell,
		Shape:   pulsetypes.CrosstabShapeLong,
	}
	if body.Margins != nil {
		spec.Margins = pulsetypes.CrosstabMargins{
			Rows:    body.Margins.Rows,
			Columns: body.Margins.Columns,
			Grand:   body.Margins.Grand,
		}
	}
	switch body.Normalize {
	case "", "none":
		// default
	case "row":
		spec.Normalize = pulsetypes.CrosstabNormalizeRow
	case "column":
		spec.Normalize = pulsetypes.CrosstabNormalizeColumn
	case "total":
		spec.Normalize = pulsetypes.CrosstabNormalizeTotal
	default:
		return nil, prismerrors.New(
			"PRISM_SPEC_034",
			fmt.Sprintf("crosstab.normalize must be one of none/row/column/total (got %q).", body.Normalize),
			map[string]any{"Normalize": body.Normalize},
		)
	}
	return &pulsetypes.Request{
		Cohort:   &pulsetypes.Cohort{Filename: ref},
		Crosstab: spec,
	}, nil
}

// translateGroupers converts spec.CrosstabGroup entries into
// pulsetypes.Group entries. v1: GROUP_CATEGORY only (Field).
func translateGroupers(groups []spec.CrosstabGroup, axis string) ([]*pulsetypes.Group, error) {
	if len(groups) == 0 {
		return nil, prismerrors.New(
			"PRISM_SPEC_032",
			fmt.Sprintf("crosstab.%s requires at least one grouper.", axis),
			map[string]any{"Axis": axis},
		)
	}
	out := make([]*pulsetypes.Group, len(groups))
	for i, g := range groups {
		if g.Field == "" {
			return nil, prismerrors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab.%s[%d] requires a non-empty field.", axis, i),
				map[string]any{"Axis": axis, "Index": i},
			)
		}
		t := g.Type
		if t == "" {
			t = "category"
		}
		if t != "category" {
			return nil, prismerrors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab grouper type %q not yet supported (v1: category only).", t),
				map[string]any{"Axis": axis, "Type": t},
			)
		}
		out[i] = &pulsetypes.Group{Type: pulsetypes.GROUP_CATEGORY, Field: g.Field}
	}
	return out, nil
}

// translateCrosstabCell converts a spec.CrosstabCell into the Pulse
// Aggregation. Reuses compile.AliasToPulse so the friendly alias set
// is identical to other aggregate transforms.
func translateCrosstabCell(cell spec.CrosstabCell, cellAs string) (*pulsetypes.Aggregation, error) {
	if cell.Aggregate == "" {
		return nil, prismerrors.New(
			"PRISM_SPEC_032",
			"crosstab.cell.aggregate is required.",
			map[string]any{},
		)
	}
	mapping, ok := compile.AliasToPulse[cell.Aggregate]
	if !ok || mapping.Type == "" {
		return nil, prismerrors.New(
			"PRISM_SPEC_032",
			fmt.Sprintf("crosstab.cell.aggregate %q is not a Pulse-backed alias.", cell.Aggregate),
			map[string]any{"Aggregate": cell.Aggregate, "Known": sortedAliases()},
		)
	}
	if cell.Field == "" && cell.Aggregate != "count" {
		return nil, prismerrors.New(
			"PRISM_SPEC_032",
			fmt.Sprintf("crosstab.cell.field is required for aggregate %q.", cell.Aggregate),
			map[string]any{"Aggregate": cell.Aggregate},
		)
	}
	return &pulsetypes.Aggregation{
		Type:   mapping.Type,
		Field:  cell.Field,
		Label:  cellAs,
		Params: mapping.Params,
	}, nil
}

func sortedAliases() []string {
	out := make([]string, 0, len(compile.AliasToPulse))
	for k := range compile.AliasToPulse {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// deriveCrosstabSchema builds the output schema for shape=long. Row
// + column grouper fields keep their source types; the cell column
// is F64; a "_margin" string column is added when any margin is
// enabled (Pulse stamps "rows" / "columns" / "grand" / "" per row).
func deriveCrosstabSchema(in *encoding.Schema, body spec.CrosstabBody, cellAs string) (*encoding.Schema, error) {
	srcByName := map[string]*encoding.Field{}
	for i := range in.Fields {
		srcByName[in.Fields[i].Name] = &in.Fields[i]
	}
	fields := make([]encoding.Field, 0, len(body.Rows)+len(body.Columns)+2)
	for _, g := range body.Rows {
		f, ok := srcByName[g.Field]
		if !ok {
			return nil, prismerrors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab.rows references unknown field %q.", g.Field),
				map[string]any{"Field": g.Field},
			)
		}
		fields = append(fields, encoding.Field{Name: g.Field, Type: f.Type})
	}
	for _, g := range body.Columns {
		f, ok := srcByName[g.Field]
		if !ok {
			return nil, prismerrors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab.columns references unknown field %q.", g.Field),
				map[string]any{"Field": g.Field},
			)
		}
		fields = append(fields, encoding.Field{Name: g.Field, Type: f.Type})
	}
	fields = append(fields, encoding.Field{Name: cellAs, Type: aggregateOutputType(body.Cell.Aggregate)})
	if body.Margins != nil && (body.Margins.Rows || body.Margins.Columns || body.Margins.Grand) {
		fields = append(fields, encoding.Field{Name: "_margin", Type: encoding.FieldTypeCategoricalU32})
	}
	return &encoding.Schema{Fields: fields}, nil
}

// tableFromCrosstabResponse materialises Response.Data into a typed
// *table.Table. Reuses the same kind switch as the chain node, with
// content hash derived from the node id (the crosstab node fingerprint
// is content-addressed by the request body already).
func tableFromCrosstabResponse(resp *pulsetypes.Response, schema *encoding.Schema, id plan.NodeID) (*table.Table, error) {
	if schema == nil {
		return nil, fmt.Errorf("crosstab: nil output schema")
	}
	rows := resp.Data
	cols := make(map[string]table.Column, len(schema.Fields))
	for i := range schema.Fields {
		f := &schema.Fields[i]
		switch table.KindFromPulseFieldType(f.Type) {
		case table.KindString:
			cols[f.Name] = make(table.StringColumn, 0, len(rows))
		case table.KindFloat:
			cols[f.Name] = make(table.FloatColumn, 0, len(rows))
		case table.KindInt:
			cols[f.Name] = make(table.IntColumn, 0, len(rows))
		case table.KindBool:
			cols[f.Name] = make(table.BoolColumn, 0, len(rows))
		case table.KindDate:
			cols[f.Name] = make(table.DateColumn, 0, len(rows))
		default:
			return nil, fmt.Errorf("crosstab: unsupported field type %s for %q", f.Type, f.Name)
		}
	}
	for _, row := range rows {
		for i := range schema.Fields {
			f := &schema.Fields[i]
			raw, present := row[f.Name]
			switch table.KindFromPulseFieldType(f.Type) {
			case table.KindString:
				s := ""
				if present && raw != nil {
					s = coerceString(raw)
				}
				cols[f.Name] = append(cols[f.Name].(table.StringColumn), s)
			case table.KindFloat:
				v := 0.0
				if present && raw != nil {
					v, _ = coerceFloatRow(raw)
				}
				cols[f.Name] = append(cols[f.Name].(table.FloatColumn), v)
			case table.KindInt:
				v := int64(0)
				if present && raw != nil {
					if f64, ok := coerceFloatRow(raw); ok {
						v = int64(f64)
					}
				}
				cols[f.Name] = append(cols[f.Name].(table.IntColumn), v)
			case table.KindBool:
				b := false
				if present && raw != nil {
					b, _ = raw.(bool)
				}
				cols[f.Name] = append(cols[f.Name].(table.BoolColumn), b)
			case table.KindDate:
				v := int64(0)
				if present && raw != nil {
					if f64, ok := coerceFloatRow(raw); ok {
						v = int64(f64)
					}
				}
				cols[f.Name] = append(cols[f.Name].(table.DateColumn), v)
			}
		}
	}
	hash := "crosstab:" + string(id)
	return table.NewTable(schema, cols, len(rows), hash)
}
