package marks

import (
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/spec"
)

// encodeDendrogram is a variant of encodeTree where:
//
//   - link_shape defaults to "step" (clade brackets),
//   - node_shape defaults to "none" (labels only, no glyph),
//
// and the layout is otherwise identical to the tidy-tree pass.
// Implementation reuses encodeTree by overriding the defaults; users
// who want explicit visuals on dendrogram marks override link_shape
// / node_shape in the spec.
func encodeDendrogram(in Inputs) ([]scene.Mark, error) {
	def := spec.MarkDef{}
	if in.Mark != nil {
		def = *in.Mark
	}
	if def.LinkShape == "" {
		def.LinkShape = "step"
	}
	if def.NodeShape == "" {
		def.NodeShape = "none"
	}
	in.Mark = &def
	return encodeTree(in)
}
