//go:build js && wasm

// fs_fetch_js.go supplies the browser build's build.Options.FS value.
//
// Before epic E4 this was a fetch-backed afero.Fs that loaded `.pulse`
// bytes over HTTP. Prism no longer reads `.pulse` in any build: browser
// data enters via inline `data.values` / `datasets.*.values` or the JS
// DataResolver registered through prism.setDataResolver. The filesystem
// seam is therefore vestigial in the browser — it is plumbed through
// build.Options.FS and the validator lookup but never dereferenced — so
// NewFetchFs returns vfs's no-op filesystem. Keeping the constructor
// preserves the wasm entry's call sites without pulling afero (and thus
// net/http, which TinyGo cannot compile for js/wasm) into the WASM
// import graph.
package resolve

import "github.com/frankbardon/prism/internal/vfs"

// NewFetchFs returns the browser build's filesystem seam: a no-op
// vfs.Fs. Prism reads no source bytes through the filesystem in the
// browser (see the package note above), so every operation on the
// returned Fs reports os.ErrPermission.
func NewFetchFs() vfs.Fs { return vfs.MemFs() }
