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

// TestLookupWithJSFallbackDelegatesToGoRegistryOnHost is E2-S5's host-
// build coverage for the new seam: on a non-WASM build, jsLookup
// (js_bridge_other.go) always misses, so LookupWithJSFallback must
// behave IDENTICALLY to Lookup — both for a name that IS registered
// (Go-side hit) and one that is not (miss, since there is no JS
// runtime on host to fall back to). The WASM-side half of the
// contract (a name resolving ONLY via the JS registry) is covered by
// internal/devtools' TinyGo custom-mark smoke test, which is the only
// place a real JS runtime is available to register into.
func TestLookupWithJSFallbackDelegatesToGoRegistryOnHost(t *testing.T) {
	custommark.ResetForTest(t)
	const name = "js-fallback-host-test"

	if _, ok := custommark.LookupWithJSFallback(name); ok {
		t.Fatalf("LookupWithJSFallback(%q): unexpectedly found before registration", name)
	}

	if err := custommark.Register(name, svgOnly{}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, ok := custommark.LookupWithJSFallback(name)
	if !ok {
		t.Fatalf("LookupWithJSFallback(%q): not found after Register", name)
	}
	if _, ok := got.(custommark.SVGCustomRenderer); !ok {
		t.Errorf("LookupWithJSFallback(%q) renderer does not satisfy SVGCustomRenderer", name)
	}
}

// TestResetForTestPreventsRegistryBleed is E2-S4's acceptance
// criterion for the shared test-isolation helper: two subtests
// register a mark under the SAME name, and the second subtest must
// not see the first's leftover registration, even though the first
// subtest never calls Unregister itself — ResetForTest's t.Cleanup
// must handle that automatically.
func TestResetForTestPreventsRegistryBleed(t *testing.T) {
	const name = "reset-for-test-bleed"

	t.Run("first", func(t *testing.T) {
		custommark.ResetForTest(t)
		if err := custommark.Register(name, svgOnly{}); err != nil {
			t.Fatalf("Register: %v", err)
		}
		if _, ok := custommark.Lookup(name); !ok {
			t.Fatalf("Lookup(%q): expected registration to be visible within the same test", name)
		}
		// On purpose, no explicit Unregister call here — proving
		// ResetForTest's cleanup, not test discipline, is what
		// prevents the bleed into the "second" subtest below.
	})

	t.Run("second", func(t *testing.T) {
		custommark.ResetForTest(t)
		if _, ok := custommark.Lookup(name); ok {
			t.Fatalf("Lookup(%q): unexpectedly saw the first subtest's leftover registration", name)
		}
		if err := custommark.Register(name, svgOnly{}); err != nil {
			t.Fatalf("Register: %v", err)
		}
		if _, ok := custommark.Lookup(name); !ok {
			t.Fatalf("Lookup(%q): expected own registration to be visible", name)
		}
	})
}

// TestResetForTestRestoresPriorState asserts ResetForTest restores
// the registry to exactly what it held before the test ran (not just
// "empty"), so a reset test never clobbers state a caller outside the
// test binary's control legitimately relies on.
func TestResetForTestRestoresPriorState(t *testing.T) {
	const preexisting = "reset-for-test-preexisting"
	if err := custommark.Register(preexisting, svgOnly{}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	t.Cleanup(func() { custommark.Unregister(preexisting) })

	// A nested subtest exercises ResetForTest: it should see the
	// pre-existing registration cleared away for its own duration,
	// register something new, and have both effects undone once its
	// own cleanup runs (verified against the parent scope below).
	t.Run("inner", func(t *testing.T) {
		custommark.ResetForTest(t)
		if _, ok := custommark.Lookup(preexisting); ok {
			t.Fatalf("Lookup(%q): expected ResetForTest to clear pre-existing registrations for the test's duration", preexisting)
		}
		if err := custommark.Register("reset-for-test-inner-only", svgOnly{}); err != nil {
			t.Fatalf("Register: %v", err)
		}
	})

	if _, ok := custommark.Lookup(preexisting); !ok {
		t.Fatalf("Lookup(%q): expected the pre-existing registration to be restored after the inner test completed", preexisting)
	}
	if _, ok := custommark.Lookup("reset-for-test-inner-only"); ok {
		t.Fatalf("Lookup(%q): expected the inner test's own registration to be cleaned up after it completed", "reset-for-test-inner-only")
	}
}
