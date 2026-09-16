package marks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/table"
)

// NullChannel binds an encoding channel name ("x", "y", …) to the
// upstream table field it reads. DropNullRows takes these pairs so
// the warning it emits can name the offending *channels* — what the
// spec author wrote — while the null test itself runs over columns.
type NullChannel struct {
	Channel string
	Field   string
}

// DropNullRows removes every row of tbl carrying a null in one of the
// scale-bound channels' fields, which is the single place Prism
// implements the drop-with-warning null policy documented in
// `docs/src/concepts/multi-source.md`. Without it a mid-series null
// reaches Scale.Apply and hard-errors with PRISM_ENCODE_001.
//
// Filtering happens on the whole table rather than per-encoder, so
// every downstream consumer — the scale domains, the row partitioner
// in group.go, tooltips, datum back-references, category styles and
// conditions — sees one consistent row set and no index can drift.
//
// Returns:
//   - (tbl, nil, nil) when nothing is null. The table is returned
//     untouched, so a spec with no nulls is byte-identical to before.
//   - (filtered, warning, nil) when some rows drop. The warning is
//     PRISM_WARN_NULL_DROPPED carrying the dropped count and the
//     offending channel names.
//   - (nil, nil, error) when every row drops — an all-null bound
//     field is a spec/data mistake, not a chart, so it surfaces
//     PRISM_ENCODE_NULL_ALL_ROWS rather than an empty plot.
//
// layerID is stamped on the warning so a composite caller can say
// which layer shed rows; "" leaves scene.Warning.Layer empty.
func DropNullRows(tbl *table.Table, layerID string, chans ...NullChannel) (*table.Table, *scene.Warning, error) {
	if tbl == nil || len(chans) == 0 {
		return tbl, nil, nil
	}
	byField := map[string]string{}
	fields := make([]string, 0, len(chans))
	for _, ch := range chans {
		if ch.Field == "" {
			continue
		}
		if _, seen := byField[ch.Field]; seen {
			continue
		}
		byField[ch.Field] = ch.Channel
		fields = append(fields, ch.Field)
	}
	if len(fields) == 0 {
		return tbl, nil, nil
	}

	kept, dropped, offending := SkipNullRows(tbl, fields...)
	if dropped == 0 {
		return tbl, nil, nil
	}

	// Report channels, not columns, and in a stable order so the
	// warning text does not depend on map iteration.
	channels := make([]string, 0, len(offending))
	for _, f := range offending {
		if name := byField[f]; name != "" {
			channels = append(channels, name)
		} else {
			channels = append(channels, f)
		}
	}
	sort.Strings(channels)
	sort.Strings(offending)
	channelList := strings.Join(channels, ", ")

	if len(kept) == 0 {
		return nil, nil, prismerrors.New(
			"PRISM_ENCODE_NULL_ALL_ROWS",
			fmt.Sprintf("Every one of the %d upstream rows is null on channel(s) %s; there is nothing left to draw.", dropped, channelList),
			map[string]any{
				"Count":    dropped,
				"Channels": channelList,
				"Fields":   strings.Join(offending, ", "),
			},
		)
	}

	keep := make([]bool, tbl.NumRows())
	for _, i := range kept {
		keep[i] = true
	}
	filtered, err := table.Filter(tbl, keep, "null-drop:"+channelList)
	if err != nil {
		return nil, nil, err
	}
	return filtered, &scene.Warning{
		Code:    scene.WarnNullDropped,
		Layer:   layerID,
		Message: fmt.Sprintf("%d rows skipped: encoding channels %s carried null values.", dropped, channelList),
		Details: map[string]any{
			"count":    dropped,
			"channels": channels,
			"fields":   offending,
		},
	}, nil
}

// SkipNullRows returns the indices of rows in tbl where every named
// field is non-null. Encoders that consume `fields` per-row use this
// to drop rows where any required channel is null so geometries don't
// render at zero positions or default colors.
//
// The skipped count + offending field names are reported back so the
// caller can surface PRISM_WARN_NULL_DROPPED. DropNullRows is the one
// caller in the encoder pipeline; prefer it over re-deriving the
// filter at a mark encoder.
func SkipNullRows(tbl *table.Table, fields ...string) (kept []int, dropped int, offending []string) {
	if tbl == nil {
		return nil, 0, nil
	}
	n := tbl.NumRows()
	kept = make([]int, 0, n)
	offendingSet := map[string]struct{}{}
	for i := 0; i < n; i++ {
		row := true
		for _, f := range fields {
			if f == "" {
				continue
			}
			col, ok := tbl.Column(f)
			if !ok {
				continue
			}
			if col.IsNull(i) {
				row = false
				offendingSet[f] = struct{}{}
			}
		}
		if row {
			kept = append(kept, i)
		} else {
			dropped++
		}
	}
	if dropped == 0 {
		return kept, 0, nil
	}
	for f := range offendingSet {
		offending = append(offending, f)
	}
	return kept, dropped, offending
}
