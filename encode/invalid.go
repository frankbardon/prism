package encode

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/encode/marks"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/table"
)

// Null handling: mark.invalid (v0.16).
//
// A row carrying a null in a scale-bound channel cannot be positioned,
// so something has to give. Prism has always dropped it — and dropped
// it BEFORE scale resolution, which means the row's category left the
// domain too. On a discrete axis that is a stronger edit than it
// sounds: four quarters with one unmeasured render three EVENLY SPACED
// points, the missing quarter absent from the axis, and nothing in the
// drawing from which a reader could recover that a period is missing.
// For a line it is stronger still, because a path closed over the hole
// asserts the series ran uninterrupted.
//
// "break" keeps the row instead. The category stays in the domain and
// holds its slot, no mark is drawn at it, and path marks split either
// side. "filter" is the default and is untouched, which is what keeps
// every pre-v0.16 chart byte-identical.
//
// The two modes are implemented differently ON PURPOSE. "filter"
// removes the rows from the table (marks.DropNullRows), so every
// downstream consumer sees one shorter row set. "break" leaves the
// table alone and carries a per-row mask, so the table the scales
// resolve from, the table the encoders draw from, and the indices
// tooltips / datum back-references / category styles stamp are all
// still the same table. Filtering for one mode and masking for the
// other is what avoids ever having two row sets in flight at once.

// invalidMode resolves mark.invalid to a mode constant, defaulting to
// spec.MarkInvalidFilter. Validation rejects anything else, so an
// unrecognised value here is treated as the default rather than
// failing the render.
func invalidMode(s *spec.Spec) string {
	if s == nil || s.Mark == nil || s.Mark.Def == nil {
		return spec.MarkInvalidFilter
	}
	if s.Mark.Def.Invalid == spec.MarkInvalidBreak {
		return spec.MarkInvalidBreak
	}
	return spec.MarkInvalidFilter
}

// breakNullRows computes the per-row skip mask for "break" mode and
// the warning that reports it.
//
// The table is returned unchanged: the whole point of the mode is that
// the null rows stay, so their categories reach the scale domain. The
// mask says which rows must not be DRAWN.
//
// Returns (nil, nil, nil) when no row is null, which keeps a chart
// with clean data byte-identical whichever mode it asks for. An
// all-null bound field still fails, exactly as it does under "filter":
// a chart with no drawable row is a data mistake, not a gap.
func breakNullRows(tbl *table.Table, layerID string, chans ...marks.NullChannel) ([]bool, *scene.Warning, error) {
	if tbl == nil || len(chans) == 0 {
		return nil, nil, nil
	}
	fields := make([]string, 0, len(chans))
	seen := map[string]bool{}
	for _, ch := range chans {
		if ch.Field == "" || seen[ch.Field] {
			continue
		}
		seen[ch.Field] = true
		fields = append(fields, ch.Field)
	}
	if len(fields) == 0 {
		return nil, nil, nil
	}
	// Reuse the one null test rather than re-deriving it; SkipNullRows
	// reports the kept indices, the dropped count and which fields
	// offended, which is everything both modes need.
	kept, dropped, offending := marks.SkipNullRows(tbl, fields...)
	if dropped == 0 {
		return nil, nil, nil
	}
	if len(kept) == 0 {
		// Delegate the all-null case so its code and message stay in
		// one place. DropNullRows returns exactly that error.
		_, _, err := marks.DropNullRows(tbl, layerID, chans...)
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	}
	skip := make([]bool, tbl.NumRows())
	for i := range skip {
		skip[i] = true
	}
	for _, i := range kept {
		skip[i] = false
	}

	// Report channels rather than columns, in a stable order, the same
	// way DropNullRows does — an author reads the warning against what
	// they wrote, and map iteration order must not reach the text.
	byField := map[string]string{}
	for _, ch := range chans {
		if _, dup := byField[ch.Field]; ch.Field != "" && !dup {
			byField[ch.Field] = ch.Channel
		}
	}
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

	return skip, &scene.Warning{
		Code:  scene.WarnNullDropped,
		Layer: layerID,
		Message: fmt.Sprintf(
			"%d rows carried null values on encoding channels %s; they keep their place on the axis and paths break at the gap.",
			dropped, strings.Join(channels, ", ")),
		Details: map[string]any{
			"count":    dropped,
			"channels": channels,
			"fields":   offending,
			"invalid":  spec.MarkInvalidBreak,
		},
	}, nil
}
