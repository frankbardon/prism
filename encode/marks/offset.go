package marks

// Offset position channels (E1-S4).
//
// `x_offset` / `y_offset` subdivide the band slot a category owns, so
// rows sharing one category value render side by side instead of on
// top of one another — the grouped (dodged) bar primitive Vega-Lite
// spells xOffset / yOffset.
//
// The binding a mark encoder sees is OffsetBinding: the column read
// per row, plus a nested band scale whose range is the PARENT band's
// extent. Applying that scale to a row's offset value yields a signed
// displacement from the slot's leading edge, and the scale's own
// BandWidth() is the sub-band's signed width. Both are folded into
// the one sign correction rectAxisExtent (span.go) already performs on
// a full slot, so a y band running bottom-to-top dodges correctly
// without a second normaliser.
//
// The zero value means "no offset bound" — the X2 / Y2 precedent —
// and every reader goes through On, so a spec that binds no offset
// takes exactly the geometry it took before this channel existed.

// OffsetBinding is the resolved x_offset / y_offset binding handed to
// the mark encoders. Built by encode.resolveOffsetBinding; never
// constructed inside this package.
type OffsetBinding struct {
	// Axis is "x" or "y" — the position channel whose band slot this
	// offset subdivides. Empty means unbound, and is the whole of the
	// zero value's meaning.
	Axis string
	// Field is the table column whose distinct values name the
	// sub-bands.
	Field string
	// Scale is the nested band scale. Its range is (0, parent
	// BandWidth()) and is therefore SIGNED: on a y band scale, which
	// runs bottom-to-top, Apply returns a negative displacement and
	// BandWidth() a negative sub-band width, exactly as the parent
	// scale reports them. Callers must not re-derive the sign; they
	// hand both to rectAxisExtent, which owns the correction.
	Scale Scale
}

// On reports whether this binding subdivides the named axis ("x" /
// "y"). It is the single guard every consumer uses: an unbound
// binding, a binding on the other axis, or one that never resolved a
// scale all answer false, which is what keeps an offset-free spec on
// the historic arithmetic exactly.
func (o OffsetBinding) On(axis string) bool {
	return o.Axis != "" && o.Axis == axis && o.Field != "" && o.Scale != nil
}

// band exposes the nested scale's signed sub-band width. A scale that
// cannot report one is treated as unbound rather than guessed at.
func (o OffsetBinding) band() (BandScaler, bool) {
	if o.Scale == nil {
		return nil, false
	}
	b, ok := o.Scale.(BandScaler)
	return b, ok
}
