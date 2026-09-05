# Prism Gallery

95 fixture specs across 16 categories. Each entry pairs a `*.prism.json`
spec with a rendered `*.svg`. Browse the source to learn the spec
shapes; open the SVGs to see what they render. The `table/` fixtures
are one exception — a top-level `table` mark renders only through
the `html` backend, so those entries link to a rendered `*.html` file
instead of an `<img>` preview. The [Custom marks](#custom-marks)
section is the other exception, and has no `*.prism.json` at all — see
that section for why.

For live interactive rendering in a browser, see [`live.html`](live.html).

## Basic marks

| Spec | Preview |
|---|---|
| [bar_basic](basic-marks/bar_basic.prism.json) | <img src="basic-marks/bar_basic.svg" width="240"> |
| [line_basic](basic-marks/line_basic.prism.json) | <img src="basic-marks/line_basic.svg" width="240"> |
| [area_basic](basic-marks/area_basic.prism.json) | <img src="basic-marks/area_basic.svg" width="240"> |
| [area_with_negatives](basic-marks/area_with_negatives.prism.json) | <img src="basic-marks/area_with_negatives.svg" width="240"> |
| [multi_series_area](basic-marks/multi_series_area.prism.json) | <img src="basic-marks/multi_series_area.svg" width="240"> |
| [point_scatter](basic-marks/point_scatter.prism.json) | <img src="basic-marks/point_scatter.svg" width="240"> |
| [rule_basic](basic-marks/rule_basic.prism.json) | <img src="basic-marks/rule_basic.svg" width="240"> |
| [text_basic](basic-marks/text_basic.prism.json) | <img src="basic-marks/text_basic.svg" width="240"> |
| [tick_strip](basic-marks/tick_strip.prism.json) | <img src="basic-marks/tick_strip.svg" width="240"> |
| [arc_basic](basic-marks/arc_basic.prism.json) | <img src="basic-marks/arc_basic.svg" width="240"> |
| [rect_heatmap_lite](basic-marks/rect_heatmap_lite.prism.json) | <img src="basic-marks/rect_heatmap_lite.svg" width="240"> |
| [multi_series_line](basic-marks/multi_series_line.prism.json) | <img src="basic-marks/multi_series_line.svg" width="240"> |

## Composite marks

| Spec | Preview |
|---|---|
| [histogram](composite-marks/histogram.prism.json) | <img src="composite-marks/histogram.svg" width="240"> |
| [histogram_long_tail](composite-marks/histogram_long_tail.prism.json) | <img src="composite-marks/histogram_long_tail.svg" width="240"> |
| [heatmap](composite-marks/heatmap.prism.json) | <img src="composite-marks/heatmap.svg" width="240"> |
| [crosstab_heatmap](composite-marks/crosstab_heatmap.prism.json) | <img src="composite-marks/crosstab_heatmap.svg" width="240"> |
| [crosstab_overlay_share](composite-marks/crosstab_overlay_share.prism.json) | <img src="composite-marks/crosstab_overlay_share.svg" width="240"> |
| [regression_trend](composite-marks/regression_trend.prism.json) | <img src="composite-marks/regression_trend.svg" width="240"> |
| [crosstab_significance_shading](composite-marks/crosstab_significance_shading.prism.json) | <img src="composite-marks/crosstab_significance_shading.svg" width="240"> |
| [boxplot](composite-marks/boxplot.prism.json) | <img src="composite-marks/boxplot.svg" width="240"> |
| [violin_score](composite-marks/violin_score.prism.json) | <img src="composite-marks/violin_score.svg" width="240"> |
| [pie](composite-marks/pie.prism.json) | <img src="composite-marks/pie.svg" width="240"> |
| [donut](composite-marks/donut.prism.json) | <img src="composite-marks/donut.svg" width="240"> |
| [donut_traffic](composite-marks/donut_traffic.prism.json) | <img src="composite-marks/donut_traffic.svg" width="240"> |

## Specialty marks

| Spec | Preview |
|---|---|
| [sankey_user_flow](specialty-marks/sankey_user_flow.prism.json) | <img src="specialty-marks/sankey_user_flow.svg" width="240"> |
| [funnel_signup](specialty-marks/funnel_signup.prism.json) | <img src="specialty-marks/funnel_signup.svg" width="240"> |
| [sparkline_inline](specialty-marks/sparkline_inline.prism.json) | <img src="specialty-marks/sparkline_inline.svg" width="240"> |
| [sparkline_inline_grid](specialty-marks/sparkline_inline_grid.prism.json) | <img src="specialty-marks/sparkline_inline_grid.svg" width="240"> |
| [sparkbar_inline](specialty-marks/sparkbar_inline.prism.json) | <img src="specialty-marks/sparkbar_inline.svg" width="240"> |
| [sparkbar_inline_grid](specialty-marks/sparkbar_inline_grid.prism.json) | <img src="specialty-marks/sparkbar_inline_grid.svg" width="240"> |
| [winloss_streak](specialty-marks/winloss_streak.prism.json) | <img src="specialty-marks/winloss_streak.svg" width="240"> |
| [winloss_streak_grid](specialty-marks/winloss_streak_grid.prism.json) | <img src="specialty-marks/winloss_streak_grid.svg" width="240"> |
| [sparkarea_inline](specialty-marks/sparkarea_inline.prism.json) | <img src="specialty-marks/sparkarea_inline.svg" width="240"> |
| [sparkarea_inline_grid](specialty-marks/sparkarea_inline_grid.prism.json) | <img src="specialty-marks/sparkarea_inline_grid.svg" width="240"> |
| [sparkline_point_last](specialty-marks/sparkline_point_last.prism.json) | <img src="specialty-marks/sparkline_point_last.svg" width="240"> |
| [sparkline_point_extent](specialty-marks/sparkline_point_extent.prism.json) | <img src="specialty-marks/sparkline_point_extent.svg" width="240"> |
| [sparkbar_point_extent](specialty-marks/sparkbar_point_extent.prism.json) | <img src="specialty-marks/sparkbar_point_extent.svg" width="240"> |
| [sparkarea_reference_band](specialty-marks/sparkarea_reference_band.prism.json) | <img src="specialty-marks/sparkarea_reference_band.svg" width="240"> |
| [bullet_revenue](specialty-marks/bullet_revenue.prism.json) | <img src="specialty-marks/bullet_revenue.svg" width="240"> |
| [bullet_satisfaction_vertical](specialty-marks/bullet_satisfaction_vertical.prism.json) | <img src="specialty-marks/bullet_satisfaction_vertical.svg" width="240"> |
| [bullet_pipeline](specialty-marks/bullet_pipeline.prism.json) | <img src="specialty-marks/bullet_pipeline.svg" width="240"> |
| [image_logo](specialty-marks/image_logo.prism.json) | <img src="specialty-marks/image_logo.svg" width="240"> |
| [path_arbitrary](specialty-marks/path_arbitrary.prism.json) | <img src="specialty-marks/path_arbitrary.svg" width="240"> |

## Geographic marks

| Spec | Preview |
|---|---|
| [world_basic](geo/world_basic.prism.json) | <img src="geo/world_basic.svg" width="240"> |
| [world_choropleth](geo/world_choropleth.prism.json) | <img src="geo/world_choropleth.svg" width="240"> |
| [usa_states](geo/usa_states.prism.json) | <img src="geo/usa_states.svg" width="240"> |

## Table

The `table` mark renders as an interactive, paginated HTML `<table>` —
not SVG geometry — so these fixtures link to a rendered `*.html` file
rather than showing an `<img>` preview. See [Marks ›
Table](../concepts/marks.md#table).

| Spec | Preview |
|---|---|
| [table_accounts](table/table_accounts.prism.json) | [table_accounts.html](table/table_accounts.html) — plain columns, paginated (`page_size: 5` over 7 rows) |
| [table_revenue_trend](table/table_revenue_trend.prism.json) | [table_revenue_trend.html](table/table_revenue_trend.html) — a `sparkline` sub-mark column |

## Custom marks

A `custom` mark (`mark: {"type": "custom", "renderer": "…"}`) resolves
its `renderer` name against a Go-level registry (`custommark.Register`)
at render time — a mechanism only a caller-supplied binary can
exercise, not the shared `prism` CLI, which has nothing registered
under any name. That means these two fixtures have **no**
`*.prism.json` companion (unlike every other category on this page,
`table/` included): a spec referencing `quote-card` or `stat-card`
cannot be plotted by `prism plot` as committed. See [Cookbook › Custom
marks: Why no gallery `*.prism.json` for these
two](../cookbook/custom-marks.md#why-no-gallery-prismjson-for-these-two)
for the full explanation, the exact spec JSON that produced each file
below, and the registered `HTMLCustomRenderer` implementation's full
source.

| Example | Preview |
|---|---|
| Quote card ([worked example](../cookbook/custom-marks.md#quote-card)) | [quote_card.html](custom-marks/quote_card.html) — pull-quote + attribution |
| Stat card ([worked example](../cookbook/custom-marks.md#stat-card)) | [stat_card.html](custom-marks/stat_card.html) — metric tile with a label, value, and up/down delta |

## Composition (layer / concat / facet / repeat)

| Spec | Preview |
|---|---|
| [layer_actual_vs_benchmark](composition/layer_actual_vs_benchmark.prism.json) | <img src="composition/layer_actual_vs_benchmark.svg" width="240"> |
| [layer_independent_color](composition/layer_independent_color.prism.json) | <img src="composition/layer_independent_color.svg" width="240"> |
| [layer_shared_y_scale](composition/layer_shared_y_scale.prism.json) | <img src="composition/layer_shared_y_scale.svg" width="240"> |
| [bar_with_benchmark_rule](composition/bar_with_benchmark_rule.prism.json) | <img src="composition/bar_with_benchmark_rule.svg" width="240"> |
| [concat_h](composition/concat_h.prism.json) | <img src="composition/concat_h.svg" width="240"> |
| [concat_v](composition/concat_v.prism.json) | <img src="composition/concat_v.svg" width="240"> |
| [hconcat_two_panels](composition/hconcat_two_panels.prism.json) | <img src="composition/hconcat_two_panels.svg" width="240"> |
| [vconcat_metrics](composition/vconcat_metrics.prism.json) | <img src="composition/vconcat_metrics.svg" width="240"> |
| [facet_by_region](composition/facet_by_region.prism.json) | <img src="composition/facet_by_region.svg" width="240"> |
| [facet_nested](composition/facet_nested.prism.json) | <img src="composition/facet_nested.svg" width="240"> |
| [facet_per_cell_y](composition/facet_per_cell_y.prism.json) | <img src="composition/facet_per_cell_y.svg" width="240"> |
| [repeat_metrics](composition/repeat_metrics.prism.json) | <img src="composition/repeat_metrics.svg" width="240"> |
| [dashboard](composition/dashboard.prism.json) | <img src="composition/dashboard.svg" width="240"> |

## Multi-source

| Spec | Preview |
|---|---|
| [actual_vs_benchmark](multi-source/actual_vs_benchmark.prism.json) | <img src="multi-source/actual_vs_benchmark.svg" width="240"> |
| [multi_source_join](multi-source/multi_source_join.prism.json) | <img src="multi-source/multi_source_join.svg" width="240"> |
| [bar_inline](multi-source/bar_inline.prism.json) | <img src="multi-source/bar_inline.svg" width="240"> |

## Transforms

| Spec | Preview |
|---|---|
| [filter_structured](transforms/filter_structured.prism.json) | <img src="transforms/filter_structured.svg" width="240"> |
| [calculate_structured](transforms/calculate_structured.prism.json) | <img src="transforms/calculate_structured.svg" width="240"> |

## Scales

| Spec | Preview |
|---|---|
| [linear](scales/linear.prism.json) | <img src="scales/linear.svg" width="240"> |
| [log](scales/log.prism.json) | <img src="scales/log.svg" width="240"> |
| [log_revenue](scales/log_revenue.prism.json) | <img src="scales/log_revenue.svg" width="240"> |
| [pow](scales/pow.prism.json) | <img src="scales/pow.svg" width="240"> |
| [sqrt](scales/sqrt.prism.json) | <img src="scales/sqrt.svg" width="240"> |
| [time](scales/time.prism.json) | <img src="scales/time.svg" width="240"> |
| [band](scales/band.prism.json) | <img src="scales/band.svg" width="240"> |
| [point](scales/point.prism.json) | <img src="scales/point.svg" width="240"> |
| [ordinal](scales/ordinal.prism.json) | <img src="scales/ordinal.svg" width="240"> |

## Selections

| Spec | Preview |
|---|---|
| [selection_point](selections/selection_point.prism.json) | <img src="selections/selection_point.svg" width="240"> |
| [selection_interval](selections/selection_interval.prism.json) | <img src="selections/selection_interval.svg" width="240"> |
| [selection_point_bar](selections/selection_point_bar.prism.json) | <img src="selections/selection_point_bar.svg" width="240"> |
| [selection_interval_brush](selections/selection_interval_brush.prism.json) | <img src="selections/selection_interval_brush.svg" width="240"> |
| [selection_cross_chart_overview](selections/selection_cross_chart_overview.prism.json) | <img src="selections/selection_cross_chart_overview.svg" width="240"> |
| [selection_cross_chart_detail](selections/selection_cross_chart_detail.prism.json) | <img src="selections/selection_cross_chart_detail.svg" width="240"> |

## Conditions

Per-channel `condition` clauses switch a channel's value based on a
selection or a structured predicate `test`. See
[Encoding › Conditions](../concepts/encoding.md#conditions).

| Spec | Preview |
|---|---|
| [brush_highlight](conditions/brush_highlight.prism.json) | <img src="conditions/brush_highlight.svg" width="240"> |
| [test_predicate](conditions/test_predicate.prism.json) | <img src="conditions/test_predicate.svg" width="240"> |

## Tree

Rooted hierarchies laid out with tidy-tree, plus the `dendrogram`
variant (step links, hidden nodes). See
[Marks › Tree / dendrogram / network](../concepts/marks.md#tree--dendrogram--network).

| Spec | Preview |
|---|---|
| [org_chart](tree/org_chart.prism.json) | <img src="tree/org_chart.svg" width="240"> |
| [decision_tree](tree/decision_tree.prism.json) | <img src="tree/decision_tree.svg" width="240"> |
| [cluster_dendrogram](tree/cluster_dendrogram.prism.json) | <img src="tree/cluster_dendrogram.svg" width="240"> |

## Network

Force-directed node-link diagrams with deterministic seeded layouts.
See [Marks › Tree / dendrogram / network](../concepts/marks.md#tree--dendrogram--network).

| Spec | Preview |
|---|---|
| [citation_network](network/citation_network.prism.json) | <img src="network/citation_network.svg" width="240"> |
| [dependency_graph](network/dependency_graph.prism.json) | <img src="network/dependency_graph.svg" width="240"> |

## Themes

Themes are applied via the `--theme` CLI flag at plot time. Each spec
below is identical; only the rendering theme differs.

| Spec | Preview |
|---|---|
| [bar_light](themes/bar_light.prism.json) | <img src="themes/bar_light.svg" width="240"> |
| [bar_dark](themes/bar_dark.prism.json) | <img src="themes/bar_dark.svg" width="240"> |
| [bar_print](themes/bar_print.prism.json) | <img src="themes/bar_print.svg" width="240"> |
| [bar_high_contrast](themes/bar_high_contrast.prism.json) | <img src="themes/bar_high_contrast.svg" width="240"> |
| [bar_colorblind](themes/bar_colorblind.prism.json) | <img src="themes/bar_colorblind.svg" width="240"> |

## Animation

Each spec declares an `animation` block plus a `key: true` channel; the
SVG previews show the **initial frame**. The tween fires in the browser
web component / WASM runtime when the spec swaps or a new dataset
arrives — see [Spec › Animation](../concepts/spec.md#animation) and
[Browser › Animation](../concepts/browser.md#animation).

| Spec | Preview |
|---|---|
| [swap_bars](animation/swap_bars.prism.json) | <img src="animation/swap_bars.svg" width="240"> |
| [race_bars](animation/race_bars.prism.json) | <img src="animation/race_bars.svg" width="240"> |
