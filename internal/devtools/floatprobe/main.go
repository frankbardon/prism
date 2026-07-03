//go:build js && wasm

// Command floatprobe is a throwaway js/wasm entrypoint used only by the
// E5-S2 float-parity test. It is NOT part of the shipping wasm bundle
// (that is cmd/prismwasm); it exists so render.FormatFloat can be
// exercised inside a real wasm module built by BOTH the standard Go
// toolchain AND TinyGo, and the two outputs diffed against the host.
//
// It publishes the render.FormatFloat rendering of the shared
// floatcorpus.Values() corpus as a plain string on
// globalThis.prismFloatProbe, then parks so the module stays live while
// the Node runner reads the value.
package main

import (
	"syscall/js"

	"github.com/frankbardon/prism/internal/devtools/floatcorpus"
)

func main() {
	js.Global().Set("prismFloatProbe", floatcorpus.Format())
	select {}
}
