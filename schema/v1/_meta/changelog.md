# Prism Schema v1 Changelog

All notable changes to the Prism JSON Schema bundle are recorded here.
The schema URN namespace is `urn:prism:schema:v1:*`.

## v1.0.0 — initial release

Initial 14-file schema bundle for Prism v1 specs.

- `spec.schema.json` — recursive top-level spec entry point.
- `data.schema.json` — `data` block and `datasets` map.
- `transform.schema.json` — 12 transform variants and `aggregate_op` enum.
- `mark.schema.json` — 20 mark types and shared mark properties.
- `encoding.schema.json` — 17 encoding channels and per-class definitions.
- `scale.schema.json` — scale types, domain, range, scheme.
- `axis.schema.json` — axis configuration.
- `legend.schema.json` — legend configuration.
- `composition.schema.json` — layer, concat, hconcat, vconcat, facet, repeat.
- `selection.schema.json` — point and interval selections.
- `resolve.schema.json` — cross-layer scale/axis/legend resolution.
- `theme.schema.json` — spec-level sparse theme overrides.
- `shared.schema.json` — shared primitives reused across files.

Format constraints (per DECISIONS D013, D014, D019):

- All `$id`s use URN form `urn:prism:schema:v1:<name>`.
- All `$ref`s between schemas are relative paths (`name.schema.json#/$defs/...`).
- All field names are snake_case.
- All objects set `additionalProperties: false`.
- All fields carry a `description`.
