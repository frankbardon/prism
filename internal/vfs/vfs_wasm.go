//go:build wasm

// See vfs_other.go for the package rationale. This is the native,
// afero-free implementation used for the wasm target so neither afero
// nor net/http enters the WASM import graph (afero's root package imports
// net/http, which TinyGo cannot compile for js/wasm). Prism reads no
// source bytes through the filesystem in the browser (data arrives as
// inline data.values or via the JS DataResolver), so the concrete Fs
// here is a no-op that refuses every operation; it exists only to
// satisfy the build.Options.FS / validator plumbing.
package vfs

import (
	"io"
	"os"
	"time"
)

// File mirrors the afero.File method set so host and wasm signatures
// stay identical. The wasm build never materialises one (every Fs
// operation below refuses), so no concrete implementation is required.
type File interface {
	io.Closer
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Writer
	io.WriterAt
	Name() string
	Readdir(count int) ([]os.FileInfo, error)
	Readdirnames(n int) ([]string, error)
	Stat() (os.FileInfo, error)
	Sync() error
	Truncate(size int64) error
	WriteString(s string) (int, error)
}

// Fs mirrors the afero.Fs method set.
type Fs interface {
	Create(name string) (File, error)
	Mkdir(name string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Open(name string) (File, error)
	OpenFile(name string, flag int, perm os.FileMode) (File, error)
	Remove(name string) error
	RemoveAll(path string) error
	Rename(oldname, newname string) error
	Stat(name string) (os.FileInfo, error)
	Name() string
	Chmod(name string, mode os.FileMode) error
	Chown(name string, uid, gid int) error
	Chtimes(name string, atime, mtime time.Time) error
}

// noopFs is the browser filesystem: it stores nothing and refuses every
// operation with os.ErrPermission. Prism's wasm build never reads source
// bytes through it.
type noopFs struct{}

var _ Fs = noopFs{}

func pathErr(op, name string) error {
	return &os.PathError{Op: op, Path: name, Err: os.ErrPermission}
}

func (noopFs) Create(name string) (File, error)          { return nil, pathErr("create", name) }
func (noopFs) Mkdir(name string, _ os.FileMode) error    { return pathErr("mkdir", name) }
func (noopFs) MkdirAll(path string, _ os.FileMode) error { return pathErr("mkdirall", path) }
func (noopFs) Open(name string) (File, error)            { return nil, pathErr("open", name) }
func (noopFs) OpenFile(name string, _ int, _ os.FileMode) (File, error) {
	return nil, pathErr("openfile", name)
}
func (noopFs) Remove(name string) error    { return pathErr("remove", name) }
func (noopFs) RemoveAll(path string) error { return pathErr("removeall", path) }
func (noopFs) Rename(oldname, _ string) error {
	return pathErr("rename", oldname)
}
func (noopFs) Stat(name string) (os.FileInfo, error)  { return nil, pathErr("stat", name) }
func (noopFs) Name() string                           { return "vfs.noopFs" }
func (noopFs) Chmod(name string, _ os.FileMode) error { return pathErr("chmod", name) }
func (noopFs) Chown(name string, _, _ int) error      { return pathErr("chown", name) }
func (noopFs) Chtimes(name string, _, _ time.Time) error {
	return pathErr("chtimes", name)
}

// OsFs returns the no-op browser filesystem (there is no OS filesystem
// in a WASM host).
func OsFs() Fs { return noopFs{} }

// MemFs returns the no-op browser filesystem.
func MemFs() Fs { return noopFs{} }

// ReadFile reads through the Fs interface. The browser Fs refuses, so
// this always errors — it exists for signature parity with host code
// (e.g. LoadDatasetRegistryFile) that never runs in the browser.
func ReadFile(fsys Fs, name string) ([]byte, error) {
	if fsys == nil {
		return nil, os.ErrInvalid
	}
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// WriteFile writes through the Fs interface (always refused on wasm).
func WriteFile(fsys Fs, name string, data []byte, _ os.FileMode) error {
	if fsys == nil {
		return os.ErrInvalid
	}
	f, err := fsys.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
