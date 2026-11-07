package encode

import (
	"github.com/frankbardon/prism/encode/marks"
	"github.com/frankbardon/prism/theme"
)

// autoDark is one document's E4-S3 auto light/dark state: the
// registry every paired mark and legend colour is filed in, and the
// active theme's DarkVariant counterpart the dark halves come from.
//
// A document has exactly one. The top-of-tree encoder call creates it
// (autoDarkFor reports owner) and hands it to every composition cell
// and layer through EncodeOpts.autoDark, so a layered, concatenated,
// faceted or repeated chart pairs its marks and legend swatches like
// a single view does; only the owner folds the pairs into the theme
// CSS (finalizeAutoDarkCSS), once, after every cell has registered.
type autoDark struct {
	reg  *marks.ColorVarRegistry
	dark *theme.Theme
}

// autoDarkFor returns the auto-dark state one encoder call works with
// and whether that call owns it. A cell handed its parent's state
// reuses it. Otherwise only a call that resolves its own theme
// (opts.Theme == nil) whose theme names a registered DarkVariant
// creates one; with no DarkVariant the state is nil and every colour
// is baked as before E4-S3.
func autoDarkFor(opts EncodeOpts, full *theme.Theme) (ad *autoDark, owner bool) {
	if opts.autoDark != nil {
		return opts.autoDark, false
	}
	if opts.Theme != nil || full == nil || full.DarkVariant == "" {
		return nil, false
	}
	dv, ok := theme.Get(full.DarkVariant)
	if !ok {
		return nil, false
	}
	return &autoDark{reg: marks.NewColorVarRegistry(), dark: dv}, true
}

// registry is the document's registry; nil when auto-dark is off.
func (a *autoDark) registry() *marks.ColorVarRegistry {
	if a == nil {
		return nil
	}
	return a.reg
}

// darkTheme is the DarkVariant counterpart; nil when auto-dark is off.
func (a *autoDark) darkTheme() *theme.Theme {
	if a == nil {
		return nil
	}
	return a.dark
}

// cellOpts hands the document's auto-dark state to a composition
// cell's (or layer's) encoder options.
func (a *autoDark) cellOpts(opts EncodeOpts) EncodeOpts {
	opts.autoDark = a
	return opts
}
