package rules

import (
	"fmt"
	"strings"

	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// LegendSymbolType implements PRISM_SPEC_052: `legend.symbol_type`
// must name a shape the swatch emitter can draw.
//
// The vocabulary is the point mark's own (scene.PointShapes) because
// a legend swatch and a point mark go through the same emitter —
// render/svg/symbols.go. An unrecognised
// name is silently ignored by the encoder, which leaves the author's
// shaped swatches rendering as the default square with nothing to
// explain why, so it is named here instead.
type LegendSymbolType struct{}

// Code returns PRISM_SPEC_052.
func (LegendSymbolType) Code() string { return "PRISM_SPEC_052" }

// Check walks every legend block in the spec tree (the shared
// walkLegendBlocks, see legend_type_coherent.go) and emits one error
// per unrecognised symbol_type.
func (LegendSymbolType) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	var out []*errors.AppError
	for _, b := range walkLegendBlocks(s) {
		if b.Legend.SymbolType == "" || legendShapeKnown(b.Legend.SymbolType) {
			continue
		}
		out = append(out, errors.New("PRISM_SPEC_052",
			fmt.Sprintf("Legend symbol_type %q on channel %q is not a shape Prism draws.",
				b.Legend.SymbolType, b.Channel),
			map[string]any{
				"Channel":    b.Channel,
				"SymbolType": b.Legend.SymbolType,
				"Allowed":    legendShapeList(),
				"Path":       b.Path,
			},
		))
	}
	return out
}

// legendShapeKnown reports whether name is one of the drawable shapes.
func legendShapeKnown(name string) bool {
	for _, s := range scene.PointShapes {
		if scene.PointShape(name) == s {
			return true
		}
	}
	return false
}

// legendShapeList renders the vocabulary for an error's details.
func legendShapeList() string {
	names := make([]string, 0, len(scene.PointShapes))
	for _, s := range scene.PointShapes {
		names = append(names, string(s))
	}
	return strings.Join(names, ", ")
}
