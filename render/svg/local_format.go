package svg

import "github.com/frankbardon/prism/render"

// formatF re-exports render.FormatFloat under a short local name so
// other files in this package (notably axes.go) can format floats
// without importing render directly + creating awkward import noise.
func formatF(v float64) string {
	return render.FormatFloat(v)
}
