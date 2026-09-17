package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// OffsetAxisCoherent implements PRISM_SPEC_064: an offset needs a band
// to subdivide, and exactly one axis to subdivide it on.
//
// Two incoherences are statically decidable from the spec alone:
//
//   - the offset's own position channel resolves to no band scale, so
//     there is no slot to cut into sub-bands;
//   - BOTH x_offset and y_offset are bound, which describes no
//     geometry at all — a mark dodges along one axis while the measure
//     runs along the other.
//
// The second half is why this rule reads enc.XOffset / enc.YOffset
// directly instead of asking spec.ResolveOffset: that resolver returns
// nil in exactly the both-bound case, refusing the ambiguity rather
// than picking a winner, so it cannot be the thing that reports it.
//
// Only statically decidable cases are reported here. A channel with no
// declared `type` resolves its scale family from the executed column's
// kind, which validate never sees; that case reaches the encoder and
// is reported there (PRISM_ENCODE_001). Duplicating it would give one
// spec two errors from two stages.
type OffsetAxisCoherent struct{}

// Code returns PRISM_SPEC_064.
func (OffsetAxisCoherent) Code() string { return "PRISM_SPEC_064" }

// Check walks every leaf that binds an offset channel.
func (OffsetAxisCoherent) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, leaf := range walkOffsetLeaves(s) {
		bound := leaf.Bound()
		if len(bound) == 2 {
			// Both axes. The per-axis band question is moot — neither
			// offset is applied — so this is the one error to report.
			out = append(out, offsetIncoherent(leaf.Path,
				`both "x_offset" and "y_offset" are bound, and a mark dodges along one axis only`))
			continue
		}
		for _, off := range bound {
			base := leaf.Position(off.Axis)
			if reason, ok := offsetAxisNotBanded(base); ok {
				out = append(out, offsetIncoherent(leaf.Path,
					fmt.Sprintf("%q subdivides the band slot of channel %q, which %s",
						off.Name, off.Axis, reason)))
			}
		}
	}
	return out
}

// offsetIncoherent builds the PRISM_SPEC_064 envelope. Detail is the
// one string the code's message template interpolates, so both halves
// of the rule read identically to an author.
func offsetIncoherent(path, detail string) *errors.AppError {
	return errors.New("PRISM_SPEC_064",
		fmt.Sprintf("Offset binding is incoherent: %s.", detail),
		map[string]any{"Path": path, "Detail": detail},
	)
}

// offsetAxisNotBanded reports whether the position channel an offset
// subdivides is KNOWN, from the spec alone, to resolve to something
// other than a band scale — and why.
//
// Returns false whenever the answer depends on the executed data: a
// bound channel with no declared type and no explicit scale.type takes
// its family from the column's kind at execute time, so validate must
// stay quiet and let the encoder answer.
//
// Only `band` subdivides. A `point` or `ordinal` scale places
// categories without giving any of them width (neither implements
// BandWidth), so neither has a slot to cut.
func offsetAxisNotBanded(ch *spec.PositionChannel) (string, bool) {
	if !channelBound(ch) {
		return "binds no field", true
	}
	if ch.Scale != nil && ch.Scale.Type != "" {
		if ch.Scale.Type == "band" {
			return "", false
		}
		return fmt.Sprintf("declares scale.type %q and so resolves to no band", ch.Scale.Type), true
	}
	switch ch.Type {
	case "quantitative", "temporal":
		return fmt.Sprintf("is %q and so resolves to a continuous scale with no slot to subdivide", ch.Type), true
	}
	return "", false
}
