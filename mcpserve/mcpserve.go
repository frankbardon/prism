//go:build !js

// Package mcpserve exposes Prism's Model Context Protocol server as a public
// entry point.
//
// The MCP wiring lives in the SDK-free mcp/ core (the transport-neutral tool
// catalog) plus the mcp/gosdk adapter, which mounts that catalog onto a
// server the caller builds; neither constructs or runs a server. This package
// is the missing runner: it lets an embedder serve a Prism MCP from its own
// process — over any io pair, or over stdio — without shelling out to the
// `prism` binary.
//
// Serving in-process is the only way a host can surface a configured Prism:
// the dataset registry, the afero filesystem seam, and the executor hooks set
// on the caller's *rpc.PrismServer, plus an on-disk example corpus supplied
// through Options, are all exposed verbatim to the MCP client. The stock
// binary can only offer what its flags reach.
package mcpserve

import (
	"context"
	"fmt"
	"io"
	"os"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/afero"

	mcpcore "github.com/frankbardon/prism/mcp"
	"github.com/frankbardon/prism/mcp/gosdk"
	"github.com/frankbardon/prism/rpc"
)

// serverName is the MCP server identity reported during initialize.
const serverName = "prism"

// defaultVersion is the advertised server version when Options.Version is empty.
const defaultVersion = "1.0.0"

// nopWriteCloser adapts an io.Writer to io.WriteCloser; go-sdk's IOTransport
// owns its streams via Close, but Serve's caller owns the lifetime of out.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// Options configures the served MCP server.
type Options struct {
	// ExamplesRoot points prism_examples_search at an on-disk directory to
	// walk instead of the embedded example corpus. Empty serves the embedded
	// corpus.
	ExamplesRoot string

	// ExamplesFS is the filesystem ExamplesRoot is read through. Only
	// consulted when ExamplesRoot is non-empty; nil defaults to the OS
	// filesystem.
	ExamplesFS afero.Fs

	// Version is the server identity advertised during initialize and threaded
	// into the adapter Config. Defaults to defaultVersion when empty.
	Version string
}

// newServer builds a bare go-sdk server and mounts the full Prism surface onto
// it through the single registration path (the gosdk adapter).
func newServer(facade *rpc.PrismServer, opts Options) (*mcpsdk.Server, error) {
	version := opts.Version
	if version == "" {
		version = defaultVersion
	}
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    serverName,
		Version: version,
	}, nil)
	if err := gosdk.Register(srv, facade, mcpcore.Config{
		ServerName:   serverName,
		Version:      version,
		ExamplesRoot: opts.ExamplesRoot,
		ExamplesFS:   opts.ExamplesFS,
	}); err != nil {
		return nil, fmt.Errorf("registering mcp surface: %w", err)
	}
	return srv, nil
}

// Serve runs an MCP server bound to facade, reading JSON-RPC requests from in
// and writing responses to out. It blocks until ctx is cancelled or a
// transport error occurs. The dataset registry, filesystem, and executor hooks
// configured on facade are exposed verbatim; a nil facade serves the zero-value
// server (empty registry, OS filesystem, no hooks).
func Serve(ctx context.Context, facade *rpc.PrismServer, opts Options, in io.Reader, out io.Writer) error {
	srv, err := newServer(facade, opts)
	if err != nil {
		return err
	}
	transport := &mcpsdk.IOTransport{
		Reader: io.NopCloser(in),
		Writer: nopWriteCloser{out},
	}
	return srv.Run(ctx, transport)
}

// ServeStdio is Serve over the process's stdin/stdout — the transport MCP
// clients use when they spawn the server as a subprocess. It blocks until
// stdin closes or the client disconnects.
func ServeStdio(facade *rpc.PrismServer, opts Options) error {
	return Serve(context.Background(), facade, opts, os.Stdin, os.Stdout)
}
