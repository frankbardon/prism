package theme

import "sort"

// registry holds the named themes registered at init time.
var registry = map[string]*Theme{}

// Register adds (or replaces) a theme under name. Safe to call from
// any package's init() block.
func Register(name string, t *Theme) {
	if t == nil {
		return
	}
	cp := t.Clone()
	cp.Name = name
	registry[name] = cp
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
	Register("light", lightTheme())
	Register("dark", darkTheme())
	Register("print", printTheme())
	Register("high_contrast", highContrastTheme())
	Register("colorblind", colorblindTheme())
}
