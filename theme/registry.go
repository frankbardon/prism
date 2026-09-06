package theme

import "sort"

// registry holds the named themes registered at init time.
var registry = map[string]*Theme{}

// Register adds (or replaces) a theme under name. Safe to call from
// any package's init() block.
//
// Returns PRISM_THEME_FILTER_UNKNOWN (and leaves the registry
// unchanged) when t declares a style-block Filter reference that does
// not resolve to a key in t.Filters — an intentional departure from
// RangeSlot.Resolve's silent-fallback behavior; see theme/validate.go.
func Register(name string, t *Theme) error {
	if t == nil {
		return nil
	}
	if err := t.Validate(); err != nil {
		return err
	}
	cp := t.Clone()
	cp.Name = name
	registry[name] = cp
	return nil
}

// Get returns the named theme + true; (nil, false) when missing.
// Returned theme is a clone — callers may mutate freely.
func Get(name string) (*Theme, bool) {
	t, ok := registry[name]
	if !ok {
		return nil, false
	}
	return t.Clone(), true
}

// MustGet panics if the theme is not registered. Convenience for
// hard-coded tests + init code.
func MustGet(name string) *Theme {
	t, ok := Get(name)
	if !ok {
		panic("theme: missing " + name)
	}
	return t
}

// Names returns the registered theme names in sorted order.
func Names() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func init() {
	mustRegister("light", lightTheme())
	mustRegister("dark", darkTheme())
	mustRegister("print", printTheme())
	mustRegister("high_contrast", highContrastTheme())
	mustRegister("colorblind", colorblindTheme())
}

// mustRegister panics on a Register error — appropriate at init time
// for the built-in themes, which are compile-time-authored and never
// expected to fail validation.
func mustRegister(name string, t *Theme) {
	if err := Register(name, t); err != nil {
		panic("theme: " + name + ": " + err.Error())
	}
}
