package observability

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/frankbardon/prism/plan"
)

// fakeBridge records every StartSpan call + the matching span-end
// (with the error passed). Safe for concurrent use; tests assert
// against the slices after exec.
type fakeBridge struct {
	mu      sync.Mutex
	started []string
	ended   []endRecord
}

type endRecord struct {
	name string
	err  error
}

func (f *fakeBridge) StartSpan(ctx context.Context, name string) (context.Context, func(error)) {
	f.mu.Lock()
	f.started = append(f.started, name)
	f.mu.Unlock()
	return ctx, func(err error) {
		f.mu.Lock()
		f.ended = append(f.ended, endRecord{name: name, err: err})
		f.mu.Unlock()
	}
}

// TestObservabilityDisabledByDefault asserts that without the env
// flag set, Hooks() returns the zero ExecOpts and no spans fire.
func TestObservabilityDisabledByDefault(t *testing.T) {
	t.Setenv(EnabledEnvVar, "")
	fb := &fakeBridge{}
	Register(fb)
	t.Cleanup(func() { Register(nil) })

	opts := Hooks()
	if opts.OnNodeStart != nil || opts.OnNodeDone != nil {
		t.Fatalf("Hooks() returned populated callbacks while disabled")
	}
	if len(fb.started) != 0 {
		t.Errorf("StartSpan called %d times; want 0", len(fb.started))
	}
}

// TestObservabilityEnabledWiresBridge asserts that with the env
// flag + a registered bridge, executor hooks call StartSpan +
// the end callback per node.
func TestObservabilityEnabledWiresBridge(t *testing.T) {
	t.Setenv(EnabledEnvVar, "1")
	fb := &fakeBridge{}
	Register(fb)
	t.Cleanup(func() { Register(nil) })

	opts := Hooks()
	if opts.OnNodeStart == nil || opts.OnNodeDone == nil {
		t.Fatalf("Hooks() returned nil callbacks while enabled")
	}

	// Simulate the executor calling start + done.
	opts.OnNodeStart(plan.NodeID("source-0"))
	opts.OnNodeStart(plan.NodeID("filter-1"))
	opts.OnNodeDone(plan.NodeID("source-0"), 5*time.Millisecond, nil)
	opts.OnNodeDone(plan.NodeID("filter-1"), 7*time.Millisecond, nil)

	if len(fb.started) != 2 {
		t.Fatalf("StartSpan called %d times; want 2", len(fb.started))
	}
	if fb.started[0] != "prism.node.source-0" || fb.started[1] != "prism.node.filter-1" {
		t.Errorf("started spans = %v", fb.started)
	}
	if len(fb.ended) != 2 {
		t.Fatalf("end called %d times; want 2", len(fb.ended))
	}
}

// TestObservabilityEnabledNoBridge asserts the hooks no-op when the
// env is set but no bridge is registered.
func TestObservabilityEnabledNoBridge(t *testing.T) {
	t.Setenv(EnabledEnvVar, "1")
	Register(nil)

	opts := Hooks()
	if opts.OnNodeStart == nil || opts.OnNodeDone == nil {
		t.Fatalf("Hooks() returned nil callbacks while enabled")
	}
	// Should not panic without a bridge.
	opts.OnNodeStart(plan.NodeID("x"))
	opts.OnNodeDone(plan.NodeID("x"), 0, nil)
}

// TestObservabilityRegisteredHelper exercises Registered().
func TestObservabilityRegisteredHelper(t *testing.T) {
	Register(nil)
	if Registered() {
		t.Fatal("Registered() true after Register(nil)")
	}
	Register(&fakeBridge{})
	if !Registered() {
		t.Fatal("Registered() false after Register(non-nil)")
	}
	Register(nil)
	if Registered() {
		t.Fatal("Registered() true after second Register(nil)")
	}
}

// TestObservabilityErrorPropagatesToEnd asserts node errors flow
// to the span-end callback.
func TestObservabilityErrorPropagatesToEnd(t *testing.T) {
	t.Setenv(EnabledEnvVar, "1")
	fb := &fakeBridge{}
	Register(fb)
	t.Cleanup(func() { Register(nil) })

	opts := Hooks()
	opts.OnNodeStart(plan.NodeID("bad"))
	opts.OnNodeDone(plan.NodeID("bad"), 0, errSentinel)

	if len(fb.ended) != 1 || fb.ended[0].err != errSentinel {
		t.Fatalf("end records = %v", fb.ended)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

var errSentinel error = errString("sentinel")
