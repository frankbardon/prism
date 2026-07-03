//go:build !wasm

// Package vfs is Prism's filesystem seam.
//
// On host builds (this file) it is a thin alias over
// github.com/spf13/afero, so every existing caller and test that passes
// an afero.Fs (afero.NewOsFs(), afero.NewMemMapFs(), …) keeps compiling
// unchanged: vfs.Fs *is* afero.Fs here.
//
// On the wasm target (vfs_wasm.go) it is a native, afero-free interface.
// afero's root package imports net/http via its HttpFs, and TinyGo's
// net/http stdlib does not compile for js/wasm; routing the resolve /
// plan / validate filesystem plumbing through vfs keeps afero (and thus
// net/http) out of the WASM import graph entirely. Prism reads no source
// bytes through the filesystem in the browser — data arrives as inline
// data.values or via the JS DataResolver — so the wasm Fs is a no-op.
package vfs

import (
	"os"

	"github.com/spf13/afero"
)

// Fs is the filesystem interface Prism plumbs through its resolve /
// plan / validate seams. On host it aliases afero.Fs.
type Fs = afero.Fs

// File is a single open file. On host it aliases afero.File.
type File = afero.File

// OsFs returns the OS-backed filesystem.
func OsFs() Fs { return afero.NewOsFs() }

// MemFs returns an in-memory filesystem.
func MemFs() Fs { return afero.NewMemMapFs() }

// ReadFile reads the named file from fsys.
func ReadFile(fsys Fs, name string) ([]byte, error) { return afero.ReadFile(fsys, name) }

// WriteFile writes data to the named file in fsys.
func WriteFile(fsys Fs, name string, data []byte, perm os.FileMode) error {
	return afero.WriteFile(fsys, name, data, perm)
}
