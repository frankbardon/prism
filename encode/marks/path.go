package marks

import (
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodePath emits exactly one Mark with PathGeom carrying the raw
// SVG path string from mark_def.path. The d-string passes through
// the encoder + renderer untouched; the renderer's attribute
// escaping handles user-supplied special characters. See D068.
//
// Validator PRISM_SPEC_017 catches empty d at validate time; the
// encoder re-checks defensively in case someone bypassed validate.
func encodePath(in Inputs) ([]scene.Mark, error) {
	d := ""
	if in.Mark != nil {
		d = in.Mark.Path
	}
	if d == "" {
		return nil, prismerrors.New(
			"PRISM_ENCODE_001",
			"path mark requires a non-empty d field (mark_def.path).",
			map[string]any{"Field": "<path.d>", "Source": "<mark_def>", "Available": "valid SVG path"},
		)
	}
	mark := scene.Mark{
		Type:  scene.MarkPath,
		ID:    "path-0",
		Style: in.Style,
		Path:  &scene.PathGeom{D: d},
	}
	return []scene.Mark{mark}, nil
}
