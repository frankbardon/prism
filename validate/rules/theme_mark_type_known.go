package rules

import (
	"fmt"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ThemeMarkTypeKnown implements PRISM_SPEC_031: every key under
// theme.marks must name a registered mark type. Typos like "bars"
// or capitalised "Bar" produce defaults that silently never apply;
// the rule surfaces them at validate time.
type ThemeMarkTypeKnown struct{}

// Code returns PRISM_SPEC_031.
func (ThemeMarkTypeKnown) Code() string { return "PRISM_SPEC_031" }

// validMarkTypes is the canonical set keyed by theme.marks. Kept
// in sync with the encoder dispatch in encode.specMarkToScene + the
// composite-mark allowlist.
var validMarkTypes = map[string]bool{
	"bar":        true,
	"line":       true,
	"area":       true,
	"point":      true,
	"rule":       true,
	"text":       true,
	"tick":       true,
	"rect":       true,
	"arc":        true,
	"pie":        true,
	"donut":      true,
	"histogram":  true,
	"heatmap":    true,
	"boxplot":    true,
	"violin":     true,
	"sankey":     true,
	"funnel":     true,
	"sparkline":  true,
	"sparkbar":   true,
	"winloss":    true,
	"sparkarea":  true,
	"bullet":     true,
	"image":      true,
	"path":       true,
	"geoshape":   true,
	"geopoint":   true,
	"tree":       true,
	"dendrogram": true,
	"network":    true,
}

// Check walks every spec node and reports unknown mark-type keys
// under theme.marks.
func (ThemeMarkTypeKnown) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil {
		return nil
	}
	var out []*errors.AppError
	walkSpecForThemeMarks(s, "", &out)
	return out
}

func walkSpecForThemeMarks(s *spec.Spec, prefix string, out *[]*errors.AppError) {
	if s == nil {
		return
	}
	if s.Theme != nil && s.Theme.Marks != nil {
		for k := range s.Theme.Marks {
			if validMarkTypes[k] {
				continue
			}
			*out = append(*out, errors.New(
				"PRISM_SPEC_031",
				fmt.Sprintf("Theme defines defaults for unknown mark type %q (at %stheme.marks.%s).", k, prefix, k),
				map[string]any{"Mark": k, "Path": prefix + "theme.marks." + k},
			))
		}
	}
	for i, layer := range s.Layer {
		walkSpecForThemeMarks(layer, fmt.Sprintf("%slayer[%d].", prefix, i), out)
	}
	for i, child := range s.Concat {
		walkSpecForThemeMarks(child, fmt.Sprintf("%sconcat[%d].", prefix, i), out)
	}
	for i, child := range s.HConcat {
		walkSpecForThemeMarks(child, fmt.Sprintf("%shconcat[%d].", prefix, i), out)
	}
	for i, child := range s.VConcat {
		walkSpecForThemeMarks(child, fmt.Sprintf("%svconcat[%d].", prefix, i), out)
	}
	if s.ChildSpec != nil {
		walkSpecForThemeMarks(s.ChildSpec, prefix+"spec.", out)
	}
}
