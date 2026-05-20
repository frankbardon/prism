package errors

import (
	"bytes"
	"sort"
	"text/template"
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
	"PRISM_SPEC_006": {
		Code:    "PRISM_SPEC_006",
		Message: `Expression failed to parse: {{.Reason}}.`,
		Fixups: []string{
			`Check Pulse expression syntax. Expression: {{.Expression}}`,
			`Quote string literals with single quotes ('value'), not double quotes.`,
			`Use Pulse expression operators (and, or, not, ==, !=, <, <=, >, >=, +, -, *, /, %).`,
		},
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
		Message: `Expression failed at runtime: {{.Reason}}.`,
		Fixups: []string{
			`Expression: {{.Expression}} (site: {{.Site}}).`,
			`Run ` + "`prism validate`" + ` first — most parse errors surface as PRISM_SPEC_006 before they reach the compiler.`,
			`Check field references match the upstream schema and that arithmetic does not divide by a possibly-zero value.`,
		},
		SeeAlso: []string{"PRISM_SPEC_006"},
	},
	"PRISM_COMPILE_003": {
		Code:    "PRISM_COMPILE_003",
		Message: `Aggregate alias {{.Alias}} is not yet supported by backend {{.Backend}}.`,
		Fixups: []string{
			`Use a supported alias: count, sum, mean, median, min, max, stdev, variance, mode, distinct, q1, q3, ci0, ci1, wmean, ratio, lift, share.`,
			`If your spec relied on an upstream alias the planner forwarded, check ` + "`compile/aggregates.go`" + ` for the canonical alias-to-Pulse mapping.`,
			`File an issue with the alias name so it can be added to the next Pulse release.`,
		},
		SeeAlso: []string{"PRISM_SPEC_002"},
	},
	"PRISM_COMPILE_004": {
		Code:    "PRISM_COMPILE_004",
		Message: `Inline data is not supported by the Pulse backend for node {{.NodeType}}: {{.Reason}}.`,
		Fixups: []string{
			`The Pulse v0.8.4 facade does not expose an in-memory cohort constructor; inline data flows through the in-memory backend.`,
			`Materialise the inline values to a ` + "`.pulse`" + ` file via ` + "`prism import`" + ` (post-P02) and reference it as a source.`,
			`Track the upstream phase: in-memory Pulse cohorts land when Pulse exposes pulse.FromTable / pulse.NewMemory (no ETA).`,
		},
	},
	"PRISM_RESOLVE_001": {
		Code:    "PRISM_RESOLVE_001",
		Message: `Dataset {{.Dataset}} not found in any registered source.`,
		Fixups: []string{
			`Verify the source path or cohort id.`,
			`Add the dataset to "datasets" or to the prism serve config.`,
		},
	},
	"PRISM_RESOLVE_002": {
		Code:    "PRISM_RESOLVE_002",
		Message: `Local .pulse file {{.Path}} not found on the configured filesystem.`,
		Fixups: []string{
			`Check the path spelling and that the file exists (` + "`ls -lh {{.Path}}`" + `).`,
			`Confirm the working directory matches what the spec assumes — relative paths are resolved against the process cwd unless an afero.Fs jail is in effect.`,
			`If the data lives in an archive, use the anchor form: ` + "`archive.pulse#shard.pulse`" + `.`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_003", "PRISM_RESOLVE_005"},
	},
	"PRISM_RESOLVE_003": {
		Code:    "PRISM_RESOLVE_003",
		Message: `Shard {{.Shard}} not present in archive {{.Archive}}.`,
		Fixups: []string{
			`Run ` + "`prism inspect {{.Archive}}`" + ` to list shard names (basenames only; no path).`,
			`Anchors are case-sensitive; copy the basename verbatim from the archive listing.`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_002"},
	},
	"PRISM_RESOLVE_004": {
		Code:    "PRISM_RESOLVE_004",
		Message: `Cohort id {{.Id}} is not registered in the active resolver registry.`,
		Fixups: []string{
			`Register the id with the resolver's Registry before resolving (` + "`registry.Lookup(\"{{.Id}}\")`" + `).`,
			`If you intended to load a file directly, drop the ` + "`cohort:`" + ` prefix and use the path form.`,
		},
	},
	"PRISM_RESOLVE_005": {
		Code:    "PRISM_RESOLVE_005",
		Message: `Reference {{.Ref}} does not match any known form (path, archive#shard, gs://, or cohort:id).`,
		Fixups: []string{
			`Use one of: ` + "`cohort.pulse`" + `, ` + "`archive.pulse#shard.pulse`" + `, ` + "`gs://bucket/path.pulse`" + `, ` + "`cohort:<id>`" + `.`,
			`Drop trailing whitespace and double-check for leading slashes that imply absolute paths.`,
		},
	},
	"PRISM_RESOLVE_006": {
		Code:    "PRISM_RESOLVE_006",
		Message: `Pulse failed to open {{.Ref}}: {{.Reason}}.`,
		Fixups: []string{
			`Run ` + "`prism inspect {{.Ref}}`" + ` for header diagnostics.`,
			`Verify the file is a real .pulse (the first 8 bytes spell ` + "`PULSE\\x00\\x00\\x00`" + `).`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_002", "PRISM_RESOLVE_003"},
	},
	"PRISM_RESOLVE_007": {
		Code:    "PRISM_RESOLVE_007",
		Message: `Materialisation refused: {{.Actual}} rows would exceed PRISM_TABLE_MAX_ROWS={{.Limit}}.`,
		Fixups: []string{
			`Raise the ceiling by setting ` + "`PRISM_TABLE_MAX_ROWS`" + ` in the environment before running prism.`,
			`Pre-aggregate, sample, or filter at the Pulse layer to bring the result under the cap.`,
			`Switch to a streaming consumer once P03 lands streaming; for v1 every node materialises a Table.`,
		},
	},
	"PRISM_RESOLVE_GCS_UNAVAILABLE": {
		Code:    "PRISM_RESOLVE_GCS_UNAVAILABLE",
		Message: `gs:// references are not implemented in v1 (ref: {{.Ref}}).`,
		Fixups: []string{
			`Stage the .pulse locally (` + "`gsutil cp gs://bucket/path.pulse ./`" + `) and reference the local path.`,
			`Track the upstream phase: gs:// support lands once Pulse ships a generic GCS afero.Fs (planned P-NN-gcs-fs).`,
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
		Message: `Render format {{.Format}} is not available in the current Prism build (lands in {{.Phase}}).`,
		Fixups: []string{
			`SVG (default) and PDF (P15) are the available renderers; use --format svg or --format pdf.`,
			`PNG support is deferred to V2; consume the JS port (prism.mjs) via prism scene + canvas for browser-native screenshots.`,
			`canvas-json consumes the Scene IR directly via 'prism scene <spec>' → render/svg's prism.mjs in the browser.`,
		},
		SeeAlso: []string{"PRISM_RENDER_001"},
	},
	"PRISM_RENDER_PDF_UNSUPPORTED_PATH": {
		Code:    "PRISM_RENDER_PDF_UNSUPPORTED_PATH",
		Message: `PDF renderer cannot translate SVG path command {{.Got}} (only M/L/H/V/Q/C/A/Z + relative forms are supported per D092).`,
		Fixups: []string{
			`Rewrite the path using only the supported subset: M / L / H / V / Q / C / A / Z (and the relative forms m / l / h / v / q / c / a / z).`,
			`Smooth cubic (S / s) and smooth quadratic (T / t) are rejected because they depend on the previous command's reflected control point; expand them to explicit C / Q commands.`,
			`If you need an arbitrary SVG shape, consider using a primitive Prism mark (rect / line / area / arc) instead of a raw <path>.`,
		},
		SeeAlso: []string{"PRISM_SPEC_017", "PRISM_RENDER_001"},
	},
	"PRISM_WARN_PDF_GRADIENT_FLATTENED": {
		Code:    "PRISM_WARN_PDF_GRADIENT_FLATTENED",
		Message: `PDF renderer flattened a gradient fill to its first color stop (gradient {{.Gradient}}).`,
		Fixups: []string{
			`Use a solid color in your spec for byte-identical PDF rendering; gradient support arrives in V2 (D091).`,
			`The SVG renderer preserves the gradient; only PDF flattens.`,
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
			`Cast the column on one side via a calculate transform so both sides share a Pulse Kind.`,
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
			`Push the join down to Pulse once Pulse exposes a relational join (deferred to a future Prism phase).`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_007"},
	},
	"PRISM_PLAN_004": {
		Code:    "PRISM_PLAN_004",
		Message: `Union input schemas disagree: {{.Diff}}.`,
		Fixups: []string{
			`Make every union input expose the same column names and Pulse types in the same order.`,
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
			`Switch the channel to a Pulse-compatible measure type so every layer publishes the same scale family.`,
			"Set `resolve.scale.{{.Channel}}` to `independent` to keep per-layer scales + per-layer axes.",
		},
		SeeAlso: []string{"PRISM_PLAN_002", "PRISM_SPEC_007", "PRISM_RESOLVE_DUPLICATE_DATASET"},
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
			`If the dataset lives behind an authentication wall, expose it through a proxy that adds the credentials before the browser hits it.`,
			`For local development serve the .pulse files via a static file server (e.g. ` + "`python -m http.server`" + `) rather than file:// URLs — fetch refuses file:// in most browsers.`,
		},
		SeeAlso: []string{"PRISM_RESOLVE_002", "PRISM_WASM_002"},
	},
	"PRISM_WASM_002": {
		Code:    "PRISM_WASM_002",
		Message: `Origin server for {{.URL}} does not honour Range: requests (status {{.Status}}); archive-shard random access is unavailable.`,
		Fixups: []string{
			`Serve archive shards from a static host that returns 206 Partial Content for Range requests (GitHub Pages, S3, Cloudflare R2, nginx with default config all do).`,
			`If random access is impossible, materialise individual shards as standalone .pulse files at build time and reference them directly.`,
			`Disable archive forms in the spec — load each shard via its own ` + "`<prism-dataset>`" + ` registration.`,
		},
		SeeAlso: []string{"PRISM_WASM_001", "PRISM_RESOLVE_003"},
	},
	"PRISM_WASM_BUDGET_EXCEEDED": {
		Code:    "PRISM_WASM_BUDGET_EXCEEDED",
		Message: `Compiled prism.wasm exceeds PRISM_WASM_MAX_BYTES={{.Limit}} (gzipped size: {{.Actual}}).`,
		Fixups: []string{
			`Raise the ceiling by setting ` + "`PRISM_WASM_MAX_BYTES`" + ` in the environment before running ` + "`make build-wasm`" + `.`,
			`Drop newly-imported dependencies from the WASM entry — confirm cmd/prismwasm/main.go imports only library packages buildable under js,wasm.`,
			`Check ` + "`go list -deps ./cmd/prismwasm | sort | uniq`" + ` for transitive imports that bloat the binary (apache/arrow-go and gonum dominate).`,
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

func renderTemplate(name, body string, ctx map[string]any) string {
	tpl, err := template.New(name).Option("missingkey=zero").Parse(body)
	if err != nil {
		return body
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ctx); err != nil {
		return body
	}
	return buf.String()
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
