package nodes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/frankbardon/pulse"
	"github.com/frankbardon/pulse/encoding"
	pulsetypes "github.com/frankbardon/pulse/types"
	"github.com/spf13/afero"

	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/plan"
	"github.com/frankbardon/prism/table"
)

// PulseChainNode collapses a source-rooted linear chain of fusable
// operators (Filter, Calculate, GroupAggregate, Sort) into a single
// `pulse.ProcessChain` call. The node is its own root — it opens the
// .pulse cohort itself rather than consuming an upstream Table, so
// fusion bypasses the per-source materialization the inmem backend
// would otherwise pay.
//
// The fusion pass (plan/passes/pulse_chain_fusion.go) constructs the
// node once eligibility is verified; v1 emits a single chain stage
// (one types.Request bundling filter + groupby + agg + sort). The
// ChainRequest envelope is used regardless so adding multi-stage
// support later is purely additive.
//
// Fallback semantics: if Pulse rejects a stage at execute time with
// PULSE_CHAIN_NOT_MERGEABLE, Execute wraps it as
// PRISM_PLAN_CHAIN_NOT_MERGEABLE and the executor surfaces the error
// like any other PRISM_* AppError. Callers that want to re-run with
// fusion disabled can swap the optimizer pass list.
type PulseChainNode struct {
	id        plan.NodeID
	ref       string
	fs        afero.Fs
	chainReq  *pulsetypes.ChainRequest
	outSchema *encoding.Schema
	summary   string
	stageIDs  []plan.NodeID
}

// NewPulseChain constructs a PulseChainNode. The chainReq must carry
// the cohort on its first stage; outSchema is the schema synthesised
// by the final stage (already validated by the fusion pass). Callers
// pass the absorbed node ids via stageIDs so the fingerprint reflects
// the upstream plan shape — two equivalent fusions hash identically.
func NewPulseChain(
	id plan.NodeID,
	ref string,
	fs afero.Fs,
	chainReq *pulsetypes.ChainRequest,
	outSchema *encoding.Schema,
	summary string,
	stageIDs []plan.NodeID,
) *PulseChainNode {
	stages := make([]plan.NodeID, len(stageIDs))
	copy(stages, stageIDs)
	return &PulseChainNode{
		id:        id,
		ref:       ref,
		fs:        fs,
		chainReq:  chainReq,
		outSchema: outSchema,
		summary:   summary,
		stageIDs:  stages,
	}
}

// DerivePulseChainID builds a stable id for the fused node by hashing
// the source ref together with the absorbed-node id list. Two
// equivalent fusions share an id, mirroring SourceNode's content
// fingerprint convention.
func DerivePulseChainID(ref string, absorbed []plan.NodeID) plan.NodeID {
	h := sha256.New()
	h.Write([]byte(ref))
	h.Write([]byte{0})
	for _, id := range absorbed {
		h.Write([]byte(id))
		h.Write([]byte{0})
	}
	return plan.NodeID("pulse_chain:" + hex.EncodeToString(h.Sum(nil)[:8]))
}

// ID implements plan.Node.
func (n *PulseChainNode) ID() plan.NodeID { return n.id }

// Inputs implements plan.Node — the chain node is a leaf (it opens
// the cohort itself).
func (n *PulseChainNode) Inputs() []plan.NodeID { return nil }

// Schema implements plan.Node. The chain output schema is computed at
// fusion time via processing.ChainOutputSchema(finalStageReq).
func (n *PulseChainNode) Schema(_ []*encoding.Schema) (*encoding.Schema, error) {
	return n.outSchema, nil
}

// Execute implements plan.Node. Opens a fresh pulse.Pulse against the
// node's afero.Fs, runs ProcessChain, and materialises the final
// stage's row maps into a typed *table.Table.
//
// Pulse errors carrying a recognised code (PULSE_CHAIN_NOT_MERGEABLE,
// PULSE_PROCESSING_*) are rewrapped as PRISM_PLAN_CHAIN_NOT_MERGEABLE
// or PRISM_RESOLVE_006 so the executor's NodeError envelope picks up
// a stable PRISM_* identifier.
func (n *PulseChainNode) Execute(ctx context.Context, _ []*table.Table) (*table.Table, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if n.fs == nil {
		return nil, fmt.Errorf("pulse_chain: fs is nil")
	}
	if n.chainReq == nil || len(n.chainReq.Stages) == 0 {
		return nil, fmt.Errorf("pulse_chain: empty chain request")
	}
	p, err := pulse.New(pulse.Options{FS: n.fs})
	if err != nil {
		return nil, prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse failed to open %s for chain execution: %v.", n.ref, err),
			map[string]any{"Ref": n.ref, "Reason": err.Error()},
			err,
		)
	}
	resp, err := p.ProcessChain(ctx, n.chainReq)
	if err != nil {
		return nil, classifyChainErr(n.ref, err)
	}
	if resp == nil || resp.Final == nil {
		return nil, prismerrors.New(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse returned an empty chain response for %s.", n.ref),
			map[string]any{"Ref": n.ref},
		)
	}
	hash := hashChainRef(n.ref, n.Fingerprint())
	return tableFromChainResponse(resp.Final, n.outSchema, hash)
}

// Fingerprint implements plan.Node. Mixes the source ref and each
// absorbed stage id so equivalent fusions share a cache key.
func (n *PulseChainNode) Fingerprint() string {
	parts := append([]string{n.ref}, idsToStrings(n.stageIDs)...)
	return fingerprintFor("PulseChainNode", parts...)
}

// Ref returns the source ref for diagnostics and tests.
func (n *PulseChainNode) Ref() string { return n.ref }

// ChainRequest returns the underlying Pulse chain request for tests
// and plan renderers.
func (n *PulseChainNode) ChainRequest() *pulsetypes.ChainRequest { return n.chainReq }

// StageIDs returns the ids of the upstream Prism nodes this chain
// replaced, in fusion order.
func (n *PulseChainNode) StageIDs() []plan.NodeID {
	out := make([]plan.NodeID, len(n.stageIDs))
	copy(out, n.stageIDs)
	return out
}

// Kind implements plan.Labeled.
func (n *PulseChainNode) Kind() string { return "PulseChainNode" }

// Summary implements plan.Labeled — human-readable label produced by
// the fusion pass.
func (n *PulseChainNode) Summary() string { return n.summary }

// classifyChainErr maps a pulse-side error onto a PRISM_* code. The
// chain gate failure (PULSE_CHAIN_NOT_MERGEABLE) gets its own code so
// the executor can be configured to fall back to per-node execution
// in a future revision; today the error surfaces verbatim.
func classifyChainErr(ref string, err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "PULSE_CHAIN_NOT_MERGEABLE"):
		return prismerrors.Wrap(
			"PRISM_PLAN_CHAIN_NOT_MERGEABLE",
			fmt.Sprintf("Pulse chain rejected a stage as non-mergeable for %s: %v.", ref, err),
			map[string]any{"Ref": ref, "Reason": err.Error()},
			err,
		)
	default:
		return prismerrors.Wrap(
			"PRISM_RESOLVE_006",
			fmt.Sprintf("Pulse chain execution failed for %s: %v.", ref, err),
			map[string]any{"Ref": ref, "Reason": err.Error()},
			err,
		)
	}
}

// hashChainRef synthesises a Table.Hash() input that is stable across
// runs but distinct from a non-fused source's hash. Cache lookups in
// the LRU need the chain's identity to vary as fingerprints do.
func hashChainRef(ref, fingerprint string) string {
	h := sha256.New()
	h.Write([]byte(ref))
	h.Write([]byte{0})
	h.Write([]byte(fingerprint))
	return "pulse_chain:" + hex.EncodeToString(h.Sum(nil)[:16])
}

// tableFromChainResponse materialises a pulse.Response into a typed
// *table.Table using the chain output schema. Categorical_u32 fields
// land as table.StringColumn (dictionary already populated by
// pulse.processing.RecordsFromChainRows during the final stage); F64
// fields land as table.FloatColumn. Missing cells become zero-value
// today — null bitmap support is a separate backlog item shared with
// the hash-join executor.
func tableFromChainResponse(resp *pulsetypes.Response, schema *encoding.Schema, hash string) (*table.Table, error) {
	if schema == nil {
		return nil, fmt.Errorf("pulse_chain: nil chain output schema")
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
			return nil, fmt.Errorf("pulse_chain: unsupported field type %s for %q", f.Type, f.Name)
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
	return table.NewTable(schema, cols, len(rows), hash)
}

func coerceString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func coerceFloatRow(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func idsToStrings(ids []plan.NodeID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}
