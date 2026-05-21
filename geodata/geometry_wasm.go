//go:build js && wasm

package geodata

import (
	"errors"
	"fmt"
	"sync"
	"syscall/js"
)

// bundleURL is set by SetBundleURL or auto-derived on first fetch from
// document.currentScript / location.origin + "/static/prism/geodata/".
var (
	bundleMu    sync.RWMutex
	bundleURL   string
	primedBytes = map[Tier][]byte{}
)

// SetBundleURL configures the base URL the WASM build fetches tier
// archives from. URL must end with "/" — the loader appends "<tier>.geo.json".
//
// Callers exposed via the JS surface: prism.geo.setBundleURL(url).
// Auto-derived from location.origin when unset.
func SetBundleURL(url string) {
	bundleMu.Lock()
	defer bundleMu.Unlock()
	bundleURL = url
}

// PrimeTier lets browser code seed a tier archive without a fetch
// roundtrip (e.g. when the host page already inlined the bytes). The
// bytes are stored verbatim and decoded lazily on first Lookup.
func PrimeTier(tier Tier, raw []byte) {
	bundleMu.Lock()
	defer bundleMu.Unlock()
	primedBytes[tier] = raw
}

func platformTierLoader(tier Tier) ([]byte, error) {
	bundleMu.RLock()
	if raw, ok := primedBytes[tier]; ok {
		bundleMu.RUnlock()
		return raw, nil
	}
	url := bundleURL
	bundleMu.RUnlock()

	if url == "" {
		url = deriveDefaultBundleURL()
	}
	if url == "" {
		return nil, fmt.Errorf("geodata: bundle URL not set (call prism.geo.setBundleURL or include data-prism-geodata-url on the embed)")
	}
	return fetchTier(url, tier)
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

func fetchTier(base string, tier Tier) ([]byte, error) {
	full := base + string(tier) + ".geo.json"
	fetchFn := js.Global().Get("fetch")
	if !fetchFn.Truthy() {
		return nil, errors.New("geodata: fetch() unavailable in this JS environment")
	}
	ch := make(chan struct {
		bytes []byte
		err   error
	}, 1)
	then := js.FuncOf(func(_ js.Value, args []js.Value) any {
		resp := args[0]
		if !resp.Get("ok").Bool() {
			ch <- struct {
				bytes []byte
				err   error
			}{nil, fmt.Errorf("geodata: fetch %s: HTTP %d", full, resp.Get("status").Int())}
			return nil
		}
		buf := resp.Call("arrayBuffer")
		buf.Call("then", js.FuncOf(func(_ js.Value, bufArgs []js.Value) any {
			ab := bufArgs[0]
			u8 := js.Global().Get("Uint8Array").New(ab)
			b := make([]byte, u8.Get("length").Int())
			js.CopyBytesToGo(b, u8)
			ch <- struct {
				bytes []byte
				err   error
			}{b, nil}
			return nil
		}))
		return nil
	})
	defer then.Release()
	catch := js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := "fetch failed"
		if len(args) > 0 && args[0].Truthy() {
			msg = args[0].Get("message").String()
		}
		ch <- struct {
			bytes []byte
			err   error
		}{nil, fmt.Errorf("geodata: fetch %s: %s", full, msg)}
		return nil
	})
	defer catch.Release()
	js.Global().Call("fetch", full).Call("then", then).Call("catch", catch)
	result := <-ch
	return result.bytes, result.err
}
