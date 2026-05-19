package scale

import (
	"fmt"

	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// OrdinalScale maps discrete categories to explicit pixel positions.
// Used when the caller already knows where each category sits
// (typical for fixed-position labels, color-scheme indexing).
type OrdinalScale struct {
	Categories []string
	Positions  []float64 // same length as Categories
}

// Apply implements Scale.
func (s *OrdinalScale) Apply(value any) (float64, error) {
	cat, ok := value.(string)
	if !ok {
		return 0, prismerrors.New(
			"PRISM_ENCODE_001",
			fmt.Sprintf("OrdinalScale.Apply: value %v (type %T) is not a string category.", value, value),
			map[string]any{"Field": "<ordinal>", "Source": "<scale>", "Available": "string"},
		)
	}
	for i, c := range s.Categories {
		if c == cat {
			return s.Positions[i], nil
		}
	}
	return 0, prismerrors.New(
		"PRISM_ENCODE_001",
		fmt.Sprintf("OrdinalScale.Apply: category %q not in domain.", cat),
		map[string]any{"Field": "<ordinal>", "Source": "<scale>", "Available": joinCats(s.Categories)},
	)
}

// ApplyColor returns the palette entry indexed by the category's
// position in Categories (mod len(palette)). Returns nil when either
// category or palette is empty.
func (s *OrdinalScale) ApplyColor(category string, palette []*scene.Color) *scene.Color {
	if len(palette) == 0 {
		return nil
	}
	for i, c := range s.Categories {
		if c == category {
			return palette[i%len(palette)]
		}
	}
	return palette[0]
}

// Domain implements Scale.
func (s *OrdinalScale) Domain() []any {
	out := make([]any, len(s.Categories))
	for i, c := range s.Categories {
		out[i] = c
	}
	return out
}

// Range implements Scale.
func (s *OrdinalScale) Range() [2]float64 {
	if len(s.Positions) == 0 {
		return [2]float64{0, 0}
	}
	mn, mx := s.Positions[0], s.Positions[0]
	for _, p := range s.Positions {
		if p < mn {
			mn = p
		}
		if p > mx {
			mx = p
		}
	}
	return [2]float64{mn, mx}
}

// Type implements Scale.
func (s *OrdinalScale) Type() scene.ScaleType { return scene.ScaleOrdinal }
