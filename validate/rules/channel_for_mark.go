package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/frankbardon/prism/errors"
	"github.com/frankbardon/prism/spec"
	"github.com/frankbardon/prism/validate"
)

// ChannelForMark implements PRISM_SPEC_003: each encoding channel must be
// supported by the spec's mark type. The supported set is per-mark; for
// example, theta/radius are supported by arc/pie/donut but not by bar.
type ChannelForMark struct{}

// Code returns PRISM_SPEC_003.
func (ChannelForMark) Code() string { return "PRISM_SPEC_003" }

// Check inspects every channel set on s.Encoding against
// allowedChannelsForMark(mark).
func (ChannelForMark) Check(s *spec.Spec, _ validate.SchemaLookup) []*errors.AppError {
	if s == nil || s.Mark == nil || s.Encoding == nil {
		return nil
	}
	mark := s.Mark.TypeName()
	if mark == "" {
		return nil
	}
	allowed := allowedChannelsForMark(mark)
	allowedSet := make(map[string]bool, len(allowed))
	for _, c := range allowed {
		allowedSet[c] = true
	}

	var out []*errors.AppError
	for _, ch := range presentChannels(s.Encoding) {
		if allowedSet[ch] {
			continue
		}
		out = append(out, errors.New("PRISM_SPEC_003",
			fmt.Sprintf("Encoding channel %q is not valid for mark type %q.", ch, mark),
			map[string]any{
				"Channel": ch,
				"Mark":    mark,
				"Allowed": strings.Join(allowed, ", "),
			},
		))
	}
	return out
}

// presentChannels returns the sorted list of channel keys that are
// non-nil on enc.
func presentChannels(enc *spec.Encoding) []string {
	out := []string{}
	if enc == nil {
		return out
	}
	type entry struct {
		name    string
		present bool
	}
	entries := []entry{
		{"x", enc.X != nil}, {"y", enc.Y != nil}, {"x2", enc.X2 != nil}, {"y2", enc.Y2 != nil},
		{"theta", enc.Theta != nil}, {"radius", enc.Radius != nil},
		{"color", enc.Color != nil}, {"fill", enc.Fill != nil}, {"stroke", enc.Stroke != nil},
		{"opacity", enc.Opacity != nil}, {"size", enc.Size != nil}, {"shape", enc.Shape != nil},
		{"text", enc.Text != nil}, {"tooltip", enc.Tooltip != nil},
		{"order", enc.Order != nil}, {"detail", enc.Detail != nil},
		{"row", enc.Row != nil}, {"column", enc.Column != nil},
		{"source", enc.Source != nil}, {"target", enc.Target != nil}, {"value", enc.Value != nil},
		{"longitude", enc.Longitude != nil}, {"latitude", enc.Latitude != nil}, {"feature", enc.Feature != nil},
	}
	for _, e := range entries {
		if e.present {
			out = append(out, e.name)
		}
	}
	sort.Strings(out)
	return out
}

// allowedChannelsForMark returns the channels supported by mark.
//
// Tooltip, order, detail, row, and column are universal across every
// mark. The per-mark differences are mostly positional vs angular vs
// text-only. Sankey lives in its own tier (source/target/value per
// D064); funnel lives in its own tier (x category + y quantity per
// D066); sparkline mirrors line (D067).
func allowedChannelsForMark(mark string) []string {
	common := []string{"tooltip", "order", "detail", "row", "column"}
	cartesianMark := []string{"x", "y", "x2", "y2", "color", "fill", "stroke", "opacity", "size", "shape"}
	polarMark := []string{"theta", "radius", "color", "fill", "stroke", "opacity"}
	textMark := []string{"x", "y", "text", "color", "fill", "stroke", "opacity", "size"}
	sankeyMark := []string{"source", "target", "value", "color", "fill", "stroke", "opacity"}
	funnelMark := []string{"x", "y", "color", "fill", "stroke", "opacity", "text"}
	sparklineMark := []string{"x", "y", "color", "fill", "stroke", "opacity"}
	sparkbarMark := []string{"x", "y", "color", "fill", "stroke", "opacity"}
	winlossMark := []string{"x", "y", "color", "fill", "stroke", "opacity"}
	sparkareaMark := []string{"x", "y", "color", "fill", "stroke", "opacity"}
	geoshapeMark := []string{"feature", "color", "fill", "stroke", "opacity"}
	geopointMark := []string{"longitude", "latitude", "color", "fill", "stroke", "opacity", "size", "shape"}
	treeMark := []string{"source", "target", "value", "text", "color", "fill", "stroke", "opacity", "size"}

	var set []string
	switch mark {
	case "bar", "line", "area", "point", "circle", "square", "tick", "rect", "rule",
		"histogram", "heatmap", "boxplot", "violin":
		set = cartesianMark
	case "arc", "pie", "donut":
		set = polarMark
	case "sankey":
		set = sankeyMark
	case "funnel":
		set = funnelMark
	case "sparkline":
		set = sparklineMark
	case "sparkbar":
		set = sparkbarMark
	case "winloss":
		set = winlossMark
	case "sparkarea":
		set = sparkareaMark
	case "text":
		set = textMark
	case "image":
		set = []string{"x", "y", "x2", "y2", "opacity"}
	case "path":
		set = []string{"x", "y", "color", "fill", "stroke", "opacity"}
	case "geoshape":
		set = geoshapeMark
	case "geopoint":
		set = geopointMark
	case "tree", "dendrogram", "network":
		set = treeMark
	default:
		set = append(cartesianMark, polarMark...)
	}
	out := append(append([]string{}, set...), common...)
	sort.Strings(out)
	return uniqueStrings(out)
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
