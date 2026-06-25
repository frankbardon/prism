package marks

import (
	"github.com/frankbardon/prism/encode/scene"
)

// encodeSparkline emits a line geometry identical to encodeLine.
// The "strip axes/legends/title" behaviour lives at the encode.go
// level (D067) — it suppresses axis builders + legend builders +
// title-block construction when markType == "sparkline".
//
// Plot region uses 4-px padding (ComputeSparkline in encode/layout.go).
// Zero new geometry code; sparkline is a thin wrapper over line.
func encodeSparkline(in Inputs) ([]scene.Mark, error) {
	marks, err := encodeLine(in)
	if err != nil {
		return nil, err
	}
	// Tag the mark IDs so the renderer can still pick them out.
	for i := range marks {
		if marks[i].ID == "" {
			marks[i].ID = "sparkline-0"
		} else {
			marks[i].ID = "sparkline-" + marks[i].ID
		}
	}
	// Opt-in adornments (E4); no-op when no adornment field is set.
	return appendSparkAdornments(in, marks)
}
