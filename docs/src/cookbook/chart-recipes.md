# Cookbook: chart recipes (Go)

The `recipes` package (`github.com/frankbardon/prism/recipes`) builds
Prism specs for common analysis charts from tidy rows. A recipe takes
`[]recipes.Row` (one `map[string]any` per observation) plus an options
struct naming the columns, and returns a `*spec.Spec` with the rows
inlined in `data.values`. The result is a plain spec: validate, compile
and render it like any other, patch it, or marshal it to JSON for the
browser bundle.

Recipes hold no state and know nothing about where the rows came from.
Anything domain-specific — which column is an estimate, what counts as
significant, how a value is labelled — is the caller's and arrives as a
column or an option.

## Vocabulary

| Type | Meaning |
|---|---|
| `Field{Name, Title, Format}` | A column. `Title` labels its axis, legend or tooltip line (empty reads as `Name`); `Format` is a d3-format specifier (`.3~g`, `,.0f`, `.1%`) for its tooltip line and any printed label. `recipes.F(name)` is the bare form. |
| `Frame{Title, Width, Height}` | Chart title and size. A zero size leaves the choice to the recipe (the renderer's width; the recipe's height). |
| `Color{Field, Domain, Scheme, Range, HideLegend}` | A colour-by column. `Domain` fixes category order (palette and legend follow it), `Scheme` names a palette, `Range` lists explicit colours. |
| `BandHeight(n)` | Default height for `n` category bands (`n·RowHeight + PanelChrome`). |
| `Extent(rows, cols...)` | Shared `[lo, hi]` over columns — pin it as `Domain` on every small multiple. |

Category columns are drawn in the given order (first at the left, or at
the top of a vertical list), else in first-seen row order, and are
stringified so numeric keys still form bands. A recipe returns
`recipes.ErrNoRows` when there is nothing to draw and an error naming
the option when a required column is unset.

## Recipes

| Recipe | Draws |
|---|---|
| `Forest(rows, ForestOptions)` | One row per label: interval rule `Low`→`High`, point at `Estimate`, dashed `References` (e.g. 0 or 1). Optional `Color` by a category. |
| `Bar(rows, BarOptions)` | One bar per category, optional `Low`/`High` interval rule, `Horizontal`. `Start` makes each bar a span from that column to `Value` (a range; with a series `Color`, a bullet's observed bar and expected tick). `Bands` (`Band{Low, High, Opacity}`) shade a value span behind every category, `References` draw dashed value rules over the bars, `Labels` / `LabelField` letter each bar at its end. |
| `GroupedBar(rows, GroupedBarOptions)` | Bars dodged by `Group` within each `Category` (`x_offset` / `y_offset`), coloured and legended by group. |
| `Line(rows, LineOptions)` | Lines over `X` (`XType` temporal / quantitative / ordinal), one per `Series`, optional `Points`. |
| `Scatter(rows, ScatterOptions)` | Points with optional `Color`, pinned domains, and `References`: `RuleX`, `RuleY`, `Polyline` (a fitted or reference curve). |
| `Heatmap(rows, HeatmapOptions)` | One cell per (`X`, `Y`) shaded by `Value`; `Domain` pins the ramp (`[-1, 1]` for a correlation), `Reverse` flips it, `Labels` prints each value (or `LabelField`); `LabelRows` letters only the named `Y` rows (a key row under a grid too large to letter), leaving the other cells plain and opaque. |
| `Box(rows, BoxOptions)` | Horizontal summary boxes from **precomputed** statistics: whisker rule, interquartile bar, centre dot. |
| `Tiles(rows, TilesOptions)` | One big number per row, labelled above it. |

## Composition

| Helper | Builds |
|---|---|
| `Stack(panels...)` | A `vconcat`; each panel keeps its height, the stack is their sum. |
| `Grid(columns, cells...)` | A `concat` wrapped at `columns` (see [Composition](../concepts/composition.md)). |
| `SmallMultiples(rows, by, order, columns, cell)` | Splits rows by a column and lays one cell per value out in a `Grid`; each cell is titled with its key unless `cell` set a title. |
| `Split(rows, col, order)` | The grouping `SmallMultiples` uses, for callers building cells themselves. |

Recipes compose through concat rather than `facet`, so each cell may be
a layered recipe (a forest is three layers) and keeps its own scales;
pin a shared domain with `Extent` when cells should compare at a glance.

## Quantile helpers

`NormalQuantile(p)` (Φ⁻¹, AS241), `PPoints(n)` (R's `ppoints`),
`Quantile(sorted, p)` (type 7) and `NormalQQ(sample)` give the geometry
of a normal QQ plot: the theoretical quantile for each sorted value, the
index of each in the input (so tooltips can carry other columns), and
the quartile line's intercept and slope for a `Polyline`. Model
statistics — residuals, leverage, influence contours, which points to
thin — stay with the caller.

## Example

```go
rows := []recipes.Row{
	{"term": "Age", "est": 0.42, "lo": 0.10, "hi": 0.74},
	{"term": "Income", "est": -0.18, "lo": -0.40, "hi": 0.04},
}
s, err := recipes.Forest(rows, recipes.ForestOptions{
	Label:      recipes.Field{Name: "term", Title: "Term"},
	Estimate:   recipes.Field{Name: "est", Title: "Estimate", Format: ".3~g"},
	Low:        "lo",
	High:       "hi",
	References: []float64{0},
	Tooltip:    []recipes.Field{{Name: "term"}, {Name: "est", Format: ".3~g"}},
	Frame:      recipes.Frame{Title: "Coefficients"},
})
if err != nil {
	return err
}
raw, _ := json.Marshal(s)
plan, err := prism.CompileJSON(ctx, raw, prism.CompileOptions{})
```

Set `s.Theme = &spec.ThemeOverride{DarkVariant: "dark"}` on the root to
render one SVG that follows the page's colour scheme; layered and
concatenated recipes pair their marks and legends like a single chart
(see [Themes](../concepts/themes.md)).

Goldens for every recipe live in `recipes/testdata/` (`<name>.spec.json`
plus `<name>.svg`; regenerate with `UPDATE_GOLDENS=1 go test ./recipes/`).
