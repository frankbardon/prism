package scene

// Warning is the structured-warning shape attached to SceneDoc and
// surfaced by the CLI / browser. Codes use the PRISM_WARN_* form.
type Warning struct {
	Code    string         `json:"code"`
	Layer   string         `json:"layer,omitempty"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Known warning codes emitted by the encoder / renderer in P05+.
const (
	WarnTimeScaleStubbed    = "PRISM_WARN_TIME_SCALE_STUBBED"
	WarnMarkNotImplemented  = "PRISM_WARN_MARK_NOT_IMPLEMENTED"
	WarnNoDataForLayer      = "PRISM_WARN_NO_DATA_FOR_LAYER"
	WarnPrecisionTruncation = "PRISM_WARN_PRECISION_TRUNCATION"
	// WarnLayerSkipped fires when a composite layer is dropped because
	// its upstream Source / sub-DAG produced no table (typically a
	// partial-failure cascade per D006). The other layers still render.
	WarnLayerSkipped = "PRISM_WARN_LAYER_SKIPPED"
	// WarnAxisConfigConflict fires when two composition children
	// specify different values for the same property of a shared
	// position axis. The first child to specify the property wins; the
	// later value is ignored and reported rather than silently dropped.
	WarnAxisConfigConflict = "PRISM_WARN_AXIS_CONFIG_CONFLICT"
	// WarnAxisValuesDropped fires when `axis.values` pins tick values
	// the axis cannot place: an entry outside the resolved scale
	// domain, an entry the scale family cannot read (a string on a
	// quantitative axis, an unparseable date on a temporal one), or a
	// category the discrete domain does not contain. The surviving
	// entries still become ticks — and, since grid lines follow the
	// tick set, grid lines. Details carry the dropped entries plus the
	// domain they were measured against.
	WarnAxisValuesDropped = "PRISM_WARN_AXIS_VALUES_DROPPED"
	// WarnTableCellUnparseable (E1) fires when a table column carries
	// a sub-mark (e.g. "sparkline") but a given row's raw field value
	// could not be parsed as a numeric series — the cell renders with
	// no nested TableCell rather than failing the whole encode.
	WarnTableCellUnparseable = "PRISM_WARN_TABLE_CELL_UNPARSEABLE"
	// WarnNullDropped fires when the encoder drops upstream rows that
	// carried a null in a scale-bound channel. Details hold the
	// dropped-row count plus the offending channel and field names.
	// The surviving rows still render; an all-null bound field is an
	// error (PRISM_ENCODE_NULL_ALL_ROWS), not a warning.
	WarnNullDropped = "PRISM_WARN_NULL_DROPPED"

	// WarnOffsetCollision (E2-S3) fires when two or more rows share
	// BOTH the category value and the offset value an x_offset /
	// y_offset binding dodges by. A sub-band is identified by that
	// pair, so the repeats land on exactly the same rect and only the
	// last one drawn stays visible. Both rects are still emitted and
	// the geometry is untouched — this names the overlap rather than
	// hiding it, because a chart whose whole point is that bars stop
	// overlapping must not go on overlapping in silence.
	//
	// It cannot fire when the measure channel aggregates: the
	// synthetic group-by keeps the category field and the offset
	// field, so the pair is unique by construction. Raw,
	// un-aggregated tables are the only place it is reachable.
	// Details carry the repeat count, an example key, and the
	// channel / field names it was read from.
	WarnOffsetCollision = "PRISM_WARN_OFFSET_COLLISION"

	// The E7-S1 inert-field family. Each fires when a spec key
	// decodes cleanly, passes validation, and then reaches no
	// consumer — the silent no-op this effort exists to eliminate.
	// They are reported once per spec by encode.InertFieldWarnings,
	// which runs at the top of the tree so a composition child is
	// never reported twice. None of them stops the chart rendering.

	// WarnMarkDefInert fires when a mark_def property is set on a
	// mark whose encoder never reads it (e.g. "pad_angle" on a bar),
	// or when no encoder reads the property at all. Details carry the
	// property, the mark type and the marks that DO read it.
	WarnMarkDefInert = "PRISM_WARN_MARK_DEF_INERT"
	// WarnChannelInert fires when an encoding channel (or a
	// channel-level key such as `title` / `format`) is bound but no
	// encoder consumes it for the spec's mark type.
	WarnChannelInert = "PRISM_WARN_CHANNEL_INERT"
	// WarnScaleFieldInert fires when a `scale` block property does
	// not apply to the scale family the channel resolves to — a
	// `padding_inner` on a linear scale, a `base` on anything but
	// log. Details carry the resolved family.
	WarnScaleFieldInert = "PRISM_WARN_SCALE_FIELD_INERT"
	// PRISM_WARN_LEGEND_FIELD_INERT was retired in E7-S3: E3-S4
	// landed the consumers for all five legend presentation keys in
	// the same wave E7-S1 declared them dead, so the warning had
	// become a false positive. See errors/codes.go.
	//
	// WarnLegendNotBuilt fires when a continuous (quantitative /
	// temporal) color channel binds and no legend is produced for it,
	// so the chart renders with no color key.
	WarnLegendNotBuilt = "PRISM_WARN_LEGEND_NOT_BUILT"
	// WarnFacetChildSkipped fires when a facet child's encoding asks
	// for a channel-level aggregate, a stack or an `order` sort:
	// plan/build strips the child encoding before Build, so none of
	// the three synthetic nodes is injected and the request is
	// silently dropped.
	WarnFacetChildSkipped = "PRISM_WARN_FACET_CHILD_SKIPPED"
)
