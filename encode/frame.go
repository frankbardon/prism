package encode

import "github.com/frankbardon/prism/spec"

// specDimension is a spec `width` / `height` as pixels: the number
// when it is a positive one, else 0 (unset, or a token such as
// "container", which leaves the size to the caller).
func specDimension(d *spec.Dimension) float64 {
	if d == nil || d.Number == nil || *d.Number <= 0 {
		return 0
	}
	return *d.Number
}

// withSpecFrame fills an unset EncodeOpts size from the spec's own
// top-level `width` / `height`. It runs once, at the top of the tree
// (Encode / EncodeComposite): a composition cell is always handed a
// size by its parent, which reads the children's declared sizes
// itself (concatGrid). Precedence is therefore an explicit EncodeOpts
// size (the CLI's --width, a caller's frame), then the spec's, then
// the encoder's own default (800 × 600, or a concat's derived size).
func withSpecFrame(opts EncodeOpts, s *spec.Spec) EncodeOpts {
	if s == nil {
		return opts
	}
	if opts.Width == 0 {
		opts.Width = specDimension(s.Width)
	}
	if opts.Height == 0 {
		opts.Height = specDimension(s.Height)
	}
	return opts
}
