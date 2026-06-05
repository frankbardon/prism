package theme

import "sort"

// Scheme kinds. Used by validate/rules + range-slot defaulting.
const (
	SchemeKindCategorical = "categorical"
	SchemeKindSequential  = "sequential"
	SchemeKindDiverging   = "diverging"
	SchemeKindCyclic      = "cyclic"
)

// SchemeInfo records a named color scheme + its kind. Names are
// the d3-scale-chromatic canonical form (lowercase verbatim:
// viridis, magma, tableau10, category10, rdbu, brbg, spectral).
// Snake-case extensions (okabe_ito, tol_bright, tol_vibrant,
// tol_muted) ship Prism's accessibility palettes.
type SchemeInfo struct {
	Name   string
	Kind   string
	Colors []string
}

// builtinSchemes is the registry. SchemeByName + SchemeKind read
// from it; SchemeNames lists registered keys in sorted order. The
// data is canonical:
//   - viridis / magma / plasma / inferno / cividis: BIDS colormaps
//     resampled to 9 stops (source: github.com/BIDS/colormap).
//   - Brewer schemes: ColorBrewer 2.0 9-class entries.
//   - Categorical sets: d3-scale-chromatic verbatim.
//   - Okabe-Ito: Wong, Nature Methods 8:441 (2011).
//   - Tol Bright/Vibrant/Muted: Paul Tol, personal.sron.nl/~pault.
var builtinSchemes = map[string]SchemeInfo{
	// Categorical — d3 / tableau / observable.
	"category10": {Name: "category10", Kind: SchemeKindCategorical, Colors: []string{
		"#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd",
		"#8c564b", "#e377c2", "#7f7f7f", "#bcbd22", "#17becf",
	}},
	"tableau10": {Name: "tableau10", Kind: SchemeKindCategorical, Colors: []string{
		"#4c78a8", "#f58518", "#e45756", "#72b7b2", "#54a24b",
		"#eeca3b", "#b279a2", "#ff9da6", "#9d755d", "#bab0ac",
	}},
	"observable10": {Name: "observable10", Kind: SchemeKindCategorical, Colors: []string{
		"#4269d0", "#efb118", "#ff725c", "#6cc5b0", "#3ca951",
		"#ff8ab7", "#a463f2", "#97bbf5", "#9c6b4e", "#9498a0",
	}},
	"accent": {Name: "accent", Kind: SchemeKindCategorical, Colors: []string{
		"#7fc97f", "#beaed4", "#fdc086", "#ffff99",
		"#386cb0", "#f0027f", "#bf5b17", "#666666",
	}},
	"dark2": {Name: "dark2", Kind: SchemeKindCategorical, Colors: []string{
		"#1b9e77", "#d95f02", "#7570b3", "#e7298a",
		"#66a61e", "#e6ab02", "#a6761d", "#666666",
	}},
	"paired": {Name: "paired", Kind: SchemeKindCategorical, Colors: []string{
		"#a6cee3", "#1f78b4", "#b2df8a", "#33a02c", "#fb9a99", "#e31a1c",
		"#fdbf6f", "#ff7f00", "#cab2d6", "#6a3d9a", "#ffff99", "#b15928",
	}},
	"pastel1": {Name: "pastel1", Kind: SchemeKindCategorical, Colors: []string{
		"#fbb4ae", "#b3cde3", "#ccebc5", "#decbe4", "#fed9a6",
		"#ffffcc", "#e5d8bd", "#fddaec", "#f2f2f2",
	}},
	"pastel2": {Name: "pastel2", Kind: SchemeKindCategorical, Colors: []string{
		"#b3e2cd", "#fdcdac", "#cbd5e8", "#f4cae4",
		"#e6f5c9", "#fff2ae", "#f1e2cc", "#cccccc",
	}},
	"set1": {Name: "set1", Kind: SchemeKindCategorical, Colors: []string{
		"#e41a1c", "#377eb8", "#4daf4a", "#984ea3", "#ff7f00",
		"#ffff33", "#a65628", "#f781bf", "#999999",
	}},
	"set2": {Name: "set2", Kind: SchemeKindCategorical, Colors: []string{
		"#66c2a5", "#fc8d62", "#8da0cb", "#e78ac3",
		"#a6d854", "#ffd92f", "#e5c494", "#b3b3b3",
	}},
	"set3": {Name: "set3", Kind: SchemeKindCategorical, Colors: []string{
		"#8dd3c7", "#ffffb3", "#bebada", "#fb8072", "#80b1d3", "#fdb462",
		"#b3de69", "#fccde5", "#d9d9d9", "#bc80bd", "#ccebc5", "#ffed6f",
	}},

	// Accessibility — colorblind-safe extensions.
	"okabe_ito": {Name: "okabe_ito", Kind: SchemeKindCategorical, Colors: []string{
		"#000000", "#e69f00", "#56b4e9", "#009e73",
		"#f0e442", "#0072b2", "#d55e00", "#cc79a7",
	}},
	"tol_bright": {Name: "tol_bright", Kind: SchemeKindCategorical, Colors: []string{
		"#4477aa", "#ee6677", "#228833", "#ccbb44",
		"#66ccee", "#aa3377", "#bbbbbb",
	}},
	"tol_vibrant": {Name: "tol_vibrant", Kind: SchemeKindCategorical, Colors: []string{
		"#0077bb", "#33bbee", "#009988", "#ee7733",
		"#cc3311", "#ee3377", "#bbbbbb",
	}},
	"tol_muted": {Name: "tol_muted", Kind: SchemeKindCategorical, Colors: []string{
		"#332288", "#88ccee", "#44aa99", "#117733", "#999933",
		"#ddcc77", "#cc6677", "#882255", "#aa4499",
	}},

	// Sequential single-hue — Brewer.
	"blues": {Name: "blues", Kind: SchemeKindSequential, Colors: []string{
		"#f7fbff", "#deebf7", "#c6dbef", "#9ecae1", "#6baed6",
		"#4292c6", "#2171b5", "#08519c", "#08306b",
	}},
	"greens": {Name: "greens", Kind: SchemeKindSequential, Colors: []string{
		"#f7fcf5", "#e5f5e0", "#c7e9c0", "#a1d99b", "#74c476",
		"#41ab5d", "#238b45", "#006d2c", "#00441b",
	}},
	"greys": {Name: "greys", Kind: SchemeKindSequential, Colors: []string{
		"#ffffff", "#f0f0f0", "#d9d9d9", "#bdbdbd", "#969696",
		"#737373", "#525252", "#252525", "#000000",
	}},
	"oranges": {Name: "oranges", Kind: SchemeKindSequential, Colors: []string{
		"#fff5eb", "#fee6ce", "#fdd0a2", "#fdae6b", "#fd8d3c",
		"#f16913", "#d94801", "#a63603", "#7f2704",
	}},
	"purples": {Name: "purples", Kind: SchemeKindSequential, Colors: []string{
		"#fcfbfd", "#efedf5", "#dadaeb", "#bcbddc", "#9e9ac8",
		"#807dba", "#6a51a3", "#54278f", "#3f007d",
	}},
	"reds": {Name: "reds", Kind: SchemeKindSequential, Colors: []string{
		"#fff5f0", "#fee0d2", "#fcbba1", "#fc9272", "#fb6a4a",
		"#ef3b2c", "#cb181d", "#a50f15", "#67000d",
	}},

	// Sequential multi-hue — Brewer.
	"bugn": {Name: "bugn", Kind: SchemeKindSequential, Colors: []string{
		"#f7fcfd", "#e5f5f9", "#ccece6", "#99d8c9", "#66c2a4",
		"#41ae76", "#238b45", "#006d2c", "#00441b",
	}},
	"bupu": {Name: "bupu", Kind: SchemeKindSequential, Colors: []string{
		"#f7fcfd", "#e0ecf4", "#bfd3e6", "#9ebcda", "#8c96c6",
		"#8c6bb1", "#88419d", "#810f7c", "#4d004b",
	}},
	"gnbu": {Name: "gnbu", Kind: SchemeKindSequential, Colors: []string{
		"#f7fcf0", "#e0f3db", "#ccebc5", "#a8ddb5", "#7bccc4",
		"#4eb3d3", "#2b8cbe", "#0868ac", "#084081",
	}},
	"orrd": {Name: "orrd", Kind: SchemeKindSequential, Colors: []string{
		"#fff7ec", "#fee8c8", "#fdd49e", "#fdbb84", "#fc8d59",
		"#ef6548", "#d7301f", "#b30000", "#7f0000",
	}},
	"pubu": {Name: "pubu", Kind: SchemeKindSequential, Colors: []string{
		"#fff7fb", "#ece7f2", "#d0d1e6", "#a6bddb", "#74a9cf",
		"#3690c0", "#0570b0", "#045a8d", "#023858",
	}},
	"pubugn": {Name: "pubugn", Kind: SchemeKindSequential, Colors: []string{
		"#fff7fb", "#ece2f0", "#d0d1e6", "#a6bddb", "#67a9cf",
		"#3690c0", "#02818a", "#016c59", "#014636",
	}},
	"purd": {Name: "purd", Kind: SchemeKindSequential, Colors: []string{
		"#f7f4f9", "#e7e1ef", "#d4b9da", "#c994c7", "#df65b0",
		"#e7298a", "#ce1256", "#980043", "#67001f",
	}},
	"rdpu": {Name: "rdpu", Kind: SchemeKindSequential, Colors: []string{
		"#fff7f3", "#fde0dd", "#fcc5c0", "#fa9fb5", "#f768a1",
		"#dd3497", "#ae017e", "#7a0177", "#49006a",
	}},
	"ylgn": {Name: "ylgn", Kind: SchemeKindSequential, Colors: []string{
		"#ffffe5", "#f7fcb9", "#d9f0a3", "#addd8e", "#78c679",
		"#41ab5d", "#238443", "#006837", "#004529",
	}},
	"ylgnbu": {Name: "ylgnbu", Kind: SchemeKindSequential, Colors: []string{
		"#ffffd9", "#edf8b1", "#c7e9b4", "#7fcdbb", "#41b6c4",
		"#1d91c0", "#225ea8", "#253494", "#081d58",
	}},
	"ylorbr": {Name: "ylorbr", Kind: SchemeKindSequential, Colors: []string{
		"#ffffe5", "#fff7bc", "#fee391", "#fec44f", "#fe9929",
		"#ec7014", "#cc4c02", "#993404", "#662506",
	}},
	"ylorrd": {Name: "ylorrd", Kind: SchemeKindSequential, Colors: []string{
		"#ffffcc", "#ffeda0", "#fed976", "#feb24c", "#fd8d3c",
		"#fc4e2a", "#e31a1c", "#bd0026", "#800026",
	}},

	// Sequential perceptually-uniform — BIDS colormaps (9-stop resample).
	"viridis": {Name: "viridis", Kind: SchemeKindSequential, Colors: []string{
		"#440154", "#482878", "#3e4a89", "#31688e", "#26828e",
		"#1f9e89", "#35b779", "#6dcd59", "#fde725",
	}},
	"magma": {Name: "magma", Kind: SchemeKindSequential, Colors: []string{
		"#000004", "#1c1044", "#4f127b", "#812581", "#b5367a",
		"#e55964", "#fb8861", "#fec287", "#fcfdbf",
	}},
	"plasma": {Name: "plasma", Kind: SchemeKindSequential, Colors: []string{
		"#0d0887", "#5302a3", "#8b0aa5", "#b83289", "#db5c68",
		"#f48849", "#febc2a", "#fcce25", "#f0f921",
	}},
	"inferno": {Name: "inferno", Kind: SchemeKindSequential, Colors: []string{
		"#000004", "#1b0c41", "#4a0c6b", "#781c6d", "#a52c60",
		"#cf4446", "#ed6925", "#fb9a06", "#fcffa4",
	}},
	"cividis": {Name: "cividis", Kind: SchemeKindSequential, Colors: []string{
		"#00224e", "#123570", "#3b496c", "#575c6d", "#707173",
		"#8a8678", "#a59c74", "#c3b369", "#fee838",
	}},
	"turbo": {Name: "turbo", Kind: SchemeKindSequential, Colors: []string{
		"#30123b", "#4145ab", "#4675ed", "#39a2fc", "#1bcfd4",
		"#24eca6", "#61fc6c", "#a4fc3b", "#d1e834", "#f3c63a",
		"#fe9b2d", "#f36315", "#d93806", "#a91201", "#7a0403",
	}},
	"warm": {Name: "warm", Kind: SchemeKindSequential, Colors: []string{
		"#6e40aa", "#963db3", "#bf3caf", "#e4419d", "#fe4b83",
		"#ff5e63", "#ff7847", "#fb9633", "#e2b72f", "#aff05b",
	}},
	"cool": {Name: "cool", Kind: SchemeKindSequential, Colors: []string{
		"#6e40aa", "#6a53b1", "#6669b3", "#637fb1", "#6296a8",
		"#65aa9b", "#6dbd8a", "#7bcd76", "#8ed85f", "#a7e045",
	}},

	// Diverging — Brewer 9-class.
	"rdbu": {Name: "rdbu", Kind: SchemeKindDiverging, Colors: []string{
		"#b2182b", "#d6604d", "#f4a582", "#fddbc7", "#f7f7f7",
		"#d1e5f0", "#92c5de", "#4393c3", "#2166ac",
	}},
	"rdylbu": {Name: "rdylbu", Kind: SchemeKindDiverging, Colors: []string{
		"#d73027", "#f46d43", "#fdae61", "#fee090", "#ffffbf",
		"#e0f3f8", "#abd9e9", "#74add1", "#4575b4",
	}},
	"brbg": {Name: "brbg", Kind: SchemeKindDiverging, Colors: []string{
		"#8c510a", "#bf812d", "#dfc27d", "#f6e8c3", "#f5f5f5",
		"#c7eae5", "#80cdc1", "#35978f", "#01665e",
	}},
	"prgn": {Name: "prgn", Kind: SchemeKindDiverging, Colors: []string{
		"#762a83", "#9970ab", "#c2a5cf", "#e7d4e8", "#f7f7f7",
		"#d9f0d3", "#a6dba0", "#5aae61", "#1b7837",
	}},
	"piyg": {Name: "piyg", Kind: SchemeKindDiverging, Colors: []string{
		"#c51b7d", "#de77ae", "#f1b6da", "#fde0ef", "#f7f7f7",
		"#e6f5d0", "#b8e186", "#7fbc41", "#4d9221",
	}},
	"puor": {Name: "puor", Kind: SchemeKindDiverging, Colors: []string{
		"#b35806", "#e08214", "#fdb863", "#fee0b6", "#f7f7f7",
		"#d8daeb", "#b2abd2", "#8073ac", "#542788",
	}},
	"rdgy": {Name: "rdgy", Kind: SchemeKindDiverging, Colors: []string{
		"#b2182b", "#d6604d", "#f4a582", "#fddbc7", "#ffffff",
		"#e0e0e0", "#bababa", "#878787", "#4d4d4d",
	}},
	"rdylgn": {Name: "rdylgn", Kind: SchemeKindDiverging, Colors: []string{
		"#d73027", "#f46d43", "#fdae61", "#fee08b", "#ffffbf",
		"#d9ef8b", "#a6d96a", "#66bd63", "#1a9850",
	}},
	"spectral": {Name: "spectral", Kind: SchemeKindDiverging, Colors: []string{
		"#d53e4f", "#f46d43", "#fdae61", "#fee08b", "#ffffbf",
		"#e6f598", "#abdda4", "#66c2a5", "#3288bd",
	}},

	// Cyclic.
	"rainbow": {Name: "rainbow", Kind: SchemeKindCyclic, Colors: []string{
		"#6e40aa", "#bf3caf", "#fe4b83", "#ff7847", "#e2b72f",
		"#aff05b", "#52f667", "#1ddfa3", "#23abd8", "#4c6edb",
		"#6e40aa",
	}},
	"sinebow": {Name: "sinebow", Kind: SchemeKindCyclic, Colors: []string{
		"#ff4040", "#e78d0b", "#a7ea52", "#42fc8f", "#23dec1",
		"#3ba0fb", "#7e4fff", "#cb13ff", "#ff10aa", "#ff4040",
	}},
}

// SchemeByName returns the registered colors. Lookup is case-folded
// against the canonical lowercase form (so "Viridis" → "viridis").
func SchemeByName(name string) ([]string, bool) {
	if name == "" {
		return nil, false
	}
	if info, ok := builtinSchemes[canonicalSchemeKey(name)]; ok {
		return append([]string(nil), info.Colors...), true
	}
	return nil, false
}

// SchemeKind reports the registered kind. Returns "" + false when
// unknown.
func SchemeKind(name string) (string, bool) {
	if info, ok := builtinSchemes[canonicalSchemeKey(name)]; ok {
		return info.Kind, true
	}
	return "", false
}

// SchemeNames returns the registered scheme names in sorted order.
func SchemeNames() []string {
	out := make([]string, 0, len(builtinSchemes))
	for k := range builtinSchemes {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// IsSchemeRegistered reports whether either the built-in catalogue
// or the theme's per-theme override defines a scheme. Used by the
// PRISM_SPEC_028 validate rule.
func IsSchemeRegistered(t *Theme, name string) bool {
	if name == "" {
		return false
	}
	if t != nil {
		if _, ok := t.Schemes[name]; ok {
			return true
		}
	}
	_, ok := builtinSchemes[canonicalSchemeKey(name)]
	return ok
}

// canonicalSchemeKey lowercases the lookup key. Display names stay
// in the SchemeInfo.Name field for emit-side parity with d3.
func canonicalSchemeKey(name string) string {
	b := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}
