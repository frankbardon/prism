package custommark_test

import (
	"testing"

	"github.com/frankbardon/prism/custommark"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

type svgOnly struct{}

func (svgOnly) RenderSVG(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return "<g/>", nil
}

// TestRegisterLookupUnregisterRoundTrip covers the register/lookup/
// unregister lifecycle this package moved from the root prism package
// in E2-S2 (to avoid a render/svg -> prism import cycle). The fuller
// behavioural coverage (accepts-SVG-only, accepts-HTML-only, rejects-
// neither, sorted Names, ...) lives in custom_mark_test.go against the
// root package's thin re-exports, which delegate here — this test
// just proves the underlying package works standalone.
func TestRegisterLookupUnregisterRoundTrip(t *testing.T) {
	const name = "custommark-pkg-test"
	if err := custommark.Register(name, svgOnly{}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	t.Cleanup(func() { custommark.Unregister(name) })

	got, ok := custommark.Lookup(name)
	if !ok {
		t.Fatalf("Lookup(%q): not found", name)
	}
	if _, ok := got.(custommark.SVGCustomRenderer); !ok {
		t.Errorf("registered renderer does not satisfy SVGCustomRenderer")
	}

	found := false
	for _, n := range custommark.Names() {
		if n == name {
			found = true
		}
	}
	if !found {
		t.Errorf("Names() = %v, missing %q", custommark.Names(), name)
	}

	custommark.Unregister(name)
	if _, ok := custommark.Lookup(name); ok {
		t.Errorf("Lookup(%q) after Unregister: unexpectedly found", name)
	}
}
