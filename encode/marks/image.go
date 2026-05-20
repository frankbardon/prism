package marks

import (
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeImage emits exactly one Mark with ImageGeom carrying the
// URL from mark_def.url and a fixed position + size. The URL is
// passed through verbatim; validator PRISM_SPEC_016 ensures it is a
// data: URL or relative path before this encoder runs (D068).
//
// Position: when x + y channels are bound, resolve via the scales.
// Otherwise the image lands at the plot region's top-left quarter
// (sensible default for a single-image decoration).
//
// Size: defaults to 64×64; override via mark_def.size (treated as
// side length — image marks are square by default).
func encodeImage(in Inputs) ([]scene.Mark, error) {
	url := ""
	if in.Mark != nil {
		url = in.Mark.URL
	}
	if url == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"image mark requires a non-empty url field (mark_def.url).",
			map[string]any{"Field": "<image.url>", "Source": "<mark_def>", "Available": "data: URL or relative path"},
		)
	}

	side := 64.0
	if in.Mark != nil && in.Mark.Size != nil && *in.Mark.Size > 0 {
		side = *in.Mark.Size
	}

	// Position: scaled x/y when bound, otherwise top-left quarter of plot.
	x := in.Layout.X + in.Layout.W/4
	y := in.Layout.Y + in.Layout.H/4
	if in.X.Field != "" && in.X.Scale != nil {
		xs, err := readField(in.Table, in.X.Field)
		if err != nil {
			return nil, err
		}
		if len(xs) > 0 {
			xv, err := in.X.Scale.Apply(xs[0])
			if err == nil {
				x = xv
			}
		}
	}
	if in.Y.Field != "" && in.Y.Scale != nil {
		ys, err := readField(in.Table, in.Y.Field)
		if err != nil {
			return nil, err
		}
		if len(ys) > 0 {
			yv, err := in.Y.Scale.Apply(ys[0])
			if err == nil {
				y = yv
			}
		}
	}

	mark := scene.Mark{
		Type:  scene.MarkImage,
		ID:    "image-0",
		Style: in.Style,
		Image: &scene.ImageGeom{
			X:    x,
			Y:    y,
			W:    side,
			H:    side,
			Href: url,
		},
	}
	return []scene.Mark{mark}, nil
}
