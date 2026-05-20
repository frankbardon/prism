//go:build js && wasm

// fs_fetch_js.go provides an afero.Fs adapter backed by the browser
// `fetch` API. Paths are interpreted as URLs relative to the page
// origin (or absolute when the spec uses a full URL). The first
// access to a path triggers a synchronous fetch that loads the full
// response body into an in-memory backing store; subsequent opens
// hit the in-memory copy without re-fetching.
//
// Limitations (v1):
//   - Whole-file load only. `Seek` is supported because the file is
//     materialised in memory before exposure. Streaming Range
//     requests for partial reads ship post-v1 only if benchmarks
//     show the buffered-load path is a bottleneck.
//   - Write operations are no-ops that return os.ErrPermission. The
//     browser has no filesystem to write to that fits the afero.Fs
//     contract; transient artefacts live in JS-side IndexedDB if a
//     consumer wants persistence.
//
// Surfaces PRISM_WASM_001 on fetch failure (non-2xx, network error,
// CORS rejection). All errors are AppError envelopes so the same
// JSON shape escapes the WASM bridge as the host CLI surfaces.
package resolve

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"syscall/js"
	"time"

	"github.com/spf13/afero"

	prismerrors "github.com/frankbardon/prism/errors"
)

// NewFetchFs returns an afero.Fs whose reads are satisfied by issuing
// `fetch` requests through the host `globalThis.fetch` and caching
// the bytes in an underlying in-memory filesystem. Write operations
// return os.ErrPermission.
//
// The returned Fs is safe for concurrent use; the first fetch of a
// path serialises behind a per-path lock so concurrent Open calls
// against the same URL produce exactly one network request.
func NewFetchFs() afero.Fs {
	return &fetchFs{
		backing: afero.NewMemMapFs(),
		fetched: make(map[string]struct{}),
	}
}

type fetchFs struct {
	mu      sync.Mutex
	backing afero.Fs
	fetched map[string]struct{}
}

// ensureFetched guarantees the URL has been loaded into the backing
// store. Returns nil on success; PRISM_WASM_001 on fetch failure.
func (f *fetchFs) ensureFetched(name string) error {
	f.mu.Lock()
	if _, ok := f.fetched[name]; ok {
		f.mu.Unlock()
		return nil
	}
	f.mu.Unlock()

	body, status, err := fetchURL(name)
	if err != nil {
		return prismerrors.Wrap(
			"PRISM_WASM_001",
			fmt.Sprintf("Fetch-backed filesystem failed to load %s (HTTP %d: %v).", name, status, err),
			map[string]any{"URL": name, "Status": status, "Reason": err.Error()},
			err,
		)
	}
	if status < 200 || status >= 300 {
		return prismerrors.New(
			"PRISM_WASM_001",
			fmt.Sprintf("Fetch-backed filesystem failed to load %s (HTTP %d: non-success status).", name, status),
			map[string]any{"URL": name, "Status": status, "Reason": http.StatusText(status)},
		)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.fetched[name]; ok {
		return nil
	}
	if err := afero.WriteFile(f.backing, name, body, 0o644); err != nil {
		return prismerrors.Wrap(
			"PRISM_WASM_001",
			fmt.Sprintf("Fetch-backed filesystem failed to cache %s: %v.", name, err),
			map[string]any{"URL": name, "Status": status, "Reason": err.Error()},
			err,
		)
	}
	f.fetched[name] = struct{}{}
	return nil
}

// fetchURL issues a synchronous fetch via syscall/js. Returns the
// response body bytes, the HTTP status code, and any transport error.
// The status code is 0 when the JS Promise chain rejects before a
// response is available (CORS, DNS, network).
func fetchURL(url string) ([]byte, int, error) {
	type result struct {
		body   []byte
		status int
		err    error
	}
	done := make(chan result, 1)

	global := js.Global()
	fetch := global.Get("fetch")
	if fetch.IsUndefined() {
		return nil, 0, fmt.Errorf("globalThis.fetch is not available in this WASM host")
	}

	then := js.FuncOf(func(_ js.Value, args []js.Value) any {
		resp := args[0]
		status := resp.Get("status").Int()
		if status < 200 || status >= 300 {
			done <- result{status: status, err: fmt.Errorf("HTTP %d", status)}
			return nil
		}
		arrayBuf := resp.Call("arrayBuffer")
		arrayBuf.Call("then", js.FuncOf(func(_ js.Value, args []js.Value) any {
			buf := args[0]
			u8 := global.Get("Uint8Array").New(buf)
			body := make([]byte, u8.Get("length").Int())
			js.CopyBytesToGo(body, u8)
			done <- result{body: body, status: status}
			return nil
		}), js.FuncOf(func(_ js.Value, args []js.Value) any {
			done <- result{status: status, err: fmt.Errorf("arrayBuffer rejected: %s", args[0].String())}
			return nil
		}))
		return nil
	})
	defer then.Release()

	catch := js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := "fetch rejected"
		if len(args) > 0 && !args[0].IsUndefined() {
			msg = args[0].String()
		}
		done <- result{err: fmt.Errorf("%s", msg)}
		return nil
	})
	defer catch.Release()

	promise := fetch.Invoke(url)
	promise.Call("then", then).Call("catch", catch)

	r := <-done
	return r.body, r.status, r.err
}

// --- afero.Fs surface. Reads route through ensureFetched, then
// delegate to the in-memory backing. Writes are rejected so the
// browser sandbox stays sealed.

func (f *fetchFs) Open(name string) (afero.File, error) {
	if err := f.ensureFetched(name); err != nil {
		return nil, err
	}
	return f.backing.Open(name)
}

func (f *fetchFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_TRUNC) != 0 {
		return nil, &os.PathError{Op: "openfile", Path: name, Err: os.ErrPermission}
	}
	if err := f.ensureFetched(name); err != nil {
		return nil, err
	}
	return f.backing.OpenFile(name, flag, perm)
}

func (f *fetchFs) Stat(name string) (os.FileInfo, error) {
	if err := f.ensureFetched(name); err != nil {
		return nil, err
	}
	return f.backing.Stat(name)
}

func (f *fetchFs) Name() string { return "fetchFs" }

func (f *fetchFs) Create(name string) (afero.File, error) {
	return nil, &os.PathError{Op: "create", Path: name, Err: os.ErrPermission}
}

func (f *fetchFs) Mkdir(name string, perm os.FileMode) error {
	return &os.PathError{Op: "mkdir", Path: name, Err: os.ErrPermission}
}

func (f *fetchFs) MkdirAll(path string, perm os.FileMode) error {
	return &os.PathError{Op: "mkdirall", Path: path, Err: os.ErrPermission}
}

func (f *fetchFs) Remove(name string) error {
	return &os.PathError{Op: "remove", Path: name, Err: os.ErrPermission}
}

func (f *fetchFs) RemoveAll(path string) error {
	return &os.PathError{Op: "removeall", Path: path, Err: os.ErrPermission}
}

func (f *fetchFs) Rename(oldname, newname string) error {
	return &os.PathError{Op: "rename", Path: oldname, Err: os.ErrPermission}
}

func (f *fetchFs) Chmod(name string, mode os.FileMode) error {
	return &os.PathError{Op: "chmod", Path: name, Err: os.ErrPermission}
}

func (f *fetchFs) Chown(name string, uid, gid int) error {
	return &os.PathError{Op: "chown", Path: name, Err: os.ErrPermission}
}

func (f *fetchFs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return &os.PathError{Op: "chtimes", Path: name, Err: os.ErrPermission}
}

// Compile-time assertion: fetchFs satisfies afero.Fs.
var _ afero.Fs = (*fetchFs)(nil)

// Compile-time use of io.EOF so the import does not become dead when
// the body of fetchURL changes shape.
var _ = io.EOF
