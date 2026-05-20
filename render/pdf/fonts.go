//go:build !js

package pdf

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/signintech/gopdf"
)

// Canonical font names registered into every PDF. Renderers reference
// these by name through gopdf.SetFont; the bundled .ttf files (or any
// override loaded via WithFontDir) supply the actual glyph bytes.
const (
	FontSansRegular = "prism-sans-regular"
	FontSansBold    = "prism-sans-bold"
	FontMonoRegular = "prism-mono-regular"
)

// embedded TTFs. Files committed under render/pdf/fonts/ per D089 —
// each under the 500KB budget the constraint imposes. The TTF subset
// embedding happens inside gopdf when AddTTFFontDataWithOption is
// called; what we ship here is the full file (gopdf picks the glyphs
// it sees referenced).
//
//go:embed fonts/Inter-Regular.ttf fonts/Inter-Bold.ttf fonts/JetBrainsMono-Regular.ttf
var embeddedFonts embed.FS

// loadFonts registers the three canonical fonts into pdf. When
// fontDir is non-empty, the renderer first tries to satisfy each
// canonical name from a .ttf file inside that directory before
// falling back to the embedded bundle. Filename match is
// case-insensitive against the canonical name with the ".ttf"
// suffix.
//
// Override directory naming:
//
//	prism-sans-regular.ttf  →  FontSansRegular
//	prism-sans-bold.ttf     →  FontSansBold
//	prism-mono-regular.ttf  →  FontMonoRegular
//
// Anything else in the directory is ignored. Missing canonical
// names fall through to the embedded set, so a partial override
// (e.g. only the sans regular) still produces a valid PDF.
func loadFonts(pdf *gopdf.GoPdf, fontDir string) error {
	loaders := []struct {
		name      string
		fallback  string
		styleBits int
	}{
		{FontSansRegular, "fonts/Inter-Regular.ttf", gopdf.Regular},
		{FontSansBold, "fonts/Inter-Bold.ttf", gopdf.Bold},
		{FontMonoRegular, "fonts/JetBrainsMono-Regular.ttf", gopdf.Regular},
	}

	for _, l := range loaders {
		var data []byte
		var err error
		if fontDir != "" {
			data, err = readFontOverride(fontDir, l.name)
			if err != nil {
				return fmt.Errorf("pdf.loadFonts: read override %s: %w", l.name, err)
			}
		}
		if data == nil {
			data, err = embeddedFonts.ReadFile(l.fallback)
			if err != nil {
				return fmt.Errorf("pdf.loadFonts: read embedded %s: %w", l.fallback, err)
			}
		}
		if err := pdf.AddTTFFontDataWithOption(l.name, data, gopdf.TtfOption{
			Style: l.styleBits,
		}); err != nil {
			return fmt.Errorf("pdf.loadFonts: register %s: %w", l.name, err)
		}
	}
	return nil
}

// readFontOverride reads <fontDir>/<canonicalName>.ttf if present and
// readable; returns (nil, nil) when missing so the caller falls back
// to the embedded set. Any unexpected error is returned verbatim.
func readFontOverride(fontDir, canonicalName string) ([]byte, error) {
	candidate := filepath.Join(fontDir, canonicalName+".ttf")
	info, err := os.Stat(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			// Try a case-insensitive walk as a courtesy — most
			// people would name the override file Inter-Regular.ttf
			// not prism-sans-regular.ttf, so let them.
			return scanFontDir(fontDir, canonicalName)
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("font override %s is a directory", candidate)
	}
	return os.ReadFile(candidate)
}

// scanFontDir looks for a .ttf file whose stem hints at the canonical
// name we want. Heuristic only: maps "sans-regular" → any .ttf whose
// lowercase stem contains both "regular" and one of {"inter","sans"};
// similar for the other two slots. Returns nil bytes when nothing
// matches so the embedded set wins.
func scanFontDir(fontDir, canonicalName string) ([]byte, error) {
	hints := map[string][2]string{
		FontSansRegular: {"regular", "inter"},
		FontSansBold:    {"bold", "inter"},
		FontMonoRegular: {"regular", "mono"},
	}
	want, ok := hints[canonicalName]
	if !ok {
		return nil, nil
	}
	var hit string
	err := fs.WalkDir(os.DirFS(fontDir), ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".ttf") {
			return nil
		}
		stem := strings.ToLower(strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)))
		if strings.Contains(stem, want[0]) && strings.Contains(stem, want[1]) {
			hit = filepath.Join(fontDir, p)
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if hit == "" {
		return nil, nil
	}
	return os.ReadFile(hit)
}
