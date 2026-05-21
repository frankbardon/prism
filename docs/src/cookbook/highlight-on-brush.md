# Cookbook: highlight bars on a brush selection

Switch a channel's value live as a selection becomes active. Useful
for emphasising the current focus without re-rendering or filtering
the underlying data.

## Spec

```json
{
  "$schema": "urn:prism:schema:v1:spec",
  "data": {
    "name": "regional_scores",
    "values": [
      {"region": "west",    "score": 0.42},
      {"region": "east",    "score": 0.91},
      {"region": "north",   "score": 0.68},
      {"region": "south",   "score": 0.55},
      {"region": "central", "score": 0.73}
    ]
  },
  "selection": {
    "brush": {"type": "interval", "encodings": ["x"]}
  },
  "mark": "bar",
  "encoding": {
    "x": {"field": "region", "type": "nominal"},
    "y": {"field": "score",  "type": "quantitative"},
    "color": {
      "condition": [
        {"selection": "brush", "value": "#22c55e"}
      ],
      "value": "#cbd5e1"
    }
  }
}
```

## How it works

1. The `selection.brush` block declares an interval brush on the x
   axis.
2. The `color.condition` entry references that selection by name. As
   long as the brush is active, every mark switches its fill to
   `#22c55e`; when the user clears the brush, the channel reverts to
   `#cbd5e1` (the `value` fallback).
3. The encoder resolves both branches up front and attaches a
   `Mark.Conditions[]` entry to each row's scene-IR mark. The
   browser-side `prism-selection` module flips the matching SVG
   attribute when the selection state changes — no re-encode, no
   network round-trip.

## Variations

- **Test-driven highlights** (no selection): swap the
  `{"selection": ...}` entry for `{"test": "score >= 0.7", "value":
  "#22c55e"}`. The encoder evaluates the expression at encode time
  and bakes the result into each mark's style, so the highlight
  appears in plain SVG / PDF output without any browser involvement.
- **Multiple conditions**: combine entries. Entries evaluate
  top-down, first match wins; the channel's own `value` is the
  final fallback.

## Caveats

- PDF cannot honour selection-driven conditions (it's a static
  format) — the renderer emits a
  `PRISM_WARN_PDF_CONDITION_FLATTENED` warning and paints the
  fallback. Static `test` conditions render fine.
- Each entry must carry exactly one of `value` or `field`
  (`PRISM_SPEC_027`); a `selection`-form entry with neither
  inherits the channel's own field binding.

## See also

- [Encoding › Conditions](../concepts/encoding.md#conditions)
- [Selections](../concepts/selections.md)
- [conditions gallery](../gallery/conditions/)
