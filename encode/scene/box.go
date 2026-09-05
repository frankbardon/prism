package scene

// Box is a plain width/height allotment carrying no position. It is
// the content area the active render backend hands to a `custom` mark
// (E2) — unlike Rect (a positioned, pixel-resolved bounding box used
// by every scale-participating mark), a custom mark is freeform /
// document-flow: it never composes against a shared x/y scale or
// layout algorithm, so it has nothing to be positioned relative to.
// The render backend is free to choose how Box.W/H are derived (e.g.
// the enclosing layer's plot rect, or a caller-configured size); that
// wiring lands with the SVG/HTML consumption stories (E2-S2/E2-S3).
type Box struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}
