package theme

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadFile reads a theme JSON from disk. When the file's `base`
// field is set to a registered theme name, the file's other fields
// merge sparsely on top of that base. Standalone (base == "") returns
// the theme as declared.
func LoadFile(path string) (*Theme, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("theme.LoadFile %s: %w", path, err)
	}
	return LoadBytes(body)
}

// LoadBytes parses a theme JSON blob. Same merge semantics as
// LoadFile.
func LoadBytes(body []byte) (*Theme, error) {
	var t Theme
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("theme.LoadBytes: %w", err)
	}
	if t.Base == "" {
		return &t, nil
	}
	base, ok := Get(t.Base)
	if !ok {
		return nil, fmt.Errorf("theme.LoadBytes: base theme %q is not registered", t.Base)
	}
	return Merge(base, &t), nil
}

// Merge returns a new Theme that combines base with the non-zero
// fields of override. String/list zero values inherit from base.
func Merge(base, override *Theme) *Theme {
	if base == nil {
		return override.Clone()
	}
	out := base.Clone()
	if override == nil {
		return out
	}
	if override.Name != "" {
		out.Name = override.Name
	}
	if override.AxisColor != "" {
		out.AxisColor = override.AxisColor
	}
	if override.GridColor != "" {
		out.GridColor = override.GridColor
	}
	if override.TextColor != "" {
		out.TextColor = override.TextColor
	}
	if override.BackgroundColor != "" {
		out.BackgroundColor = override.BackgroundColor
	}
	if override.FontSans != "" {
		out.FontSans = override.FontSans
	}
	if override.FontMono != "" {
		out.FontMono = override.FontMono
	}
	if override.FontSizeLabel != 0 {
		out.FontSizeLabel = override.FontSizeLabel
	}
	if override.FontSizeTitle != 0 {
		out.FontSizeTitle = override.FontSizeTitle
	}
	if override.FontSizeAxisTitle != 0 {
		out.FontSizeAxisTitle = override.FontSizeAxisTitle
	}
	if len(override.ColorSchemeCategorical) > 0 {
		out.ColorSchemeCategorical = append([]string(nil), override.ColorSchemeCategorical...)
	}
	if len(override.ColorSchemeSequential) > 0 {
		out.ColorSchemeSequential = append([]string(nil), override.ColorSchemeSequential...)
	}
	return out
}
