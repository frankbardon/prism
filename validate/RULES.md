# Prism Spec Validation — Rules + Layering

## Validation layering

Prism validates a spec in two stages, in this order:

1. **Shape validation** (`validate/shape.go`) — JSON Schema check using the
   embedded v1 bundle. Catches: unknown fields, missing required fields,
   wrong types, oneOf misses, enum violations, pattern mismatches.
2. **Semantic validation** (`validate/semantic.go`) — Go-side rules that
   need awareness of the data source schema, Pulse expressions, or
   cross-field invariants. Returns `PRISM_SPEC_*` errors with fixup
   metadata.

A spec that fails shape never reaches the semantic stage; many semantic
rules assume well-formed structure.

## Why each rule lives where it does

| Rule | Layer | Why |
|---|---|---|
| Unknown fields | Shape | JSON Schema `additionalProperties: false`. |
| Missing required fields | Shape | JSON Schema `required`. |
| Enum membership (mark types, agg ops, scale types) | Shape | JSON Schema `enum`. |
| Numeric ranges (opacity 0..1, dimension ≥ 0) | Shape | JSON Schema `minimum`/`maximum`. |
| Pattern (snake_case dataset names) | Shape | JSON Schema `pattern`. |
| `oneOf` between composition keys | Shape | JSON Schema top-level `oneOf`. |
| `PRISM_SPEC_001` field exists | Semantic | Needs Pulse schema lookup. |
| `PRISM_SPEC_002` agg / field type compat | Semantic | Needs Pulse schema lookup. |
| `PRISM_SPEC_003` channel valid for mark | Semantic | Cross-field; per-mark allowlist table. |
| `PRISM_SPEC_004` selection ref resolves | Semantic | Cross-field check inside the spec. |
| `PRISM_SPEC_005` dataset ref resolves | Semantic | Cross-field check inside the spec. |
| `PRISM_SPEC_006` expression parses | Semantic | Needs Pulse expression parser. |
| `PRISM_SPEC_007` scale type compat with field type | Semantic | Needs field type lookup. |
| `PRISM_SPEC_008` pie/donut requires theta + color | Semantic | Cross-field rule per mark type. |
| `PRISM_SPEC_009` `$schema` references known schema | Semantic | Requires the bundle URN registry. |
| `PRISM_SPEC_019` selection encoding channel is bound | Semantic | Cross-field check: walks selection.encodings and matches against bound channels. |
| `PRISM_SPEC_020` interval selection encodings are position channels | Semantic | Spec-internal allowlist (x/y/x2/y2/theta). |
| `PRISM_SPEC_021` geo projection / geo-mark channel bindings | Semantic | Cross-field rule for geoshape + geopoint; ensures `projection.type` is known and the matching encoding channels are bound. |
| `PRISM_SPEC_022` animation easing name is known | Semantic | Enum check against `spec.AnimationEasings`. Lives at the semantic layer so the error message can suggest the full easing list. |
| `PRISM_SPEC_023` animation declares a join key | Semantic | Cross-field rule: when `animation` is set, at least one descendant encoding channel must carry `key: true`. |
| `PRISM_SPEC_024` animation join key is unique | Semantic | Cross-field rule: at most one encoding channel per spec node may carry `key: true`. Composite keys deferred. |

## $ref resolution strategy

Each schema file declares `$id: urn:prism:schema:v1:<name>`. Cross-file
`$ref`s are authored as relative paths (`data.schema.json#/$defs/data`)
per the format rules in `.planning/design/03-spec-format.md`.

Because `santhosh-tekuri/jsonschema/v6` resolves relative refs against the
document's `$id`, relative refs cannot resolve naturally when `$id` is a
URN — there is no notion of "next to" within the `urn:` namespace.

Choice: at compile time, the validator walks each loaded schema document
and rewrites every relative `$ref` (`name.schema.json[#…]`) to its URN
form (`urn:prism:schema:v1:name[#…]`). Intra-file refs (`#/$defs/x`) and
refs already in URN form are left untouched. The on-disk source keeps the
human-friendly relative-ref form; only the in-memory compile graph uses
URNs.

This keeps:

- The on-disk schemas portable across editors that resolve relative refs.
- The runtime resolver consistent: every cross-file reference becomes a
  URN before the JSON Schema engine sees it.

Alternative considered: register each file under a synthetic
`https://prism.local/v1/<name>.schema.json` URL and resolve relative refs
that way. Rejected because it would diverge from the design-doc rule
that `$id` is the canonical URN.

## Adding a new semantic rule

1. Add a file `validate/rules/<short_name>.go` implementing `SemanticRule`.
2. Register it in `defaultRules()` in `validate/semantic.go`.
3. Add a `PRISM_SPEC_xxx` entry in `errors/codes.go` with `Message`,
   `Fixups`, and any `SeeAlso` links.
4. Add a positive + negative fixture under `testdata/specs/` and
   `testdata/specs/invalid/`.
5. Update the table above.
