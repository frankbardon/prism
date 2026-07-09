package errors

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// CodeMetadata describes one Prism error code: its message template,
// fixup templates, and any cross-references.
type CodeMetadata struct {
	// Code is the PRISM_* identifier.
	Code string
	// Message is the user-facing template (Go text/template syntax).
	Message string
	// Fixups is the ordered list of fixup templates (Go text/template).
	Fixups []string
	// FixupNotApplicable marks codes that legitimately have no fixups.
	FixupNotApplicable bool
	// SeeAlso lists related codes or doc references.
	SeeAlso []string
}

// Codes is the canonical Prism error code catalog. Codes share the
// PRISM_<DOMAIN>_NNN form. New codes append at the bottom of their
// domain block; existing codes are not renumbered.
var Codes = map[string]CodeMetadata{
	"PRISM_SPEC_001": {
		Code:    "PRISM_SPEC_001",
		Message: `Field {{.Field}} not in source schema for dataset {{.Dataset}}.`,
		Fixups: []string{
			`Check the field name spelling. Available fields: {{.Available}}`,
			`If the field comes from a transform, make sure the transform's "as" output name matches.`,
			`Run ` + "`prism inspect {{.Dataset}}`" + ` to list all fields in the source.`,
		},
		SeeAlso: []string{"PRISM_SPEC_002", "PRISM_SPEC_005"},
	},
	"PRISM_SPEC_002": {
		Code:    "PRISM_SPEC_002",
		Message: `Aggregate op {{.Op}} is not compatible with field {{.Field}} of type {{.FieldType}}.`,
		Fixups: []string{
			`Choose an aggregate compatible with {{.FieldType}}: {{.Compatible}}.`,
			`If you need a numeric aggregate on a {{.FieldType}} field, change the field's measure type or pre-cast it in a calculate transform.`,
		},
		SeeAlso: []string{"PRISM_SPEC_001"},
	},
	"PRISM_SPEC_003": {
		Code:    "PRISM_SPEC_003",
		Message: `Encoding channel {{.Channel}} is not valid for mark type {{.Mark}}.`,
		Fixups: []string{
			`Use a channel supported by {{.Mark}}: {{.Allowed}}.`,
			`If you want {{.Channel}} semantics, switch to a compatible mark type.`,
		},
		SeeAlso: []string{"PRISM_SPEC_008"},
	},
	"PRISM_SPEC_004": {
		Code:    "PRISM_SPEC_004",
		Message: `Selection reference {{.Selection}} does not resolve to a declared selection.`,
		Fixups: []string{
			`Declare the selection in the spec's "selection" block before referencing it.`,
			`Available selections: {{.Available}}.`,
		},
	},
	"PRISM_SPEC_005": {
		Code:    "PRISM_SPEC_005",
		Message: `Dataset reference {{.Dataset}} does not resolve to a declared dataset.`,
		Fixups: []string{
			`Declare the dataset in the spec's "datasets" block, register it via prism serve, or declare it page-side via <prism-dataset>.`,
			`Available datasets: {{.Available}}.`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_001"},
	},
	// PRISM_SPEC_006 is RETAINED but no longer emitted. Prism dropped its
	// expression language: `filter`, `calculate`, and condition `test`
	// are structured JSON built-ins whose shape errors surface as
	// PRISM_SPEC_037 (filter) / PRISM_SPEC_038 (calculate) at validate
	// time and are rejected at decode when a raw string is supplied. The
	// entry stays so existing SeeAlso cross-references (PRISM_COMPILE_002,
	// PRISM_SPEC_037) and `prism errors lookup PRISM_SPEC_006` still
	// resolve.
	"PRISM_SPEC_006": {
		Code:    "PRISM_SPEC_006",
		Message: `Retired code: expression parsing was replaced by structured filter / calculate built-ins.`,
		Fixups: []string{
			`Prism has no expression language. Write a structured predicate for filter (see PRISM_SPEC_037) and a structured expression tree for calculate (see PRISM_SPEC_038).`,
			`A raw string where a predicate or expression is expected is rejected at decode time — replace it with {op, field, value} leaves / {op, operands} nodes.`,
		},
		SeeAlso: []string{"PRISM_SPEC_037", "PRISM_SPEC_038"},
	},
	"PRISM_SPEC_007": {
		Code:    "PRISM_SPEC_007",
		Message: `Scale type {{.ScaleType}} is not compatible with field {{.Field}} of type {{.FieldType}}.`,
		Fixups: []string{
			`Use a scale type compatible with {{.FieldType}}: {{.Compatible}}.`,
			`If you intended the field to be {{.ScaleFor}}, change the encoding's "type" to match.`,
		},
		SeeAlso: []string{"PRISM_SPEC_002"},
	},
	"PRISM_SPEC_008": {
		Code:    "PRISM_SPEC_008",
		Message: `Mark {{.Mark}} requires theta encoding (and typically color), not x/y.`,
		Fixups: []string{
			`Replace the x/y encodings with theta + color: { "theta": {"field": "...", "type": "quantitative"}, "color": {"field": "...", "type": "nominal"} }.`,
			`If you need x/y semantics, switch to a mark like bar or rect.`,
		},
		SeeAlso: []string{"PRISM_SPEC_003"},
	},
	"PRISM_SPEC_009": {
		Code:    "PRISM_SPEC_009",
		Message: `$schema value {{.Schema}} does not reference a known Prism schema.`,
		Fixups: []string{
			`Use the canonical URN: "$schema": "urn:prism:schema:v1:spec".`,
			`Or a relative path that ends in spec.schema.json (e.g. "./.prism/schemas/spec.schema.json").`,
		},
	},

	// --- Plan / compile codes (P03+).
	"PRISM_PLAN_001": {
		Code:    "PRISM_PLAN_001",
		Message: `Cyclic dataset reference detected (involving {{.Cycle}}; {{.Nodes}} nodes unscheduled).`,
		Fixups: []string{
			`Break the cycle by introducing an intermediate named alias.`,
			`Check transform "data" and "as" aliases for accidental loops.`,
			`Run ` + "`prism plan <spec> --format dot`" + ` to visualise the DAG and locate the cycle.`,
		},
	},
	"PRISM_PLAN_002": {
		Code:    "PRISM_PLAN_002",
		Message: `Unknown or unsupported plan kind {{.Kind}} (deferred to {{.Phase}}).`,
		Fixups: []string{
			`This spec uses a feature that is not yet implemented in the current Prism build.`,
			`Composition primitives (layer, concat, facet, repeat) land in P08/P09; selections land in P13.`,
			`Track the rollout in .planning/ROADMAP.md or run ` + "`prism errors lookup PRISM_PLAN_002`" + ` for the latest status.`,
		},
	},
	"PRISM_PLAN_003": {
		Code:    "PRISM_PLAN_003",
		Message: `Transform references undeclared dataset {{.Dataset}} (available: {{.Available}}).`,
		Fixups: []string{
			`Declare the dataset in "datasets" or earlier in the transform pipeline.`,
			`Check the spelling of the data/source reference.`,
			`If the dataset lives in another spec, hoist it into a top-level "datasets" entry.`,
		},
		SeeAlso: []string{"PRISM_SPEC_005", "PRISM_RESOLVE_001"},
	},
	"PRISM_COMPILE_001": {
		Code:    "PRISM_COMPILE_001",
		Message: `Node type {{.NodeType}} is not implemented yet (lands in {{.Phase}}).`,
		Fixups: []string{
			`This node is a P03 placeholder; the real Execute body ships in {{.Phase}}.`,
			`Until then the DAG builds and the rest of the pipeline runs — only this node fails.`,
			`Track progress: ` + "`prism errors lookup PRISM_COMPILE_001`" + ` or .planning/ROADMAP.md.`,
		},
	},
	"PRISM_COMPILE_002": {
		Code:    "PRISM_COMPILE_002",
		Message: `Transform evaluation failed at runtime: {{.Reason}}.`,
		Fixups: []string{
			`Site: {{.Site}} — a structured filter / calculate built-in referenced an unknown column or an unsupported operator.`,
			`Run ` + "`prism validate`" + ` first — well-formedness problems surface as PRISM_SPEC_037 (filter) / PRISM_SPEC_038 (calculate) before they reach the compiler.`,
			`Check that every field / to_field operand matches the upstream schema. (A runtime zero divisor yields null silently; a literal-zero divisor is rejected by PRISM_SPEC_038.)`,
		},
		SeeAlso: []string{"PRISM_SPEC_037", "PRISM_SPEC_038"},
	},
	"PRISM_COMPILE_003": {
		Code:    "PRISM_COMPILE_003",
		Message: `Aggregate alias {{.Alias}} is not yet supported by backend {{.Backend}}.`,
		Fixups: []string{
			`Use a supported alias: count, sum, mean, median, min, max, stdev, variance, mode, distinct, q1, q3, ci0, ci1, wmean, ratio, lift, share.`,
			`If your spec relied on an upstream alias the planner forwarded, check ` + "`compile/aggregates.go`" + ` for the canonical alias catalogue.`,
			`File an issue with the alias name so it can be added to the in-memory aggregate set.`,
		},
		SeeAlso: []string{"PRISM_SPEC_002"},
	},
	// PRISM_COMPILE_004 is RETIRED but retained so `prism errors lookup`
	// still resolves it. It signalled the old Pulse backend's inability to
	// accept an in-memory cohort. Prism dropped the Pulse loader in epic
	// E4: every node now runs over the in-memory backend against a
	// materialised table.Table, so inline data is always supported and
	// this code can no longer be emitted.
	"PRISM_COMPILE_004": {
		Code:    "PRISM_COMPILE_004",
		Message: `Retired code: the Pulse backend was removed; inline data always runs over the in-memory backend.`,
		Fixups: []string{
			`This code is no longer emitted. Inline ` + "`data.values`" + ` / ` + "`datasets.*.values`" + ` materialise into a table.Table that every plan node consumes directly — there is no external backend that can reject them.`,
		},
	},
	"PRISM_RESOLVE_001": {
		Code:    "PRISM_RESOLVE_001",
		Message: `Dataset {{.Dataset}} not found in any registered source.`,
		Fixups: []string{
			`Verify the dataset name or ` + "`cohort:<id>`" + ` ref.`,
			`Add the dataset to the spec's ` + "`datasets`" + ` block (with inline ` + "`values`" + `) or register it with the prism serve / DataResolver config.`,
		},
	},
	// PRISM_RESOLVE_002 is RETIRED but retained so `prism errors lookup`
	// and existing SeeAlso cross-references still resolve. It reported a
	// missing local `.pulse` file. Prism removed the Pulse file loader in
	// epic E4 — it never opens `.pulse` files, so there is no filesystem
	// lookup that can miss. Data arrives inline (`values`) or through a
	// DataResolver ref (PRISM_RESOLVE_REF_UNRESOLVED covers a ref no
	// resolver can satisfy).
	"PRISM_RESOLVE_002": {
		Code:    "PRISM_RESOLVE_002",
		Message: `Retired code: Prism no longer opens files from disk; supply rows inline or via a DataResolver.`,
		Fixups: []string{
			`This code is no longer emitted. Provide data as inline ` + "`data.values`" + ` / ` + "`datasets.*.values`" + `, or as a ` + "`data.ref`" + ` backed by a DataResolver. An unbacked ref surfaces as PRISM_RESOLVE_REF_UNRESOLVED.`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_REF_UNRESOLVED"},
	},
	// PRISM_RESOLVE_003 is RETIRED but retained so `prism errors lookup`
	// and existing SeeAlso cross-references still resolve. It reported a
	// missing archive shard. Prism removed the Pulse loader (and its
	// archive/shard addressing) in epic E4, so there are no shards to
	// miss.
	"PRISM_RESOLVE_003": {
		Code:    "PRISM_RESOLVE_003",
		Message: `Retired code: archive-shard addressing was removed with the Pulse loader.`,
		Fixups: []string{
			`This code is no longer emitted. Prism has no archive/shard concept — register each dataset by name with inline ` + "`values`" + ` or a DataResolver ref instead.`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_REF_UNRESOLVED"},
	},
	"PRISM_RESOLVE_004": {
		Code:    "PRISM_RESOLVE_004",
		Message: `Cohort id {{.Id}} is not registered in the active resolver registry.`,
		Fixups: []string{
			`Register the id with the resolver's Registry before resolving (` + "`registry.Lookup(\"{{.Id}}\")`" + `) so the ` + "`cohort:<id>`" + ` indirection points at a backing ref.`,
			`Or skip the indirection entirely: reference the dataset by name and supply its rows inline (` + "`datasets.*.values`" + `) or via a DataResolver.`,
		},
	},
	"PRISM_RESOLVE_005": {
		Code:    "PRISM_RESOLVE_005",
		Message: `Reference {{.Ref}} does not match any known form (dataset name or cohort:id).`,
		Fixups: []string{
			`Use one of: a plain dataset name (registered in ` + "`datasets`" + ` or with the DataResolver), or ` + "`cohort:<id>`" + ` indirection through the resolver Registry.`,
			`Drop trailing whitespace and any leading slashes — Prism no longer opens files, so a filesystem-looking path is not a valid ref.`,
		},
	},
	// PRISM_RESOLVE_006 is RETIRED but retained so `prism errors lookup`
	// still resolves it. It wrapped a Pulse open/decode failure. Prism
	// removed the Pulse loader in epic E4 and never parses `.pulse` bytes,
	// so there is no open step that can fail here.
	"PRISM_RESOLVE_006": {
		Code:    "PRISM_RESOLVE_006",
		Message: `Retired code: Prism no longer opens or decodes .pulse bytes.`,
		Fixups: []string{
			`This code is no longer emitted. Rows enter as inline ` + "`values`" + ` or through a DataResolver; a malformed inline row surfaces as PRISM_RESOLVE_INLINE_TYPE_MISMATCH, and an unbacked ref as PRISM_RESOLVE_REF_UNRESOLVED.`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_INLINE_TYPE_MISMATCH", "PRISM_RESOLVE_REF_UNRESOLVED"},
	},
	"PRISM_RESOLVE_007": {
		Code:    "PRISM_RESOLVE_007",
		Message: `Materialisation refused: {{.Actual}} rows would exceed PRISM_TABLE_MAX_ROWS={{.Limit}}.`,
		Fixups: []string{
			`Raise the ceiling by setting ` + "`PRISM_TABLE_MAX_ROWS`" + ` in the environment before running prism.`,
			`Pre-aggregate, sample, or filter the rows upstream (in the host that produces the inline ` + "`values`" + ` / DataResolver rows) to bring the result under the cap.`,
			`Add a ` + "`sample`" + ` or ` + "`aggregate`" + ` transform to the spec so the plan shrinks the table before it is fully materialised.`,
		},
	},
	"PRISM_RESOLVE_GCS_UNAVAILABLE": {
		Code:    "PRISM_RESOLVE_GCS_UNAVAILABLE",
		Message: `gs:// references are not a supported ref form (ref: {{.Ref}}).`,
		Fixups: []string{
			`Prism does not fetch remote objects. Fetch the data in the host, then pass the rows to Prism as inline ` + "`data.values`" + ` / ` + "`datasets.*.values`" + `.`,
			`Or serve the rows through a ` + "`resolve.DataResolver`" + ` and reference them with ` + "`data.ref`" + ` so the host owns the transport.`,
		},
	},
	"PRISM_RESOLVE_INLINE_TYPE_MISMATCH": {
		Code:    "PRISM_RESOLVE_INLINE_TYPE_MISMATCH",
		Message: `Inline row {{.Row}} field {{.Field}} has type {{.GotType}} but the schema (inferred from row 0) declared {{.WantType}}.`,
		Fixups: []string{
			`Make every row use the same JSON kind per field — strings, numbers, and bools cannot mix in a column.`,
			`Declare types explicitly via ` + "`data.fields`" + ` so the inference path is skipped.`,
		},
		SeeAlso: []string{"PRISM_SPEC_001"},
	},
	"PRISM_SPEC_PATCH_001": {
		Code:    "PRISM_SPEC_PATCH_001",
		Message: `Spec patch operation failed: {{.Op}} on {{.Path}}.`,
		Fixups: []string{
			`Inspect the failing operation index ({{.OpIndex}}) in the returned envelope — operations are applied left-to-right and the first failure stops the patch.`,
			`Confirm the JSON Pointer in {{.Path}} resolves against the current spec (use ` + "`prism plan`" + ` or ` + "`prism scene`" + ` to dump the live shape).`,
			`Atomic semantics: a failing op leaves the original spec unchanged. Re-apply with corrected ops or rebuild from a known baseline.`,
		},
		SeeAlso: []string{"PRISM_SPEC_009"},
	},
	"PRISM_RESOLVE_REF_UNRESOLVED": {
		Code:    "PRISM_RESOLVE_REF_UNRESOLVED",
		Message: `Data ref "{{.Ref}}" was not resolved by the active DataResolver.`,
		Fixups: []string{
			`Pass a DataResolver via ` + "`build.Options.DataResolver`" + ` that handles this ref.`,
			`In the browser, register a callback with ` + "`prism.setDataResolver((ref) => …)`" + ` before calling ` + "`prism.execute`" + `/` + "`prism.compile`" + `.`,
			`Pre-resolve and inject the dataset under the same name via ` + "`datasets`" + ` if a static binding suffices.`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_001", "PRISM_RESOLVE_004"},
	},
	"PRISM_SPEC_010": {
		Code:    "PRISM_SPEC_010",
		Message: `Log scale on channel {{.Channel}} requires a strictly positive domain (got {{.Value}}).`,
		Fixups: []string{
			`Filter out zero and negative values upstream of the encoded field.`,
			`Switch to scale type "linear" or "sqrt" if the domain naturally includes zero.`,
			`If the value comes from a calculate transform, guard with a clamp expression (e.g. ` + "`max(field, 1e-9)`" + `).`,
		},
		SeeAlso: []string{"PRISM_SPEC_007"},
	},
	"PRISM_SPEC_011": {
		Code:    "PRISM_SPEC_011",
		Message: `Format string {{.Spec}} on {{.Where}} is not a recognised d3-format specifier ({{.Reason}}).`,
		Fixups: []string{
			`Supported specifiers: ,.Nf | .N% | % | ,d | .Ne | .Ns | %Y | %m | %d | %H | %M | %S.`,
			`See encode/format/README.md for the full list with examples.`,
			`Drop the format property to fall back to the default rendering.`,
		},
	},
	"PRISM_RENDER_001": {
		Code:    "PRISM_RENDER_001",
		Message: `Mark geometry is malformed for {{.Mark}}.`,
		Fixups: []string{
			`Inspect the encoding values driving this mark — non-finite or null values often cause this.`,
		},
	},
	"PRISM_RENDER_FORMAT_UNAVAILABLE": {
		Code:    "PRISM_RENDER_FORMAT_UNAVAILABLE",
		Message: `Render format {{.Format}} is not available in the current Prism build.`,
		Fixups: []string{
			`SVG (default) is the built-in renderer; use --format svg.`,
			`PDF output was removed from Prism — render to SVG instead (--format svg), or convert the SVG to PDF with an external tool if you need a print artifact.`,
			`PNG support is deferred to V2; consume the JS port (prism.mjs) via prism scene + canvas for browser-native screenshots.`,
			`canvas-json consumes the Scene IR directly via 'prism scene <spec>' → render/svg's prism.mjs in the browser.`,
		},
		SeeAlso: []string{"PRISM_RENDER_001"},
	},
	"PRISM_RENDER_SCENE_EMPTY": {
		Code:    "PRISM_RENDER_SCENE_EMPTY",
		Message: `Encoded scene is empty — no marks were produced ({{.Reason}}).`,
		Fixups: []string{
			`Check the upstream transform pipeline — a filter may have removed every row.`,
			`Run ` + "`prism execute <spec>`" + ` to inspect the table the encoder consumed.`,
			`If the spec intentionally produces no marks, verify axes still render in the SVG output.`,
		},
	},
	"PRISM_RENDER_THEME_UNKNOWN": {
		Code:    "PRISM_RENDER_THEME_UNKNOWN",
		Message: `Unknown theme {{.Theme}} (registered themes: {{.Available}}).`,
		Fixups: []string{
			`Use one of the built-in theme names: light | dark | print.`,
			`To use a custom theme, load it via theme.LoadFile(path) before rendering.`,
			`Drop --theme to fall back to the default (light).`,
		},
	},
	"PRISM_ENCODE_001": {
		Code:    "PRISM_ENCODE_001",
		Message: `Encode-time mismatch: field {{.Field}} not present in upstream table from source {{.Source}}.`,
		Fixups: []string{
			`Available fields in the upstream table: {{.Available}}.`,
			`Run ` + "`prism validate <spec>`" + ` — most field-existence errors surface as PRISM_SPEC_001 earlier.`,
			`Check that the transform pipeline does not project away the field before the mark consumes it.`,
		},
		SeeAlso: []string{"PRISM_SPEC_001"},
	},

	// --- P07 multi-source / join / union / optimizer codes.
	"PRISM_RESOLVE_DUPLICATE_DATASET": {
		Code: "PRISM_RESOLVE_DUPLICATE_DATASET",
		Message: `Dataset alias {{.Alias}} is declared more than once ` +
			`(first at {{.First}}, again at {{.Second}}).`,
		Fixups: []string{
			`Rename one of the colliding aliases so each dataset has a unique name in the spec.`,
			`If the second occurrence is a transform "as" name, pick a name that does not collide with a registered dataset.`,
			`Run ` + "`prism plan <spec> --format json`" + ` to inspect the alias registry the builder produced.`,
		},
		SeeAlso: []string{"PRISM_PLAN_003", "PRISM_RESOLVE_001"},
	},
	"PRISM_JOIN_001": {
		Code:    "PRISM_JOIN_001",
		Message: `Join key {{.Key}} has incompatible kinds on the two sides (left={{.LeftKind}}, right={{.RightKind}}).`,
		Fixups: []string{
			`Cast the column on one side via a calculate transform so both sides share a table.FieldType.`,
			`If one side is categorical and the other numeric, decide which storage shape the join semantically requires.`,
			`Inspect the schemas with ` + "`prism execute <spec>`" + ` to see each side's columns + kinds.`,
		},
		SeeAlso: []string{"PRISM_JOIN_002", "PRISM_JOIN_003"},
	},
	"PRISM_JOIN_002": {
		Code:    "PRISM_JOIN_002",
		Message: `Join key {{.Key}} is missing on the {{.Side}} side (available: {{.Available}}).`,
		Fixups: []string{
			`Check the spelling of the join key against the table the {{.Side}} input produces.`,
			`If the column is produced by a transform, ensure that transform runs before the join.`,
			`Use ` + "`prism plan <spec> --format dot`" + ` to confirm the DAG wiring matches the spec.`,
		},
		SeeAlso: []string{"PRISM_JOIN_001"},
	},
	"PRISM_JOIN_003": {
		Code:    "PRISM_JOIN_003",
		Message: `Join would produce {{.Actual}} rows (left × right) and exceeds PRISM_JOIN_MAX_ROWS={{.Limit}}.`,
		Fixups: []string{
			`Pre-aggregate one or both sides upstream of the join so the cartesian product fits under the cap.`,
			`Raise the ceiling by setting ` + "`PRISM_JOIN_MAX_ROWS`" + ` in the environment (warning: 5M ≈ 500MB at 20 columns).`,
			`Perform the join upstream in the host that materialises the rows, then hand Prism the pre-joined ` + "`values`" + ` inline.`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_007"},
	},
	"PRISM_PLAN_004": {
		Code:    "PRISM_PLAN_004",
		Message: `Union input schemas disagree: {{.Diff}}.`,
		Fixups: []string{
			`Make every union input expose the same column names and table.FieldTypes in the same order.`,
			`If you need a relational union of differing shapes, project each side first to the shared columns.`,
			`Inspect each input's schema via ` + "`prism plan <spec> --format json`" + ` and reconcile differences.`,
		},
		SeeAlso: []string{"PRISM_PLAN_003"},
	},
	"PRISM_PLAN_005": {
		Code:    "PRISM_PLAN_005",
		Message: `Channel {{.Channel}} cannot be resolved as shared: layers disagree on type ({{.Types}}).`,
		Fixups: []string{
			`Convert one layer's channel to the matching type via a "calculate" cast upstream of the encoder.`,
			`Switch the channel to a compatible measure type so every layer publishes the same scale family.`,
			"Set `resolve.scale.{{.Channel}}` to `independent` to keep per-layer scales + per-layer axes.",
		},
		SeeAlso: []string{"PRISM_PLAN_002", "PRISM_SPEC_007", "PRISM_RESOLVE_DUPLICATE_DATASET"},
	},
	// PRISM_PLAN_CHAIN_NOT_MERGEABLE is RETIRED but retained so
	// `prism errors lookup` still resolves it. It was emitted by the
	// chain-fusion optimizer pass, which pushed a source-rooted linear
	// chain down to an external columnar reader. Prism now materialises
	// every source into a table.Table and computes all filters and
	// aggregates client-side over the in-memory backend, so there is no
	// chain gate that can reject a fused stage.
	"PRISM_PLAN_CHAIN_NOT_MERGEABLE": {
		Code:    "PRISM_PLAN_CHAIN_NOT_MERGEABLE",
		Message: `Retired code: chain-fusion was removed; all transforms run over the in-memory backend.`,
		Fixups: []string{
			`This code is no longer emitted. Filters, group-aggregates, and sorts all execute client-side against the materialised table — there is no fused chain that can fail.`,
		},
		SeeAlso: []string{"PRISM_PLAN_003"},
	},
	"PRISM_WARN_DOWNSAMPLE": {
		Code:    "PRISM_WARN_DOWNSAMPLE",
		Message: `Source {{.Source}} exceeds PRISM_RENDER_MAX_MARKS={{.Limit}} ({{.Actual}} rows); injected SampleNode({{.SampleN}}).`,
		Fixups: []string{
			`If you need every row plotted, raise the ceiling via PRISM_RENDER_MAX_MARKS or pass --no-downsample (when --no-downsample is wired).`,
			`If the chart is exploratory, the sample is deterministic for the spec's seed.`,
			`Pre-aggregate upstream of the encoder to avoid the auto-sample entirely.`,
		},
	},
	"PRISM_WARN_LAYER_SKIPPED": {
		Code:    "PRISM_WARN_LAYER_SKIPPED",
		Message: `Layer {{.Layer}} skipped: upstream Source {{.Source}} failed ({{.Code}}).`,
		Fixups: []string{
			"Rerun with `--abort-on-error` to fail fast instead of dropping the layer.",
			`Inspect the upstream error code via ` + "`prism errors lookup {{.Code}}`" + ` and unblock the failing Source.`,
			`Remove the offending dataset from "datasets" if it is no longer published.`,
		},
		SeeAlso: []string{"PRISM_COMPILE_001"},
	},

	// --- P09 facet / repeat codes.
	"PRISM_SPEC_012": {
		Code:    "PRISM_SPEC_012",
		Message: `Repeat substitution {{.Ref}} references axis {{.Axis}} but the parent repeat block declares only {{.Declared}}.`,
		Fixups: []string{
			`Declare the missing axis on the parent repeat block (e.g. "repeat": {"{{.Axis}}": ["field_a", "field_b"]}).`,
			`If the child spec needs a literal field name, replace the {"repeat": ...} substitution with a bare {"field": "name"}.`,
			`If you intended a different axis, update the substitution to match: {{.Declared}}.`,
		},
		SeeAlso: []string{"PRISM_SPEC_005", "PRISM_PLAN_002"},
	},

	// --- P10 composite mark codes.
	"PRISM_SPEC_013": {
		Code:    "PRISM_SPEC_013",
		Message: `Composite mark {{.Mark}} cannot expand: {{.Reason}}.`,
		Fixups: []string{
			`Check the mark's required channels: pie/donut → theta + color; histogram → x (quantitative); heatmap → x + y + color; boxplot/violin → one category axis + one quantitative axis.`,
			`Replace the mark with a primitive (bar/rect/arc/rule/point) when the encoding does not fit the composite's required shape.`,
			`If you need a different aggregation, write the expansion by hand using primitive marks.`,
		},
		SeeAlso: []string{"PRISM_SPEC_003", "PRISM_SPEC_008"},
	},

	// --- P11 specialty mark codes.
	"PRISM_SPEC_016": {
		Code:    "PRISM_SPEC_016",
		Message: `Image URL {{.URL}} is not allowed (offline-first; only data: and relative paths are accepted).`,
		Fixups: []string{
			`Embed the image as a base64 data: URL ("data:image/png;base64,...").`,
			`Reference a relative path under the spec's working directory; the renderer passes the string through to <image href>.`,
			`Remote fetch is intentionally disabled — Prism plots must render without network access. See PROJECT.md.`,
		},
		SeeAlso: []string{"PRISM_RENDER_001"},
	},
	"PRISM_SPEC_017": {
		Code:    "PRISM_SPEC_017",
		Message: `Mark "path" requires a non-empty d field (got {{.Got}}).`,
		Fixups: []string{
			`Set mark_def.path or encoding.path.value to a valid SVG path string (e.g. "M 0 0 L 10 10 Z").`,
			`Path mark is the escape hatch for SVG primitives without first-class Prism support — its sole input is the d string passed through to <path d=...>.`,
			`If you intended a polyline, use mark "line" with x/y encodings instead.`,
		},
		SeeAlso: []string{"PRISM_SPEC_003"},
	},
	"PRISM_SPEC_018": {
		Code:    "PRISM_SPEC_018",
		Message: `Sankey mark requires source, target, and value channels (missing: {{.Missing}}).`,
		Fixups: []string{
			`Bind each channel: { "source": {"field": "src", "type": "nominal"}, "target": {"field": "tgt", "type": "nominal"}, "value": {"field": "v", "type": "quantitative"} }.`,
			`Sankey reads a flat-table form: one row per link with src node, tgt node, and flow magnitude.`,
			`If you have a {nodes, links} two-array form, flatten it to a single table with the three required columns before passing to Prism.`,
		},
		SeeAlso: []string{"PRISM_SPEC_013"},
	},

	// --- P13 selection codes.
	"PRISM_SPEC_019": {
		Code:    "PRISM_SPEC_019",
		Message: `Selection {{.Selection}} encoding {{.Channel}} is not bound in the spec encoding block (available: {{.Available}}).`,
		Fixups: []string{
			`Bind the {{.Channel}} channel in the spec's "encoding" block — selections can only respond to channels that have a backing field.`,
			`Remove "{{.Channel}}" from the selection's "encodings" list if the channel is intentionally unbound.`,
			`Channel names are lowercase (x | y | x2 | y2 | theta | color | size | shape | opacity | fill | stroke); match the casing exactly.`,
		},
		SeeAlso: []string{"PRISM_SPEC_004", "PRISM_SPEC_020"},
	},
	"PRISM_SPEC_020": {
		Code:    "PRISM_SPEC_020",
		Message: `Interval selection {{.Selection}} uses non-position channel {{.Channel}} (intervals brush over position axes only).`,
		Fixups: []string{
			`Change "{{.Channel}}" to a position channel (x | y | x2 | y2 | theta); intervals brush over continuous axes only.`,
			`For filtering by color / size / shape values, use a point selection on the underlying field instead of an interval brush.`,
			`Theta intervals brush over polar position; valid for arc / pie / donut marks.`,
		},
		SeeAlso: []string{"PRISM_SPEC_019"},
	},

	// --- P17 WASM standalone runtime codes.
	"PRISM_WASM_001": {
		Code:    "PRISM_WASM_001",
		Message: `Fetch-backed filesystem failed to load {{.URL}} (HTTP {{.Status}}: {{.Reason}}).`,
		Fixups: []string{
			`Confirm the URL is reachable from the page origin and the server allows CORS for cross-origin requests.`,
			`The browser runtime only fetches geodata tier bundles (` + "`<tier>.geo.json`" + `) — set the base via ` + "`prism.geo.setBundleURL(url)`" + ` or ` + "`data-prism-geodata-url`" + ` and serve them from a static host (` + "`prism static-bundle`" + ` emits them alongside the wasm).`,
			`Chart data does not travel over fetch: supply it inline as ` + "`data.values`" + ` / ` + "`datasets.*.values`" + `, or wire ` + "`prism.setDataResolver`" + ` to return the rows for a ` + "`data.ref`" + `.`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_REF_UNRESOLVED", "PRISM_GEODATA_TIER_MISSING"},
	},
	// PRISM_WASM_002 is RETIRED but retained so `prism errors lookup`
	// still resolves it. It reported a static host that refused Range
	// requests, blocking archive-shard random access in the browser. The
	// Pulse loader and its archive/shard fetch path were removed in epic
	// E4: the browser runtime fetches only whole geodata tiles and reads
	// chart rows from inline `values` / a DataResolver, so there is no
	// Range-based shard access to fail.
	"PRISM_WASM_002": {
		Code:    "PRISM_WASM_002",
		Message: `Retired code: browser archive-shard fetch was removed with the Pulse loader.`,
		Fixups: []string{
			`This code is no longer emitted. The wasm runtime fetches only whole geodata tiles; chart rows arrive inline (` + "`values`" + `) or through ` + "`prism.setDataResolver`" + `, so no HTTP Range support is required.`,
		},
		SeeAlso: []string{"PRISM_WASM_001"},
	},
	// PRISM_WASM_BUDGET_EXCEEDED is RETIRED but retained so `prism errors
	// lookup` still resolves it. It guarded the standard-Go js/wasm size gate
	// (`PRISM_WASM_MAX_BYTES` / `PRISM_WASM_RAW_MAX_BYTES` over the
	// `make build-wasm` artifact). The standard-Go wasm build was dropped —
	// TinyGo is now the sole browser artifact — so this AppError is no longer
	// emitted. The TinyGo module carries its own, tighter size budget in
	// `internal/gates/wasm_tinygo_size_test.go` (`PRISM_WASM_TINYGO_MAX_BYTES`
	// / `PRISM_WASM_TINYGO_RAW_MAX_BYTES`); an overrun there surfaces as a
	// plain Go test failure (labelled with this identifier for lookup), not a
	// PRISM_* envelope.
	"PRISM_WASM_BUDGET_EXCEEDED": {
		Code:    "PRISM_WASM_BUDGET_EXCEEDED",
		Message: `Retired code: the standard-Go wasm size gate was removed when TinyGo became the sole browser build.`,
		Fixups: []string{
			`This code is no longer emitted. The TinyGo module is guarded by ` + "`internal/gates/wasm_tinygo_size_test.go`" + ` via ` + "`PRISM_WASM_TINYGO_MAX_BYTES`" + ` / ` + "`PRISM_WASM_TINYGO_RAW_MAX_BYTES`" + `; an overrun is a plain test failure, not a PRISM_* envelope.`,
			`To shrink the TinyGo artifact, drop newly-imported dependencies from ` + "`cmd/prismwasm/main.go`" + ` and check ` + "`go list -deps ./cmd/prismwasm`" + ` (built under ` + "`GOOS=js GOARCH=wasm`" + `) for transitive imports that bloat the binary.`,
		},
	},
	"PRISM_WARN_WASM_COLD_START": {
		Code:    "PRISM_WARN_WASM_COLD_START",
		Message: `WASM cold-start exceeded the soft timing budget ({{.Actual}}ms vs {{.Budget}}ms p95).`,
		Fixups: []string{
			`Cold-start variance is acceptable on first load; warm renders should fall well under the budget.`,
			`Preload the wasm asset with ` + "`<link rel=\"preload\" as=\"fetch\" type=\"application/wasm\" crossorigin>`" + ` so the download starts in parallel with the loader parse.`,
			`Confirm the host serves prism.wasm with ` + "`Content-Type: application/wasm`" + ` so the browser uses ` + "`WebAssembly.instantiateStreaming`" + `.`,
		},
	},
	"PRISM_SPEC_021": {
		Code:    "PRISM_SPEC_021",
		Message: `Geo projection or geo-mark binding is invalid: {{.Field}}.`,
		Fixups: []string{
			`Set ` + "`projection.type`" + ` to one of: mercator | equirectangular | naturalearth | albers_usa | orthographic.`,
			`Geoshape marks require ` + "`encoding.feature.field`" + `; geopoint marks require both ` + "`encoding.longitude.field`" + ` and ` + "`encoding.latitude.field`" + `.`,
			`Tier values must be one of: world-110m | world-50m | admin1-50m.`,
		},
		SeeAlso: []string{"PRISM_GEO_001"},
	},
	"PRISM_GEO_001": {
		Code:    "PRISM_GEO_001",
		Message: `Feature {{.Field}} not found in geodata tier {{.Source}} (got id {{.Available}}).`,
		Fixups: []string{
			`Check that the feature id matches the manifest: admin-0 uses ISO 3166-1 alpha-3 (USA, CAN, GBR, ...); admin-1 uses ISO 3166-2 (US-CA, CA-ON, ...).`,
			`Set ` + "`projection.tier`" + ` to ` + "`admin1-50m`" + ` when looking up state/province features; the default tier (world-110m) only carries countries.`,
			`Run ` + "`prism inspect --geo`" + ` to list the feature ids present in the embedded manifest.`,
		},
		SeeAlso: []string{"PRISM_ENCODE_001"},
	},
	"PRISM_GEO_002": {
		Code:    "PRISM_GEO_002",
		Message: `Geo bundle could not be loaded for tier {{.Tier}}: {{.Reason}}.`,
		Fixups: []string{
			`Host build: this should never fail — regenerate the embedded artifact via ` + "`make geodata`" + `.`,
			`WASM build: confirm ` + "`prism static-bundle`" + ` was run and the geodata/ directory is served at the URL passed to ` + "`prism.geo.setBundleURL`" + ` (default: /static/prism/geodata/).`,
			`Check the browser console for a 404 on the missing tier file.`,
		},
		SeeAlso: []string{"PRISM_GEO_001"},
	},
	"PRISM_GEODATA_DIR_UNSET": {
		Code:    "PRISM_GEODATA_DIR_UNSET",
		Message: `Geodata bundle directory is not configured; cannot load tier {{.Tier}}.`,
		Fixups: []string{
			`Pass ` + "`--geodata-dir <path>`" + ` or set the ` + "`PRISM_GEODATA`" + ` environment variable to a directory holding the tier files (world-110m.geo.json, world-50m.geo.json, admin1-50m.geo.json).`,
			`The committed tiers ship in the repo's geodata/ directory; for a standalone binary, download them from https://frankbardon.github.io/prism/static/prism/geodata/ or emit them with ` + "`prism static-bundle`" + `.`,
		},
		SeeAlso: []string{"PRISM_GEODATA_TIER_MISSING", "PRISM_GEO_002"},
	},
	"PRISM_GEODATA_TIER_MISSING": {
		Code:    "PRISM_GEODATA_TIER_MISSING",
		Message: `Geodata tier file for {{.Tier}} not found at {{.Path}}.`,
		Fixups: []string{
			`Confirm the configured geodata directory contains ` + "`{{.Tier}}.geo.json`" + `.`,
			`Regenerate the committed tiers with ` + "`make geodata`" + `, or download {{.Tier}}.geo.json from https://frankbardon.github.io/prism/static/prism/geodata/{{.Tier}}.geo.json.`,
			`Point ` + "`--geodata-dir`" + ` / ` + "`PRISM_GEODATA`" + ` at the directory that actually holds the tier files.`,
		},
		SeeAlso: []string{"PRISM_GEODATA_DIR_UNSET", "PRISM_GEO_002"},
	},
	"PRISM_SPEC_022": {
		Code:    "PRISM_SPEC_022",
		Message: `animation.easing {{.Easing}} is not a known easing name.`,
		Fixups: []string{
			`Use one of the supported easings: linear, cubic_in, cubic_out, cubic_in_out, quad_in, quad_out, quad_in_out, sine_in, sine_out, sine_in_out, expo_in, expo_out, expo_in_out.`,
			`Omit ` + "`animation.easing`" + ` to use the default (cubic_in_out).`,
		},
		SeeAlso: []string{"PRISM_SPEC_023"},
	},
	"PRISM_SPEC_023": {
		Code:    "PRISM_SPEC_023",
		Message: `animation block declared but no encoding channel carries ` + "`key: true`" + `.`,
		Fixups: []string{
			`Add ` + "`\"key\": true`" + ` to one position or mark channel so tweens can match marks across scene swaps (e.g. ` + "`encoding.x`" + ` for object-constancy on the x-axis category).`,
			`Without a key, the animator falls back to positional matching, which is ambiguous when row counts change.`,
		},
		SeeAlso: []string{"PRISM_SPEC_024", "PRISM_WARN_ANIM_FALLBACK"},
	},
	"PRISM_SPEC_024": {
		Code:    "PRISM_SPEC_024",
		Message: `multiple encoding channels carry ` + "`key: true`" + ` (channels: {{.Channels}}); at most one is allowed.`,
		Fixups: []string{
			`Pick the single channel whose field provides stable per-row identity across scene swaps and remove ` + "`key: true`" + ` from the rest.`,
			`Composite keys are not supported in v1.`,
		},
		SeeAlso: []string{"PRISM_SPEC_023"},
	},
	"PRISM_SPEC_025": {
		Code:    "PRISM_SPEC_025",
		Message: `Condition on channel {{.Channel}} references selection {{.Selection}} which is not declared.`,
		Fixups: []string{
			`Declare the selection in the spec's "selection" block before referencing it in a condition.`,
			`Available selections: {{.Available}}.`,
			`Use a ` + "`{test: {op, field, value}}`" + ` structured-predicate condition instead of a named selection.`,
		},
		SeeAlso: []string{"PRISM_SPEC_004", "PRISM_SPEC_026"},
	},
	"PRISM_SPEC_026": {
		Code:    "PRISM_SPEC_026",
		Message: `Condition test predicate is not well-formed ({{.Reason}}) at {{.Site}}.`,
		Fixups: []string{
			`A condition ` + "`test`" + ` is a structured predicate, not an expression string: use {op, field, value} leaves and and/or/not combinators.`,
			`Confirm every referenced field exists in the dataset and comparison operands share a type (numbers vs numbers, strings vs strings).`,
			`See PRISM_SPEC_037 for the same predicate grammar used by the filter transform.`,
		},
		SeeAlso: []string{"PRISM_SPEC_037"},
	},
	"PRISM_SPEC_028": {
		Code:    "PRISM_SPEC_028",
		Message: `Mark {{.Mark}} requires source + target channels (missing: {{.Missing}}).`,
		Fixups: []string{
			`Bind ` + "`encoding.source`" + ` to the parent-id field and ` + "`encoding.target`" + ` to the child-id field.`,
			`The optional ` + "`encoding.text`" + ` channel supplies per-node labels.`,
		},
		SeeAlso: []string{"PRISM_SPEC_018"},
	},
	"PRISM_SPEC_029": {
		Code:    "PRISM_SPEC_029",
		Message: `tree mark expects exactly one root (parent field empty / null); got {{.Count}}.`,
		Fixups: []string{
			`Exactly one input row must have an empty / null parent field. Synthesise a single root if your data has multiple top-level entries.`,
			`Multi-root forests render via ` + "`layer`" + ` (one tree per layer).`,
		},
		SeeAlso: []string{"PRISM_SPEC_028"},
	},
	"PRISM_SPEC_032": {
		Code:    "PRISM_SPEC_032",
		Message: `Crosstab transform malformed (at {{.Axis}}{{if .Aggregate}} / cell {{.Aggregate}}{{end}}{{if .Field}} / field {{.Field}}{{end}}).`,
		Fixups: []string{
			`Required: ` + "`crosstab.rows`" + ` (>=1 grouper), ` + "`crosstab.columns`" + ` (>=1 grouper), ` + "`crosstab.cell.aggregate`" + ` (e.g. sum, mean, count), ` + "`crosstab.cell.field`" + ` (omit only for count). Example: ` + "`{crosstab: {rows: [{field: \"region\"}], columns: [{field: \"quarter\"}], cell: {aggregate: \"sum\", field: \"revenue\", as: \"revenue\"}}}`" + `.`,
			`Grouper type defaults to "category" (GROUP_CATEGORY). Date / range / quantile groupers land in a follow-up.`,
			`Cell aggregate must be a supported client-side alias (count, sum, mean, median, min, max, stdev, variance, q1, q3, ci0, ci1, wmean, ratio). lift + share are not yet wired into crosstab.`,
		},
		SeeAlso: []string{"PRISM_SPEC_034"},
	},
	// PRISM_SPEC_033 is RETIRED but retained so `prism errors lookup` and
	// existing SeeAlso cross-references still resolve. It reported a
	// crosstab that was not the first transform on the chain. Crosstab now
	// accepts derived input (it may follow another Prism transform — see
	// epic E3), so the chain-position constraint no longer exists and this
	// code is never emitted. Shape violations surface as PRISM_SPEC_032.
	"PRISM_SPEC_033": {
		Code:    "PRISM_SPEC_033",
		Message: `Retired code: crosstab now accepts derived input, so the "must be the first transform" constraint was removed.`,
		Fixups: []string{
			`This code is no longer emitted. A ` + "`crosstab`" + ` transform may follow another transform (e.g. ` + "`filter`" + `→` + "`crosstab`" + `); it consumes the upstream materialised rows. Shape problems (missing rows/columns/cell.aggregate) surface as PRISM_SPEC_032.`,
		},
		SeeAlso: []string{"PRISM_SPEC_032"},
	},
	"PRISM_SPEC_034": {
		Code:    "PRISM_SPEC_034",
		Message: `crosstab.normalize must be one of none/row/column/total (got {{.Normalize}}).`,
		Fixups: []string{
			`Pick one of "none" (default — raw cell aggregations), "row" (cells in each row sum to 1), "column" (cells in each column sum to 1), "total" (whole table sums to 1).`,
			`Margins are computed internally when normalize is row / column / total; their emission is independent — set ` + "`crosstab.margins.{rows,columns,grand}`" + ` to surface them on the response.`,
		},
		SeeAlso: []string{"PRISM_SPEC_032"},
	},
	// PRISM_PLAN_CROSSTAB_REQUIRES_SOURCE is RETIRED but retained so
	// `prism errors lookup` and existing SeeAlso cross-references still
	// resolve. It reported a crosstab plan node whose immediate input was
	// not a SourceNode. Crosstab gained derived-input support (epic E3):
	// the build now accepts any upstream node's materialised table, so
	// there is no source-linkage precondition and this code is never
	// emitted.
	"PRISM_PLAN_CROSSTAB_REQUIRES_SOURCE": {
		Code:    "PRISM_PLAN_CROSSTAB_REQUIRES_SOURCE",
		Message: `Retired code: crosstab now accepts derived input, so it no longer requires a SourceNode as its immediate build input.`,
		Fixups: []string{
			`This code is no longer emitted. The crosstab plan node consumes the upstream node's materialised table.Table whether it is a source, an inline dataset, or a derived transform output. Compute failures surface as PRISM_PLAN_CROSSTAB_PROCESS.`,
		},
		SeeAlso: []string{"PRISM_PLAN_CROSSTAB_PROCESS"},
	},
	"PRISM_PLAN_CROSSTAB_PROCESS": {
		Code:    "PRISM_PLAN_CROSSTAB_PROCESS",
		Message: `The crosstab computation failed for {{.Ref}}: {{.Reason}}.`,
		Fixups: []string{
			`The reason names the precise rule — check that every grouper field exists in the input schema, that the cell field's column type is numeric for sum / mean / etc., and that no aggregator alias was promoted from client-side (lift, share).`,
			`Run ` + "`prism inspect`" + ` to view the input schema without re-executing.`,
		},
	},
	"PRISM_SPEC_035": {
		Code:    "PRISM_SPEC_035",
		Message: `regression transform must declare target + at least one predictor (at {{.Path}}).`,
		Fixups: []string{
			`Declare both ` + "`target`" + ` (the dependent variable) and a non-empty ` + "`predictors`" + ` list, e.g. ` + "`{regression: {target: \"sales\", predictors: [\"spend\"], as: \"fitted\"}}`" + `.`,
			`A ` + "`regression`" + ` transform may follow another transform (e.g. ` + "`filter`" + `→` + "`regression`" + `); it fits the upstream materialised rows and no longer needs to be the first transform.`,
		},
		SeeAlso: []string{"PRISM_PLAN_REGRESSION_PROCESS"},
	},
	// PRISM_PLAN_REGRESSION_REQUIRES_SOURCE is RETIRED but retained so
	// `prism errors lookup` and existing SeeAlso cross-references still
	// resolve. It reported a regression plan node whose immediate input
	// was not a SourceNode. Regression gained derived-input support (epic
	// E3): the build now accepts any upstream node's materialised table,
	// so there is no source-linkage precondition and this code is never
	// emitted.
	"PRISM_PLAN_REGRESSION_REQUIRES_SOURCE": {
		Code:    "PRISM_PLAN_REGRESSION_REQUIRES_SOURCE",
		Message: `Retired code: regression now accepts derived input, so it no longer requires a SourceNode as its immediate build input.`,
		Fixups: []string{
			`This code is no longer emitted. The regression plan node fits the upstream node's materialised table.Table whether it is a source, an inline dataset, or a derived transform output. Fit failures surface as PRISM_PLAN_REGRESSION_PROCESS.`,
		},
		SeeAlso: []string{"PRISM_PLAN_REGRESSION_PROCESS"},
	},
	"PRISM_PLAN_REGRESSION_PROCESS": {
		Code:    "PRISM_PLAN_REGRESSION_PROCESS",
		Message: `The regression fit failed for {{.Ref}}: {{.Reason}}.`,
		Fixups: []string{
			`Check that the target and predictor exist in the input schema, are numeric columns, and that at least two complete (predictor, target) records remain after filtering.`,
			`Run ` + "`prism inspect`" + ` to view the input schema without re-executing.`,
		},
	},
	"PRISM_SPEC_030": {
		Code:    "PRISM_SPEC_030",
		Message: `Unknown color scheme {{.Scheme}} (at {{.Path}}).`,
		Fixups: []string{
			`Pick a registered scheme. Built-in categoricals: tableau10, category10, observable10, set1/2/3, dark2, paired, pastel1/2, accent, okabe_ito, tol_bright, tol_vibrant, tol_muted. Sequentials: viridis, magma, plasma, inferno, cividis, blues/greens/greys/oranges/purples/reds, turbo. Divergings: rdbu, rdylbu, brbg, prgn, piyg, puor, rdgy, rdylgn, spectral.`,
			`Or define a custom scheme under ` + "`theme.schemes`" + `: ` + "`{schemes: {brand: [\"#001eff\", \"#33ffaa\"]}}`" + `.`,
			`Run ` + "`prism schema show theme.schema.json`" + ` to inspect the full scheme catalogue.`,
		},
	},
	"PRISM_SPEC_031": {
		Code:    "PRISM_SPEC_031",
		Message: `Theme defines defaults for unknown mark type {{.Mark}} (at theme.marks.{{.Mark}}).`,
		Fixups: []string{
			`Mark type must match a registered mark: bar, line, area, point, rule, text, tick, rect, arc, pie, donut, histogram, heatmap, boxplot, violin, sankey, funnel, sparkline, image, path, geoshape, geopoint, tree, dendrogram, network.`,
			`Typos like ` + "`bars`" + ` or ` + "`Bar`" + ` (with capital) fail this rule — mark names are lowercase, singular.`,
		},
	},
	"PRISM_SPEC_036": {
		Code:    "PRISM_SPEC_036",
		Message: `Bullet bands must be strictly ascending (bands[{{.Index}}]={{.Value}} is not greater than its predecessor {{.Previous}}).`,
		Fixups: []string{
			`A bullet mark's ` + "`bands`" + ` are cumulative qualitative range bounds measured from zero, so each bound must be strictly greater than the one before it, e.g. ` + "`{mark: {type: \"bullet\", bands: [150, 225, 300]}}`" + `.`,
			`Flat (equal) or descending bounds would render an inverted or zero-width background range. Sort the bounds ascending and drop duplicates.`,
		},
		SeeAlso: []string{"PRISM_SPEC_003"},
	},
	"PRISM_SPEC_037": {
		Code:    "PRISM_SPEC_037",
		Message: `Filter predicate is not well-formed ({{.Reason}}) at {{.Site}}.`,
		Fixups: []string{
			`A filter predicate is a structured node, not an expression string. Use a leaf like ` + "`{op: \"gt\", field: \"Horsepower\", value: 100}`" + ` (or ` + "`to_field`" + ` for a field-vs-field compare) and nest with ` + "`{and: [...]}`" + `, ` + "`{or: [...]}`" + `, ` + "`{not: {...}}`" + `.`,
			`Every ` + "`field`" + ` / ` + "`to_field`" + ` must exist in the source schema, comparisons must be type-compatible (numeric vs numeric, string vs string), a ` + "`between`" + ` must have ` + "`lo`" + ` <= ` + "`hi`" + `, and ` + "`one_of`" + ` / ` + "`not_one_of`" + ` need a non-empty ` + "`values`" + ` set.`,
		},
		SeeAlso: []string{"PRISM_SPEC_001", "PRISM_SPEC_006"},
	},
	"PRISM_SPEC_038": {
		Code:    "PRISM_SPEC_038",
		Message: `Calculate expression is not well-formed ({{.Reason}}) at {{.Site}}.`,
		Fixups: []string{
			`A calculate expression is a structured node, not an expression string. Use ` + "`{op: \"div\", operands: [{field: \"Horsepower\"}, {field: \"Weight\"}]}`" + ` for arithmetic, ` + "`{fn: \"coalesce\", args: [...]}`" + ` for functions, ` + "`{concat: [...]}`" + ` to build strings, or ` + "`{case: [{when: <predicate>, then: <expr>}], else: <expr>}`" + ` for conditionals.`,
			`Every ` + "`field`" + ` operand must exist in the source schema, a ` + "`div`" + ` / ` + "`mod`" + ` by a literal zero is rejected (a runtime zero divisor yields null), and ` + "`as`" + ` must be a non-empty name that does not shadow an existing source column.`,
		},
		SeeAlso: []string{"PRISM_SPEC_001", "PRISM_SPEC_037"},
	},
	"PRISM_SPEC_039": {
		Code:    "PRISM_SPEC_039",
		Message: `The data ` + "`source`" + ` variant was removed: Prism no longer opens .pulse files, so a spec cannot name an external source.`,
		Fixups: []string{
			`Inline the rows: replace ` + "`{\"data\": {\"source\": \"cohort.pulse\"}}`" + ` with ` + "`{\"data\": {\"values\": [{\"col\": 1}, …]}}`" + ` (or a named ` + "`datasets`" + ` entry whose value carries inline ` + "`values`" + `).`,
			`Or keep the spec portable and defer the data to the host: use ` + "`{\"data\": {\"ref\": \"<id>\"}}`" + ` and supply a ` + "`resolve.DataResolver`" + ` (server / ` + "`prism.setDataResolver`" + ` in the browser) that returns the rows for that ref.`,
		},
		SeeAlso: []string{"PRISM_SPEC_009", "PRISM_RESOLVE_REF_UNRESOLVED"},
	},
	"PRISM_WARN_NETWORK_CYCLE": {
		Code:    "PRISM_WARN_NETWORK_CYCLE",
		Message: `network input graph contains a cycle; force layout may produce a visually messy result.`,
		Fixups: []string{
			`Cycles are valid for the network mark — the layout converges but visually-clean output benefits from acyclic / DAG inputs.`,
			`If the data is genuinely hierarchical, switch to the ` + "`tree`" + ` mark which enforces acyclicity (` + "`PRISM_ENCODE_TREE_CYCLE`" + `).`,
		},
		SeeAlso: []string{"PRISM_ENCODE_TREE_CYCLE"},
	},
	"PRISM_ENCODE_TREE_CYCLE": {
		Code:    "PRISM_ENCODE_TREE_CYCLE",
		Message: `tree mark cannot be laid out: input graph has a cycle.`,
		Fixups: []string{
			`Tree-style marks require a directed acyclic graph rooted at one parentless node. Break the cycle in the upstream data or switch to the ` + "`network`" + ` mark.`,
		},
	},
	"PRISM_ENCODE_NETWORK_NONFINITE": {
		Code:    "PRISM_ENCODE_NETWORK_NONFINITE",
		Message: `network force layout failed to converge: a node position became non-finite (NaN / Inf).`,
		Fixups: []string{
			`Reduce ` + "`mark.charge`" + ` magnitude or shrink ` + "`mark.link_distance`" + `; very large repulsion forces can blow up the gradient.`,
			`Disconnected components without any edges can also slip into Inf — keep at least one edge per component.`,
		},
	},
	"PRISM_SPEC_027": {
		Code:    "PRISM_SPEC_027",
		Message: `Condition entry on channel {{.Channel}} must carry exactly one of value or field (got: {{.Got}}).`,
		Fixups: []string{
			`Set ` + "`value`" + ` for a literal applied when the condition matches (e.g. ` + "`{\"selection\":\"brush\",\"value\":\"#22c55e\"}`" + `).`,
			`Set ` + "`field`" + ` (+ ` + "`type`" + `) to bind the matching rows to a field-driven encoding.`,
			`A selection-form entry without ` + "`value`" + ` is allowed only when no ` + "`field`" + ` is also set — it inherits the channel's own field binding.`,
		},
		SeeAlso: []string{"PRISM_SPEC_025", "PRISM_SPEC_026"},
	},
	"PRISM_WARN_NULL_DROPPED": {
		Code:    "PRISM_WARN_NULL_DROPPED",
		Message: `{{.Count}} rows skipped: encoding channels {{.Channels}} carried null values.`,
		Fixups: []string{
			`Source data had {{.Count}} rows where one or more channel-bound fields were null (often from a left / outer join with no match on the right). Filter or impute those rows upstream to suppress the warning.`,
			`See ` + "`docs/src/concepts/multi-source.md`" + ` for join null semantics.`,
		},
		SeeAlso: []string{"PRISM_JOIN_001"},
	},
	"PRISM_WARN_NULL_AGG_ALL": {
		Code:    "PRISM_WARN_NULL_AGG_ALL",
		Message: `Aggregate {{.Op}} over field {{.Field}} produced a null result: every input row was null.`,
		Fixups: []string{
			`The group has no non-null values for ` + "`{{.Field}}`" + `. Filter the empty group upstream or supply a default via a calculate transform.`,
		},
		SeeAlso: []string{"PRISM_WARN_NULL_DROPPED"},
	},
	"PRISM_WARN_ANIM_FALLBACK": {
		Code:    "PRISM_WARN_ANIM_FALLBACK",
		Message: `animation skipped: {{.Reason}}.`,
		Fixups: []string{
			`Animation only runs when successive scenes share the same composition shape (layer count, mark families, axis types). Structural changes snap to the new scene instantly.`,
			`Set ` + "`animation.enter`" + ` and ` + "`animation.exit`" + ` to ` + "`none`" + ` to suppress the fade on first render.`,
		},
		SeeAlso: []string{"PRISM_SPEC_023"},
	},
}

// CodesSorted returns the catalog keys in ascending order.
func CodesSorted() []string {
	out := make([]string, 0, len(Codes))
	for k := range Codes {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// formatFixups expands each fixup template against ctx. A template that
// fails to render falls back to the literal template string so callers
// always see *some* hint rather than a missing line.
func formatFixups(templates []string, ctx map[string]any) []string {
	if len(templates) == 0 {
		return nil
	}
	out := make([]string, 0, len(templates))
	for i, tpl := range templates {
		out = append(out, renderTemplate("fixup_"+itoa(i), tpl, ctx))
	}
	return out
}

// RenderMessage expands a code's Message template against ctx. Exposed
// for callers that want to surface the canonical message without
// constructing a full AppError.
func RenderMessage(code string, ctx map[string]any) string {
	meta, ok := Codes[code]
	if !ok {
		return code
	}
	return renderTemplate("msg_"+code, meta.Message, ctx)
}

// renderTemplate expands a fixup / message body against ctx using a
// small, non-reflective interpolator instead of text/template. TinyGo
// does not implement reflect.Value.MethodByName, which text/template's
// Execute reaches even for the trivial `{{.Field}}` actions used here —
// so under TinyGo every error path that rendered a fixup crashed the
// wasm with "RuntimeError: unreachable" and no envelope ever reached JS.
//
// The interpolator recognises exactly the action forms the catalog
// uses and reproduces text/template's `missingkey=zero` behaviour over
// a map[string]any byte-for-byte:
//
//   - {{.Ident}} / {{ .Ident }} — substitute ctx["Ident"]; a missing or
//     nil value renders as the literal "<no value>" (the string
//     text/template + missingkey=zero emits for a nil interface).
//   - {{if .Ident}} … {{end}} / spaced forms — emit the body only when
//     ctx["Ident"] is "true" by text/template's isTrue rules (non-empty
//     string / non-zero number / true / non-empty collection); missing
//     or nil is false. Frames nest via a stack.
//
// name is retained for signature stability with the previous
// text/template-backed helper; it is not otherwise consulted.
func renderTemplate(name, body string, ctx map[string]any) string {
	if !strings.Contains(body, "{{") {
		return body
	}

	var sb strings.Builder
	// frames holds one bool per open {{if}}: whether that frame (given
	// all its parents) is currently emitting. active() is the effective
	// emit state.
	var frames []bool
	active := func() bool {
		if len(frames) == 0 {
			return true
		}
		return frames[len(frames)-1]
	}

	i := 0
	for i < len(body) {
		open := strings.Index(body[i:], "{{")
		if open < 0 {
			if active() {
				sb.WriteString(body[i:])
			}
			break
		}
		open += i
		if active() {
			sb.WriteString(body[i:open])
		}

		rel := strings.Index(body[open+2:], "}}")
		if rel < 0 {
			// Unterminated action: emit the remainder verbatim.
			if active() {
				sb.WriteString(body[open:])
			}
			break
		}
		closeIdx := open + 2 + rel
		action := strings.TrimSpace(body[open+2 : closeIdx])
		i = closeIdx + 2

		switch {
		case action == "end":
			if len(frames) > 0 {
				frames = frames[:len(frames)-1]
			}
		case action == "if" || strings.HasPrefix(action, "if "):
			parent := active()
			arg := strings.TrimSpace(strings.TrimPrefix(action, "if"))
			v, _ := lookupField(ctx, arg)
			frames = append(frames, parent && truthy(v))
		case strings.HasPrefix(action, "."):
			if active() {
				v, ok := lookupField(ctx, action)
				sb.WriteString(formatValue(v, ok))
			}
		default:
			// Unrecognised action: reproduce it verbatim so nothing is
			// silently dropped (matches the old fall-back-to-body intent).
			if active() {
				sb.WriteString(body[open : closeIdx+2])
			}
		}
	}
	return sb.String()
}

// lookupField resolves a `.Ident` reference against ctx, returning the
// value and whether the key was present.
func lookupField(ctx map[string]any, ref string) (any, bool) {
	key := strings.TrimPrefix(ref, ".")
	v, ok := ctx[key]
	return v, ok
}

// formatValue renders a substituted value the way text/template +
// missingkey=zero does: a missing key or a nil value becomes the
// literal "<no value>"; anything else is printed with %v (which matches
// text/template's default fmt-backed formatting for the scalar and
// slice values the catalog carries).
func formatValue(v any, ok bool) string {
	if !ok || v == nil {
		return "<no value>"
	}
	return fmt.Sprintf("%v", v)
}

// truthy mirrors text/template's isTrue for the {{if}} guard: the zero
// value of a type is false, everything else true. Reflection here is
// limited to Kind/Len/Bool/Int/Uint/Float/IsNil (all TinyGo-supported);
// it never reaches MethodByName.
func truthy(v any) bool {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return false
	}
	switch rv.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len() > 0
	case reflect.Bool:
		return rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() != 0
	case reflect.Complex64, reflect.Complex128:
		return rv.Complex() != 0
	case reflect.Chan, reflect.Func, reflect.Ptr, reflect.Interface:
		return !rv.IsNil()
	default:
		return true
	}
}

// itoa is a tiny inline integer formatter; avoids importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
