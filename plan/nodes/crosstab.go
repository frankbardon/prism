package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/frankbardon/prism/compile"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/internal/vfs"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// CrosstabNode builds a contingency table (rows × columns × cell
// aggregate → long-form table, plus optional margin axes and overlay
// layers) from its single upstream input table. Execution is pure Go
// via the in-memory backend (compile/inmem/crosstab.go) — the node
// carries no engine coupling.
//
// v1 ships shape=long only. The heatmap encoder consumes the flat
// long-form rows regardless of margins / normalisation / overlays, so
// the wire shape stays the same. When any overlay is present the node
// runs body cells only (overlays decorate the dense cell grid; user
// margin flags do not reach the long output).
//
// Grouper support: category groupers (one Field per axis entry) and
// date groupers (type "date", calendar bucketing by period — bucket
// keys are string labels). Range / rounded / quantile groupers land
// behind a follow-up.
type CrosstabNode struct {
	id        plan.NodeID
	input     plan.NodeID
	ref       string
	fs        vfs.Fs
	body      spec.CrosstabBody
	outSchema *table.Schema
	cellAs    string
	backend   plan.Backend
}

// NewCrosstab constructs a CrosstabNode. inSchema is the upstream
// input's schema — used to compute the row / column grouper output
// types so the downstream encoder sees the right column kinds. The
// caller (plan/build) obtains it via SourceNode.Schema(nil).
func NewCrosstab(id, input plan.NodeID, ref string, fs vfs.Fs, inSchema *table.Schema, body spec.CrosstabBody) (*CrosstabNode, error) {
	if inSchema == nil {
		return nil, fmt.Errorf("crosstab: nil input schema")
	}
	if err := validateCrosstabCell(body.Cell); err != nil {
		return nil, err
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
		input:     input,
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

// Inputs implements plan.Node. Crosstab consumes the single upstream
// table it pivots.
func (n *CrosstabNode) Inputs() []plan.NodeID { return []plan.NodeID{n.input} }

// Schema implements plan.Node. Pre-computed at construction from the
// upstream schema; the row / column grouper output types plus the
// cell (and any overlay / margin sentinel) columns.
func (n *CrosstabNode) Schema(_ []*table.Schema) (*table.Schema, error) {
	return n.outSchema, nil
}

// Fingerprint implements plan.Node.
func (n *CrosstabNode) Fingerprint() string {
	return fingerprintFor("CrosstabNode", string(n.input), n.ref, crosstabBodyKey(n.body))
}

// Ref returns the source ref so plan-visualisation tooling can show
// the underlying cohort. Mirrors SourceNode.Ref().
func (n *CrosstabNode) Ref() string { return n.ref }

// FS returns the afero filesystem this node was constructed with.
func (n *CrosstabNode) FS() vfs.Fs { return n.fs }

// Body returns the crosstab body for the backend + test inspection.
func (n *CrosstabNode) Body() spec.CrosstabBody { return n.body }

// CellAs returns the resolved cell output column name (defaults to
// "_value" when the spec omits `as`).
func (n *CrosstabNode) CellAs() string { return n.cellAs }

// OutSchema returns the pre-computed output schema so the in-memory
// backend materialises the long rows with the right column kinds.
func (n *CrosstabNode) OutSchema() *table.Schema { return n.outSchema }

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

// Execute implements plan.Node via the injected backend.
func (n *CrosstabNode) Execute(ctx context.Context, in []*table.Table) (*table.Table, error) {
	if n.backend == nil {
		return nil, notImplementedErr("CrosstabNode")
	}
	return n.backend.Compile(ctx, n, in)
}

// SetBackend wires the compile backend that powers Execute.
func (n *CrosstabNode) SetBackend(b plan.Backend) { n.backend = b }

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

// overlayColumnName returns the output column name for an overlay,
// defaulting to the kind when As is empty.
func overlayColumnName(o spec.CrosstabOverlay) string {
	if o.As != "" {
		return o.As
	}
	return o.Kind
}

// validateCrosstabCell checks the cell aggregate is a known friendly
// alias and that a field is supplied when the aggregate requires one.
// The alias set is the shared aggregate registry (compile.Aliases as the
// name catalogue only — the in-memory backend maps each alias straight
// to its client-side computation).
func validateCrosstabCell(cell spec.CrosstabCell) error {
	if cell.Aggregate == "" {
		return prismerrors.New(
			"PRISM_SPEC_032",
			"crosstab.cell.aggregate is required.",
			map[string]any{},
		)
	}
	if !compile.IsAlias(cell.Aggregate) {
		return prismerrors.New(
			"PRISM_SPEC_032",
			fmt.Sprintf("crosstab.cell.aggregate %q is not a known aggregate alias.", cell.Aggregate),
			map[string]any{"Aggregate": cell.Aggregate, "Known": compile.AllAliases()},
		)
	}
	if cell.Field == "" && cell.Aggregate != "count" {
		return prismerrors.New(
			"PRISM_SPEC_032",
			fmt.Sprintf("crosstab.cell.field is required for aggregate %q.", cell.Aggregate),
			map[string]any{"Aggregate": cell.Aggregate},
		)
	}
	return nil
}

// crosstabDateComponents is the calendar-component set the date grouper
// accepts. "month" is the default when Period is empty. Mirrors the
// validate rule (validate/rules/crosstab_position.go).
var crosstabDateComponents = map[string]bool{
	"year": true, "quarter": true, "month": true,
	"week": true, "day": true, "day_of_week": true,
}

// deriveCrosstabSchema builds the long-form output schema. Row +
// column grouper fields keep their source types (date groupers emit
// string bucket-key labels); the cell column is the aggregate's output
// type; one F64 column is added per overlay layer; a "_margin" string
// column is appended when any margin is enabled (the sentinel carries
// "row" / "column" / "grand" / "" per row). Margin columns are emitted
// only in the plain (non-overlay) path.
func deriveCrosstabSchema(in *table.Schema, body spec.CrosstabBody, cellAs string) (*table.Schema, error) {
	fields := make([]table.Field, 0, len(body.Rows)+len(body.Columns)+2)
	appendAxis := func(groups []spec.CrosstabGroup, axis string) error {
		for _, g := range groups {
			if g.Field == "" {
				return prismerrors.New(
					"PRISM_SPEC_032",
					fmt.Sprintf("crosstab.%s requires a non-empty field.", axis),
					map[string]any{"Axis": axis},
				)
			}
			src := in.Field(g.Field)
			if src == nil {
				return prismerrors.New(
					"PRISM_SPEC_032",
					fmt.Sprintf("crosstab.%s references unknown field %q.", axis, g.Field),
					map[string]any{"Axis": axis, "Field": g.Field},
				)
			}
			ft, err := crosstabGroupOutputType(g, src.Type)
			if err != nil {
				return err
			}
			fields = append(fields, table.Field{Name: g.Field, Type: ft})
		}
		return nil
	}
	if err := appendAxis(body.Rows, "rows"); err != nil {
		return nil, err
	}
	if err := appendAxis(body.Columns, "columns"); err != nil {
		return nil, err
	}
	fields = append(fields, table.Field{Name: cellAs, Type: aggregateOutputType(body.Cell.Aggregate)})
	// One F64 column per overlay layer (index-aligned to body.Overlays).
	for _, o := range body.Overlays {
		fields = append(fields, table.Field{Name: overlayColumnName(o), Type: table.FieldTypeF64})
	}
	// Margin rows are emitted only in the plain (non-overlay) long path;
	// overlay mode outputs body cells only.
	if len(body.Overlays) == 0 && body.Margins != nil && (body.Margins.Rows || body.Margins.Columns || body.Margins.Grand) {
		fields = append(fields, table.Field{Name: "_margin", Type: table.FieldTypeCategoricalU32})
	}
	return &table.Schema{Fields: fields}, nil
}

// crosstabGroupOutputType returns the column type for a grouper in the
// long-form output. Category groupers preserve the source field type;
// date groupers emit string bucket-key labels ("2024", "2024-Q1", ...)
// regardless of the underlying date field.
func crosstabGroupOutputType(g spec.CrosstabGroup, srcType table.FieldType) (table.FieldType, error) {
	switch g.Type {
	case "", "category":
		return srcType, nil
	case "date":
		if g.Period != "" && !crosstabDateComponents[g.Period] {
			return 0, prismerrors.New(
				"PRISM_SPEC_032",
				fmt.Sprintf("crosstab date grouper period %q must be one of year/quarter/month/week/day/day_of_week.", g.Period),
				map[string]any{"Period": g.Period},
			)
		}
		return table.FieldTypeCategoricalU32, nil
	default:
		return 0, prismerrors.New(
			"PRISM_SPEC_032",
			fmt.Sprintf("crosstab grouper type %q not supported (use category or date).", g.Type),
			map[string]any{"Type": g.Type},
		)
	}
}
