# Encoding

The `encoding` object binds data fields to visual channels.

## Channels

| Family | Channels |
|---|---|
| Position | `x`, `y`, `x2`, `y2`, `theta`, `theta2`, `radius`, `radius2` |
| Color & opacity | `color`, `fill`, `stroke`, `opacity` |
| Size & shape | `size`, `shape` |
| Text & order | `text`, `tooltip`, `order`, `detail` |
| Facet | `row`, `column` |
| Sankey | `source`, `target`, `value` |

## Channel shape

```json
"x": {
  "field": "score",
  "type": "quantitative",
  "aggregate": "mean",
  "scale": {"type": "log"},
  "axis": {"title": "Average score", "format": ".2f"},
  "sort": "-y"
}
```

| Key | Purpose |
|---|---|
| `field` | Column from the source (or transform output). |
| `type` | One of `nominal`, `ordinal`, `quantitative`, `temporal`. |
| `aggregate` | Friendly alias: `mean`, `sum`, `count`, `median`, `q1`, `q3`, `min`, `max`, `stdev`, `variance`, `ci0`, `ci1`, `distinct`, `mode`, plus `wmean`, `ratio`, `lift`, `share`. |
| `scale` | Scale spec (`type`, `domain`, `range`, `scheme`, `padding`, ...). |
| `axis` | Axis config (`title`, `format`, `grid`, `tick_count`, `label_angle`, ...). |
| `legend` | Legend config (`title`, `orient`, `direction`, ...). |
| `format` | d3-format string for label formatting. |
| `sort` | `"ascending"` / `"descending"` / `"-y"` / `[explicit, order, ...]`. |

## Scales

Eight types: `linear` (default for quantitative), `log`, `pow`, `sqrt`,
`time` (default for temporal), `band` (default for nominal bar x),
`point` (default for nominal point x), `ordinal` (default for color
over nominal).

See the [scales gallery](../gallery/scales) for one fixture per type.

## Axes & legends

Both are auto-generated based on the encoded channels but can be
overridden per channel. Bundled support: 4 orientations
(bottom/left/top/right), major + minor ticks, grid toggle, label
rotation, overlap handling, gradient + symbol legends.

## Tooltip channel

```json
"tooltip": [
  {"field": "brand_id"},
  {"field": "score", "format": ".2f"}
]
```

Materialized in the Scene IR as pre-formatted `TooltipLine` lists.
SVG emits `<title>` per mark; the JS port renders rich HTML tooltips
in P12+.

## Further reading

- [Spec field reference](../reference/spec.md) — every channel
  property exhaustively.
- [Themes](themes.md) — how scale color schemes resolve.
