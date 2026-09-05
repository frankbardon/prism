//go:build !js && !wasm

// See js_bridge_js.go for the package rationale. This is the stub
// half of the build-tag-gated seam: every non-WASM build (the host CLI,
// rpc/, mcp/, every `go test` invocation) links this file instead, so
// syscall/js never enters the host import graph. jsLookup always
// reports "not found" here — the host build has no JS runtime to call
// into, so LookupWithJSFallback degrades to exactly Lookup's own
// behaviour.
package custommark

// jsLookup always misses on a non-WASM build. See js_bridge_js.go.
func jsLookup(_ string) (CustomRenderer, bool) {
	return nil, false
}
