//go:build js && wasm

package geodata

import (
	"errors"
	"fmt"
	"sync"
	"syscall/js"
)

// bundleURL is the base URL fetch operations target. Set explicitly via
// SetBundleURL or auto-derived from location.origin on first FetchTier
// call.
var (
	bundleMu    sync.RWMutex
	bundleURL   string
	primedBytes = map[Tier][]byte{}
)

// SetBundleURL configures the base URL prism.geo.preload fetches tier
// archives from. URL must end with "/". Exposed to JS as
// prism.geo.setBundleURL(url).
func SetBundleURL(url string) {
	bundleMu.Lock()
	defer bundleMu.Unlock()
	bundleURL = url
}

// BundleURL returns the configured base URL, deriving a default from
// location.origin when none has been set explicitly. Returns "" only
// when document.location is unavailable.
func BundleURL() string {
	bundleMu.RLock()
	u := bundleURL
	bundleMu.RUnlock()
	if u != "" {
		return u
	}
	return deriveDefaultBundleURL()
}

// PrimeTier installs raw bytes for a tier so subsequent Lookups
// resolve without a network roundtrip. The async preload entry-point
// in cmd/prismwasm/geo.go calls this after the fetch settles.
func PrimeTier(tier Tier, raw []byte) {
	bundleMu.Lock()
	defer bundleMu.Unlock()
	primedBytes[tier] = raw
}

// platformTierLoader is the SYNCHRONOUS loader the memoryStore uses
// from inside Lookup. On WASM, fetching during a synchronous JS call
// deadlocks (JS can't run the fetch microtask while Go holds the
// thread). So this only returns primed bytes; callers must
// pre-populate via the async preload path.
func platformTierLoader(tier Tier) ([]byte, error) {
	bundleMu.RLock()
	raw, ok := primedBytes[tier]
	bundleMu.RUnlock()
	if ok {
		return raw, nil
	}
	return nil, fmt.Errorf("geodata: tier %q not loaded (call prism.geo.preload(%q) before encoding a geoshape mark)", tier, tier)
}

func deriveDefaultBundleURL() string {
	loc := js.Global().Get("location")
	if !loc.Truthy() {
		return ""
	}
	origin := loc.Get("origin")
	if !origin.Truthy() {
		return ""
	}
	return origin.String() + "/static/prism/geodata/"
}

// FetchTierBytes is the ASYNCHRONOUS network fetch the JS-facing
// Promise wrapper drives from inside a goroutine. The blocking channel
// receive is safe here because the caller (cmd/prismwasm/geo.go's
// Promise body) returns to JS immediately, freeing the event loop to
// process the fetch microtask. Once fetch resolves, the then
// callback fires (a separate JS-dispatched Go invocation), sends on
// the channel, and unblocks this goroutine.
//
// Callers should typically PrimeTier with the returned bytes so
// subsequent Lookups resolve synchronously without re-fetching.
func FetchTierBytes(tier Tier) ([]byte, error) {
	base := BundleURL()
	if base == "" {
		return nil, errors.New("geodata: bundle URL not set (call prism.geo.setBundleURL first)")
	}
	full := base + string(tier) + ".geo.json"

	fetchFn := js.Global().Get("fetch")
	if !fetchFn.Truthy() {
		return nil, errors.New("geodata: fetch() unavailable in this JS environment")
	}
	ch := make(chan fetchResult, 1)

	then := js.FuncOf(func(_ js.Value, args []js.Value) any {
		resp := args[0]
		if !resp.Get("ok").Bool() {
			ch <- fetchResult{err: fmt.Errorf("geodata: fetch %s: HTTP %d", full, resp.Get("status").Int())}
			return nil
		}
		bufThen := js.FuncOf(func(_ js.Value, bufArgs []js.Value) any {
			ab := bufArgs[0]
			u8 := js.Global().Get("Uint8Array").New(ab)
			b := make([]byte, u8.Get("length").Int())
			js.CopyBytesToGo(b, u8)
			ch <- fetchResult{bytes: b}
			return nil
		})
		resp.Call("arrayBuffer").Call("then", bufThen)
		// bufThen is intentionally not Released — releasing before the
		// promise resolves would invalidate the callback. The runtime
		// reclaims it when the page unloads.
		return nil
	})
	catch := js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := "fetch failed"
		if len(args) > 0 && args[0].Truthy() {
			msg = args[0].Get("message").String()
		}
		ch <- fetchResult{err: fmt.Errorf("geodata: fetch %s: %s", full, msg)}
		return nil
	})

	js.Global().Call("fetch", full).Call("then", then).Call("catch", catch)
	result := <-ch
	then.Release()
	catch.Release()
	return result.bytes, result.err
}

type fetchResult struct {
	bytes []byte
	err   error
}
