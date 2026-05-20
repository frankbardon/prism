// Package observability ships an opt-in OpenTelemetry adapter that
// bridges Prism's executor hooks (P03's plan.ExecOpts.OnNodeStart /
// OnNodeDone) to an OTel-emitting Bridge. Per D086 the package
// imports zero OTel SDK code; integrators register their own
// SDK-backed Bridge implementation.
//
// Wiring:
//
//	import "github.com/frankbardon/prism/internal/observability"
//
//	// In the consumer process (or in tests):
//	observability.Register(&myOtelBridge{tracer: provider.Tracer("prism")})
//	os.Setenv("PRISM_OTEL_ENABLED", "1")
//
//	// In the Twirp handler or executor wiring:
//	opts := observability.Hooks()
//	// merge into plan.ExecOpts before calling plan.Execute
//
// When PRISM_OTEL_ENABLED is unset (or != "1") OR no Bridge has
// been registered, Hooks() returns the zero plan.ExecOpts and the
// executor sees no observability overhead.
package observability

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/frankbardon/prism/plan"
)

// Bridge is the OTel adapter interface. Integrators implement this
// once against their preferred OTel SDK and call Register at process
// init. StartSpan returns the (possibly-augmented) context and a
// span-end callback that takes an optional error.
type Bridge interface {
	StartSpan(ctx context.Context, name string) (context.Context, func(error))
}

// EnabledEnvVar is the gating environment variable. Set to "1" to
// arm the executor hooks.
const EnabledEnvVar = "PRISM_OTEL_ENABLED"

var (
	mu     sync.RWMutex
	active Bridge
)

// Register installs a Bridge. Passing nil clears the previous
// registration. Safe for concurrent callers.
func Register(b Bridge) {
	mu.Lock()
	active = b
	mu.Unlock()
}

// Registered returns true when a Bridge is currently installed.
// Tests use it to assert opt-in pre-conditions.
func Registered() bool {
	mu.RLock()
	defer mu.RUnlock()
	return active != nil
}

// Enabled reports whether the env gate is set. Decoupled from
// Registered() so tests can exercise each gate independently.
func Enabled() bool {
	return os.Getenv(EnabledEnvVar) == "1"
}

// Hooks returns a plan.ExecOpts populated with OnNodeStart /
// OnNodeDone callbacks that emit OTel spans through the active
// Bridge. Returns the zero ExecOpts when either gate fails (env
// unset or no Bridge); the caller can merge unconditionally.
func Hooks() plan.ExecOpts {
	if !Enabled() {
		return plan.ExecOpts{}
	}

	// We capture per-node span-end callbacks in a small map keyed by
	// NodeID. The map is per-Hooks() call (one ExecOpts per Execute),
	// so the mutex is only held during start/done bookkeeping.
	type pending struct {
		end func(error)
	}
	var (
		lk       sync.Mutex
		spans    = map[plan.NodeID]pending{}
	)

	return plan.ExecOpts{
		OnNodeStart: func(id plan.NodeID) {
			mu.RLock()
			b := active
			mu.RUnlock()
			if b == nil {
				return
			}
			_, end := b.StartSpan(context.Background(), "prism.node."+string(id))
			lk.Lock()
			spans[id] = pending{end: end}
			lk.Unlock()
		},
		OnNodeDone: func(id plan.NodeID, _ time.Duration, err error) {
			lk.Lock()
			p, ok := spans[id]
			delete(spans, id)
			lk.Unlock()
			if !ok || p.end == nil {
				return
			}
			p.end(err)
		},
	}
}
