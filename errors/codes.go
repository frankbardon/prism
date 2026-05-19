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

	// --- Initial codes for downstream phases (smoke-only; deeper rules land later).
	"PRISM_PLAN_001": {
		Code:    "PRISM_PLAN_001",
		Message: `Cyclic dataset reference involving {{.Dataset}}.`,
		Fixups: []string{
			`Break the cycle by introducing an intermediate named alias.`,
			`Check transform "data" and "as" aliases for accidental loops.`,
		},
	},
	"PRISM_PLAN_002": {
		Code:    "PRISM_PLAN_002",
		Message: `Transform references undefined dataset {{.Dataset}}.`,
		Fixups: []string{
			`Declare the dataset in "datasets" or earlier in the transform pipeline.`,
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
	"PRISM_RENDER_001": {
		Code:    "PRISM_RENDER_001",
		Message: `Mark geometry is malformed for {{.Mark}}.`,
		Fixups: []string{
			`Inspect the encoding values driving this mark — non-finite or null values often cause this.`,
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
