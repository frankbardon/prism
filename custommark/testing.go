package custommark

import "testing"

// ResetForTest snapshots the current custom-mark registry and
// arranges, via t.Cleanup, to restore that exact snapshot once the
// calling test (or subtest) finishes — clearing the live registry to
// empty in the meantime.
//
// Call it at the top of any test that registers a custom mark
// (directly via Register, or indirectly through the root prism
// package's RegisterCustomMark) to guarantee the test starts from a
// clean registry and can never leak its own registrations into a
// sibling test, regardless of whether that test also remembers to
// call Unregister itself.
//
// This is the shared test-isolation helper called for in
// .planning/html-renderer/interview.md's "Global registry deviation"
// risk section: the registry is process-global mutable state (an
// intentional deviation from the rest of the codebase's hermetic,
// dependency-threaded convention — see the package doc comment), so
// without an explicit reset a registration made by one test can bleed
// into an unrelated test that runs later in the same test binary.
//
// Safe to call from any package's tests (custommark's own, render/
// svg's, render/html's, the root prism package's, or a downstream
// consumer's) since the registry this resets is process-global and
// keyed by name only. Safe for concurrent use, though in practice
// tests calling this should not also run t.Parallel against each
// other, since they share the one process-global registry this reset
// clears.
func ResetForTest(t *testing.T) {
	t.Helper()

	mu.Lock()
	snapshot := make(map[string]CustomRenderer, len(registry))
	for name, renderer := range registry {
		snapshot[name] = renderer
	}
	clear(registry)
	mu.Unlock()

	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		clear(registry)
		for name, renderer := range snapshot {
			registry[name] = renderer
		}
	})
}
