//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/frankbardon/prism/geodata"
)

// buildGeoAPI returns an Object exposing the geo bundle controls:
//
//	prism.geo.setBundleURL(url)         — set base URL for fetch
//	prism.geo.primeTier(tier, bytes)    — inject bytes (Uint8Array)
//	prism.geo.preload(tier)             — Promise: fetch + prime
//
// preload MUST be awaited before the first execute() call that needs
// the tier — synchronous fetch from inside execute() deadlocks the
// WASM runtime (JS can't pump the fetch microtask while Go holds
// the event-loop thread).
func buildGeoAPI() js.Value {
	obj := js.Global().Get("Object").New()
	obj.Set("setBundleURL", js.FuncOf(geoSetBundleURL))
	obj.Set("primeTier", js.FuncOf(geoPrimeTier))
	obj.Set("preload", js.FuncOf(geoPreload))
	return obj
}

func geoSetBundleURL(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return errEnvelope("PRISM_WASM_001", "geo.setBundleURL(url): missing url argument")
	}
	geodata.SetBundleURL(args[0].String())
	return `{"ok":true}`
}

func geoPrimeTier(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return errEnvelope("PRISM_WASM_001", "geo.primeTier(tier, bytes): missing arguments")
	}
	tier := args[0].String()
	u8 := args[1]
	n := u8.Get("length").Int()
	buf := make([]byte, n)
	js.CopyBytesToGo(buf, u8)
	geodata.PrimeTier(geodata.Tier(tier), buf)
	return `{"ok":true}`
}

// geoPreload returns a Promise that resolves once the tier bytes have
// been fetched + primed. Runs the network roundtrip in a Go
// goroutine so the synchronous JS → Go invocation can return
// immediately and let the JS event loop run the fetch microtask.
func geoPreload(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return wrapInPromise(func() (any, error) {
			return nil, errFromMessage("geo.preload(tier): missing tier argument")
		})
	}
	tier := args[0].String()
	return wrapInPromise(func() (any, error) {
		raw, err := geodata.FetchTierBytes(geodata.Tier(tier))
		if err != nil {
			return nil, err
		}
		geodata.PrimeTier(geodata.Tier(tier), raw)
		return `{"ok":true}`, nil
	})
}

// wrapInPromise materialises a Promise whose body executes fn in a
// new goroutine. Successful results are passed to resolve verbatim;
// errors are converted into the standard PRISM_WASM_001 error
// envelope and passed to reject. The pattern follows the canonical
// syscall/js async bridging recipe (Go runtime cannot block inside
// a JS-synchronous call; goroutines wrapped in a Promise can).
func wrapInPromise(fn func() (any, error)) js.Value {
	handler := js.FuncOf(func(_ js.Value, resolveReject []js.Value) any {
		resolve := resolveReject[0]
		reject := resolveReject[1]
		go func() {
			result, err := fn()
			if err != nil {
				reject.Invoke(errFromError(err))
				return
			}
			resolve.Invoke(result)
		}()
		return nil
	})
	return js.Global().Get("Promise").New(handler)
}

// errFromMessage builds a synthetic *AppError-shaped JSON envelope for
// boundary errors that don't originate from the prismerrors catalog.
type stringErr string

func (s stringErr) Error() string { return string(s) }

func errFromMessage(msg string) error { return stringErr(msg) }
