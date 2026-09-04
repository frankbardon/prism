package prism

import (
	"testing"

	"github.com/frankbardon/prism/custommark"
	"github.com/frankbardon/prism/encode/scene"
	"github.com/frankbardon/prism/table"
	"github.com/frankbardon/prism/theme"
)

// svgOnlyRenderer implements only SVGCustomRenderer — verifying a
// concrete CustomRenderer implementation is free to skip
// HTMLCustomRenderer entirely.
type svgOnlyRenderer struct{}

func (svgOnlyRenderer) RenderSVG(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return "<g/>", nil
}

// htmlOnlyRenderer implements only HTMLCustomRenderer — the mirror
// case of svgOnlyRenderer.
type htmlOnlyRenderer struct{}

func (htmlOnlyRenderer) RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return "<div></div>", nil
}

// bothRenderer implements both single-method interfaces.
type bothRenderer struct{}

func (bothRenderer) RenderSVG(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return "<g/>", nil
}

func (bothRenderer) RenderHTML(rows []table.Row, box scene.Box, tokens *theme.Theme) (string, error) {
	return "<div></div>", nil
}

// neitherRenderer implements neither interface.
type neitherRenderer struct{}

// TestRegisterCustomMarkAcceptsSVGOnly asserts a renderer implementing
// only RenderSVG registers successfully and type-asserts to
// SVGCustomRenderer but not HTMLCustomRenderer — the "optional method"
// contract the interface split exists to express.
func TestRegisterCustomMarkAcceptsSVGOnly(t *testing.T) {
	custommark.ResetForTest(t)
	const name = "test-svg-only"
	if err := RegisterCustomMark(name, svgOnlyRenderer{}); err != nil {
		t.Fatalf("RegisterCustomMark: %v", err)
	}

	got, ok := LookupCustomMark(name)
	if !ok {
		t.Fatalf("LookupCustomMark(%q): not found", name)
	}
	if _, ok := got.(SVGCustomRenderer); !ok {
		t.Errorf("registered renderer does not satisfy SVGCustomRenderer")
	}
	if _, ok := got.(HTMLCustomRenderer); ok {
		t.Errorf("registered renderer unexpectedly satisfies HTMLCustomRenderer")
	}
}

// TestRegisterCustomMarkAcceptsHTMLOnly mirrors
// TestRegisterCustomMarkAcceptsSVGOnly for the HTML-only case.
func TestRegisterCustomMarkAcceptsHTMLOnly(t *testing.T) {
	custommark.ResetForTest(t)
	const name = "test-html-only"
	if err := RegisterCustomMark(name, htmlOnlyRenderer{}); err != nil {
		t.Fatalf("RegisterCustomMark: %v", err)
	}

	got, ok := LookupCustomMark(name)
	if !ok {
		t.Fatalf("LookupCustomMark(%q): not found", name)
	}
	if _, ok := got.(HTMLCustomRenderer); !ok {
		t.Errorf("registered renderer does not satisfy HTMLCustomRenderer")
	}
	if _, ok := got.(SVGCustomRenderer); ok {
		t.Errorf("registered renderer unexpectedly satisfies SVGCustomRenderer")
	}
}

// TestRegisterCustomMarkAcceptsBoth asserts a renderer implementing
// both methods registers successfully and type-asserts to both
// interfaces.
func TestRegisterCustomMarkAcceptsBoth(t *testing.T) {
	custommark.ResetForTest(t)
	const name = "test-both"
	if err := RegisterCustomMark(name, bothRenderer{}); err != nil {
		t.Fatalf("RegisterCustomMark: %v", err)
	}

	got, ok := LookupCustomMark(name)
	if !ok {
		t.Fatalf("LookupCustomMark(%q): not found", name)
	}
	if _, ok := got.(SVGCustomRenderer); !ok {
		t.Errorf("registered renderer does not satisfy SVGCustomRenderer")
	}
	if _, ok := got.(HTMLCustomRenderer); !ok {
		t.Errorf("registered renderer does not satisfy HTMLCustomRenderer")
	}
}

// TestRegisterCustomMarkRejectsNeither asserts a renderer implementing
// neither single-method interface is rejected at registration time,
// and never appears in the registry.
func TestRegisterCustomMarkRejectsNeither(t *testing.T) {
	custommark.ResetForTest(t)
	const name = "test-neither"
	if err := RegisterCustomMark(name, neitherRenderer{}); err == nil {
		t.Fatalf("RegisterCustomMark: expected error, got nil")
	}
	if _, ok := LookupCustomMark(name); ok {
		t.Fatalf("LookupCustomMark(%q): unexpectedly found after rejected registration", name)
	}
}

// TestRegisterCustomMarkRejectsEmptyNameAndNil covers the remaining
// input-validation guards.
func TestRegisterCustomMarkRejectsEmptyNameAndNil(t *testing.T) {
	custommark.ResetForTest(t)
	if err := RegisterCustomMark("", svgOnlyRenderer{}); err == nil {
		t.Errorf("RegisterCustomMark(\"\", ...): expected error, got nil")
	}
	if err := RegisterCustomMark("test-nil", nil); err == nil {
		t.Errorf("RegisterCustomMark(name, nil): expected error, got nil")
	}
}

// TestLookupCustomMarkMissing asserts an unregistered name reports
// ok=false rather than panicking or returning a zero-value renderer
// silently.
func TestLookupCustomMarkMissing(t *testing.T) {
	custommark.ResetForTest(t)
	if _, ok := LookupCustomMark("does-not-exist"); ok {
		t.Errorf("LookupCustomMark: expected ok=false for unregistered name")
	}
}

// TestCustomMarkNamesSorted asserts CustomMarkNames reflects
// registrations in sorted order, mirroring theme.Names().
func TestCustomMarkNamesSorted(t *testing.T) {
	custommark.ResetForTest(t)
	names := []string{"test-names-b", "test-names-a", "test-names-c"}
	for _, n := range names {
		if err := RegisterCustomMark(n, svgOnlyRenderer{}); err != nil {
			t.Fatalf("RegisterCustomMark(%q): %v", n, err)
		}
	}
	got := CustomMarkNames()
	want := map[string]bool{"test-names-a": true, "test-names-b": true, "test-names-c": true}
	seen := 0
	prev := ""
	for _, n := range got {
		if want[n] {
			seen++
			if n < prev {
				t.Fatalf("CustomMarkNames() not sorted: %v", got)
			}
		}
		prev = n
	}
	if seen != len(names) {
		t.Fatalf("CustomMarkNames() = %v, missing some of %v", got, names)
	}
}
