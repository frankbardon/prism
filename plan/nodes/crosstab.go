package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
// Grouper support: GROUP_CATEGORY (one Field per axis entry) and
// GROUP_DATE (type "date", calendar bucketing by period — bucket keys
// are string labels). Range / rounded / quantile groupers land behind
// a follow-up that wires their Interval params.
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
	// Overlay mode runs in matrix shape: build the long rows from the
	// coordinate-aligned base + overlay matrices. The plain path reads
	// Pulse's long-form Response.Data directly.
	rows := resp.Data
	if len(n.body.Overlays) > 0 {
		rows, err = longRowsFromMatrix(resp, n.body)
		if err != nil {
			return nil, err
		}
	}
	return tableFromLongRows(rows, n.outSchema, n.id)
}

// crosstabBodyKey returns a stable canonical key for fingerprinting.
func crosstabBodyKey(b spec.CrosstabBody) string {
	var sb strings.Builder
	sb.WriteString("rows=")
	for _, g := range b.Rows {
		writeGroupKey(&sb, g)
	}
	sb.WriteString(";cols=")
	for _, g := range b.Columns {
		writeGroupKey(&sb, g)
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
	for _, o := range b.Overlays {
		fmt.Fprintf(&sb, ";overlay=%s/%s/%s", o.Kind, o.Axis, overlayColumnName(o))
	}
	return sb.String()
}

// writeGroupKey appends a stable canonical encoding of one grouper
// (field + type + period) to the fingerprint builder, so two crosstabs
// that differ only in grouper kind / period hash differently.
func writeGroupKey(sb *strings.Builder, g spec.CrosstabGroup) {
	sb.WriteString(g.Field)
	if g.Type != "" {
		sb.WriteByte(':')
		sb.WriteString(g.Type)
	}
	if g.Period != "" {
		sb.WriteByte('/')
		sb.WriteString(g.Period)
	}
	sb.WriteByte(',')
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
	overlays, overlayMargins, err := buildOverlays(body.Overlays)
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
	// Overlays decorate the dense cell grid, so run the base crosstab in
	// matrix shape and force the margins the overlay handlers need as
	// denominators. The overlay/base matrices are coordinate-aligned, so
	// materialisation reads cell [r][c] from each. Margin slots live off
	// the Cells grid and never reach the long output here.
	if len(overlays) > 0 {
		spec.Shape = pulsetypes.CrosstabShapeMatrix
		spec.Margins.Rows = spec.Margins.Rows || overlayMargins.Rows
		spec.Margins.Columns = spec.Margins.Columns || overlayMargins.Columns
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
	req := &pulsetypes.Request{
		Cohort:   &pulsetypes.Cohort{Filename: ref},
		Crosstab: spec,
	}
	for _, o := range overlays {
		req.Overlays = append(req.Overlays, o.spec)
	}
	return req, nil
}

// overlayKindMapping resolves a friendly crosstab overlay name to its
// Pulse OverlayKind and the margin axis it needs as a denominator.
// Limited to cell-scoped, matrix-payload kinds that align one-to-one
// with heatmap cells.
type overlayKindMapping struct {
	kind pulsetypes.OverlayKind
	// fixedAxis is the margin axis baked into the kind name
	// (share_of_row → row, share_of_col → column). Empty when the kind
	// takes a user-supplied axis (index_vs_margin).
	fixedAxis pulsetypes.MarginAxis
	// userAxis is true when the kind requires the spec author to supply
	// an axis (index_vs_margin).
	userAxis bool
}

// All three supported kinds are cell-scoped over a matrix host and need
// an explicit Ref.Margin denominator; the forced margins are an internal
// request detail (margin rows never reach the long output).
var crosstabOverlayKinds = map[string]overlayKindMapping{
	"share_of_row":    {kind: pulsetypes.OverlayKindShareOfRow, fixedAxis: pulsetypes.MarginAxisRow},
	"share_of_col":    {kind: pulsetypes.OverlayKindShareOfCol, fixedAxis: pulsetypes.MarginAxisColumn},
	"index_vs_margin": {kind: pulsetypes.OverlayKindIndexVsMargin, userAxis: true},
	// zscore_vs_margin emits a per-cell standardized z-score against the
	// chosen axis margin — a single-request significance proxy (|z| >
	// 1.96 ≈ p < .05) for opacity shading on a heatmap.
	"zscore_vs_margin": {kind: pulsetypes.OverlayKindZScoreVsMargin, userAxis: true},
}

// builtOverlay carries one translated overlay plus its output column.
type builtOverlay struct {
	spec pulsetypes.OverlaySpec
	as   string
}

// overlayColumnName returns the output column name for an overlay,
// defaulting to the kind when As is empty.
func overlayColumnName(o spec.CrosstabOverlay) string {
	if o.As != "" {
		return o.As
	}
	return o.Kind
}

// buildOverlays translates spec overlays into Pulse OverlaySpecs and
// reports the margins they collectively require. Returns nil when there
// are no overlays.
func buildOverlays(overlays []spec.CrosstabOverlay) ([]builtOverlay, pulsetypes.CrosstabMargins, error) {
	var margins pulsetypes.CrosstabMargins
	if len(overlays) == 0 {
		return nil, margins, nil
	}
	out := make([]builtOverlay, len(overlays))
	for i, o := range overlays {
		m, ok := crosstabOverlayKinds[o.Kind]
		if !ok {
			return nil, margins, prismerrors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab overlay kind %q not supported (use share_of_row/share_of_col/index_vs_margin).", o.Kind),
				map[string]any{"Kind": o.Kind},
			)
		}
		axis := m.fixedAxis
		if m.userAxis {
			switch o.Axis {
			case "row":
				axis = pulsetypes.MarginAxisRow
			case "column":
				axis = pulsetypes.MarginAxisColumn
			default:
				return nil, margins, prismerrors.New(
					"PRISM_SPEC_032",
					fmt.Sprintf("crosstab overlay %q requires axis row or column (got %q).", o.Kind, o.Axis),
					map[string]any{"Kind": o.Kind, "Axis": o.Axis},
				)
			}
		}
		os := pulsetypes.OverlaySpec{
			Name:  overlayColumnName(o),
			Kind:  m.kind,
			Scope: pulsetypes.OverlayScopeCell,
			Ref:   pulsetypes.OverlayRef{Margin: &pulsetypes.OverlayMarginRef{Axis: axis}},
		}
		switch axis {
		case pulsetypes.MarginAxisRow:
			margins.Rows = true
		case pulsetypes.MarginAxisColumn:
			margins.Columns = true
		}
		out[i] = builtOverlay{spec: os, as: overlayColumnName(o)}
	}
	return out, margins, nil
}

// dateGroupComponents is the set of calendar components Pulse's
// GROUP_DATE accepts (mirrors processing/grouper.go
// validDateGroupComponents). "month" is the default when Period is
// empty.
var dateGroupComponents = map[string]bool{
	"year": true, "quarter": true, "month": true,
	"week": true, "day": true, "day_of_week": true,
}

// translateGroupers converts spec.CrosstabGroup entries into
// pulsetypes.Group entries. Supports GROUP_CATEGORY (default) and
// GROUP_DATE (type "date", calendar bucketing by Period).
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
		switch t {
		case "category":
			out[i] = &pulsetypes.Group{Type: pulsetypes.GROUP_CATEGORY, Field: g.Field}
		case "date":
			period := g.Period
			if period == "" {
				period = "month"
			}
			if !dateGroupComponents[period] {
				return nil, prismerrors.New(
					"PRISM_SPEC_032",
					fmt.Sprintf("crosstab date grouper period %q must be one of year/quarter/month/week/day/day_of_week.", period),
					map[string]any{"Axis": axis, "Period": period},
				)
			}
			params, err := json.Marshal(map[string]string{"component": period})
			if err != nil {
				return nil, err
			}
			out[i] = &pulsetypes.Group{Type: pulsetypes.GROUP_DATE, Field: g.Field, Params: params}
		default:
			return nil, prismerrors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab grouper type %q not supported (use category or date).", t),
				map[string]any{"Axis": axis, "Type": t},
			)
		}
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
		fields = append(fields, encoding.Field{Name: g.Field, Type: crosstabGroupOutputType(g, f.Type)})
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
		fields = append(fields, encoding.Field{Name: g.Field, Type: crosstabGroupOutputType(g, f.Type)})
	}
	fields = append(fields, encoding.Field{Name: cellAs, Type: aggregateOutputType(body.Cell.Aggregate)})
	// One F64 column per overlay layer (index-aligned to body.Overlays).
	for _, o := range body.Overlays {
		fields = append(fields, encoding.Field{Name: overlayColumnName(o), Type: encoding.FieldTypeF64})
	}
	// Margin rows are emitted only in the plain (non-overlay) long path;
	// overlay mode runs in matrix shape and outputs body cells only.
	if len(body.Overlays) == 0 && body.Margins != nil && (body.Margins.Rows || body.Margins.Columns || body.Margins.Grand) {
		fields = append(fields, encoding.Field{Name: "_margin", Type: encoding.FieldTypeCategoricalU32})
	}
	return &encoding.Schema{Fields: fields}, nil
}

// crosstabGroupOutputType returns the column type for a grouper in the
// long-form output. Category groupers preserve the source field type;
// date groupers emit string bucket-key labels ("2024", "2024-Q1", ...)
// regardless of the underlying date field.
func crosstabGroupOutputType(g spec.CrosstabGroup, srcType encoding.FieldType) encoding.FieldType {
	if g.Type == "date" {
		return encoding.FieldTypeCategoricalU32
	}
	return srcType
}

// longRowsFromMatrix turns the matrix-shape response (base + overlay
// layers) into the same []map[string]any long-row shape Pulse emits for
// Shape=long, so it flows through tableFromLongRows unchanged. The base
// matrix and each overlay matrix are coordinate-aligned: cell [r][c]
// addresses the same (row-key, column-key) tuple across all layers.
// Margin slots (RowMargins / ColumnMargins / GrandTotal) live off the
// Cells grid and are intentionally not emitted.
func longRowsFromMatrix(resp *pulsetypes.Response, body spec.CrosstabBody) ([]map[string]any, error) {
	if resp.Crosstab == nil || resp.Crosstab.Matrix == nil {
		return nil, prismerrors.New(
			"PRISM_PLAN_CROSSTAB_PROCESS",
			"crosstab overlay run returned no matrix payload.",
			map[string]any{},
		)
	}
	base := resp.Crosstab.Matrix
	cellAs := body.Cell.As
	if cellAs == "" {
		cellAs = "_value"
	}
	var rows []map[string]any
	for r := range base.Cells {
		for c := range base.Cells[r] {
			cell := base.Cells[r][c]
			if !cell.Present {
				continue
			}
			row := map[string]any{cellAs: cell.Scalar()}
			// Row / column grouper key values, by header field name.
			for k, field := range base.RowHeader.Fields {
				if k < len(base.RowKeys[r]) {
					row[field] = base.RowKeys[r][k]
				}
			}
			for k, field := range base.ColumnHeader.Fields {
				if k < len(base.ColumnKeys[c]) {
					row[field] = base.ColumnKeys[c][k]
				}
			}
			// Overlay value per layer, index-aligned to body.Overlays.
			for i := range body.Overlays {
				as := overlayColumnName(body.Overlays[i])
				if i < len(resp.Overlays) {
					if m := resp.Overlays[i].Payload.Matrix; m != nil &&
						r < len(m.Cells) && c < len(m.Cells[r]) {
						row[as] = m.Cells[r][c].Scalar()
						continue
					}
				}
				row[as] = 0.0
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// tableFromLongRows materialises long-form rows (Pulse Response.Data or
// the matrix-derived equivalent) into a typed *table.Table. Reuses the
// same kind switch as the chain node, with content hash derived from the
// node id (the crosstab node fingerprint is content-addressed by the
// request body already).
func tableFromLongRows(rows []map[string]any, schema *encoding.Schema, id plan.NodeID) (*table.Table, error) {
	if schema == nil {
		return nil, fmt.Errorf("crosstab: nil output schema")
	}
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
