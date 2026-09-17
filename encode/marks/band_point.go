package marks

// Positioning a mark that has NO extent along a band axis.
//
// A band scale's Apply returns the LEADING EDGE of a category's slot,
// which is d3's scaleBand() contract and exactly what a rect wants: a
// bar takes that edge as its origin and spans BandWidth() from it, so
// it fills the slot and reads as belonging to the category.
//
// A mark with no width along that axis has no such second coordinate
// to close the gap. A point, a line vertex or a text anchor placed at
// the leading edge sits at the START of its slot rather than on it —
// visibly offset from the tick label naming the category, by half a
// band plus the outer padding. With four quarters across 800px that is
// roughly 83px, which is most of the way to the previous category.
//
// PointPixel is the one place that correction lives. A mark that draws
// no extent along an axis resolves that axis through this function
// rather than calling Scale.Apply directly, and gets the slot MIDPOINT
// on a band scale and an unchanged Apply on every other scale.
//
// BandWidth() is signed — a y band scale built with an inverted range
// reports a negative width, because its slots run bottom-to-top. Half
// a signed width is still the midpoint, so no orientation branch is
// needed here; this is the same property CategorySlots and
// rectAxisExtent rely on, and it is why the correction must be applied
// as an offset from Apply rather than recomputed from a slot index.
//
// Marks that seat themselves in a band by design — bar, rect,
// heatmap, boxplot, violin, tick, winloss, progress and the spark
// adornments — must NOT route through here. They already resolve
// geometry through CategorySlots / CategoryCenters in orient.go, which
// own the band-relative math for a mark that HAS extent.
func PointPixel(ch Channel, v any) (float64, error) {
	p, err := ch.Scale.Apply(v)
	if err != nil {
		return 0, err
	}
	if band, ok := ch.Scale.(BandScaler); ok {
		p += band.BandWidth() / 2
	}
	return p, nil
}
