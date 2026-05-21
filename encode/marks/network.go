package marks

import (
	"github.com/frankbardon/prism/encode/scene"
	prismerrors "github.com/frankbardon/prism/errors"
)

// encodeNetwork is the network mark scaffold. The full
// Fruchterman-Reingold layout lands in tier1-04 PR4; this PR2 stub
// exists so the marks dispatch compiles and a `tree` / `network`
// spec can validate even if the network case isn't wired yet.
//
// Today the stub returns PRISM_ENCODE_NETWORK_NONFINITE so any spec
// that picks the network mark sees a clear "not yet implemented"
// signal pointing at the upcoming PR.
func encodeNetwork(in Inputs) ([]scene.Mark, error) {
	_ = in
	return nil, prismerrors.New(
		"PRISM_ENCODE_NETWORK_NONFINITE",
		"network mark layout not yet implemented — pending tier1-04 PR4 (force-directed layout).",
		map[string]any{"Mark": "network"},
	)
}
