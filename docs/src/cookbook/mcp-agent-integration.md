# Cookbook: MCP agent integration

Expose Prism to an LLM agent so it can plot, validate, describe, and
search example specs as tool calls. Prism ships its Model Context
Protocol surface four ways:

1. **The `prism mcp` CLI** — a ready-to-run stdio server (zero Go code).
2. **The SDK-free `mcp.Tools(cfg)` catalog** — mount Prism's tools on
   *your own* MCP server with **no Prism-supplied MCP SDK** in your build.
3. **The `mcpserve.Serve` / `ServeStdio` runner** — run a fully wired
   Prism MCP server inside *your* process, over an io pair or over
   stdio, without shelling out to the `prism` binary.
4. **The `mcp/gosdk.Register` one-call adapter** — graft all four tools
   plus the embedded example resources onto a
   [`modelcontextprotocol/go-sdk`](https://github.com/modelcontextprotocol/go-sdk)
   server you already own.

## Start the MCP server

```
prism mcp
```

Reads JSON-RPC frames on stdin, writes responses on stdout. Standard
MCP stdio transport, backed by the `modelcontextprotocol/go-sdk` runtime.
The same four tools are exposed regardless of which mounting path you use.

## Tools exposed

| Tool | Args | Returns |
|---|---|---|
| `prism_plot` | `{spec, format?}` | `{bytes (base64), mime, caption, warnings?}` |
| `prism_validate` | `{spec}` | `{ok, errors}` |
| `prism_describe` | `{spec}` | `{summary}` |
| `prism_examples_search` | `{query}` | `{examples: [{name, summary, spec}]}` |

`prism_plot` supports `svg` (default); `png` returns
`PRISM_RENDER_FORMAT_UNAVAILABLE`. `prism_examples_search` returns up to
five matches by substring on spec name + title.

## Configure a host

Add Prism to your agent host's MCP server config (Claude Desktop,
Cursor, Cody, etc.):

```json
{
  "mcpServers": {
    "prism": {
      "command": "prism",
      "args": ["mcp"]
    }
  }
}
```

## Worked invocation

The agent reasons: "user asked for brand-score chart" → invokes
`prism_plot({spec: ..., format: "svg"})` → receives base64 SVG bytes
+ a natural-language caption. The caption is generated from the
parsed spec (mark + encoding fields + dataset names).

For server-mode integrations (HTTP, not stdio), use the Twirp surface
at `prism serve --addr :8080`. Generated clients live under
`rpc/` — Go is built-in; protoc can regenerate for JS/Python/Rust.

## Embed Prism's tools in your own Go binary

The CLI is a thin adapter over the importable
`github.com/frankbardon/prism/mcp` core and the `mcpserve` runner that
wraps it. Pick the path that matches whether you want an MCP SDK in your
dependency graph, and how much of the server you want to own.

### SDK-free: mount the `Tools(cfg)` catalog

`mcp.Tools(cfg)` returns a slice of transport- and SDK-agnostic
`ToolDescriptor`s. Each carries the tool name, description, reflected
input/output JSON Schemas (as `json.RawMessage`), and a **type-erased
`Invoke`** that unmarshals raw arguments, calls the typed handler, and
returns the typed output as `any` with the facade's coded error verbatim.
Mount them on whatever MCP server you already run — Prism's core imports
**no MCP SDK at all**, so importing it pulls none into your build.

```go
import (
	"context"
	"encoding/json"

	"github.com/frankbardon/prism/mcp"
	"github.com/frankbardon/prism/rpc"
)

func mountPrism(facade *rpc.PrismServer) {
	cfg := mcp.Config{
		ServerName: "prism",
		Version:    "0.1.0",
		// ExamplesRoot left empty → serve the embedded example corpus.
		// Set it (plus ExamplesFS) to walk an on-disk directory instead.
	}

	for _, d := range mcp.Tools(cfg) {
		// d.Name, d.Description     — register on your server
		// d.InputSchema/.OutputSchema (json.RawMessage) — advertise to the agent
		// d.Invoke(ctx, facade, raw json.RawMessage) (any, error) — dispatch a call
		myServer.Register(d.Name, d.Description, d.InputSchema, d.OutputSchema,
			func(ctx context.Context, raw json.RawMessage) (any, error) {
				return d.Invoke(ctx, facade, raw)
			})
	}
}
```

The typed handlers (`mcp.PlotTool`, `mcp.ValidateTool`, `mcp.DescribeTool`,
`mcp.ExamplesSearchTool`) and their I/O structs (`mcp.PlotInput` /
`mcp.PlotOutput`, etc.) are exported too, if you prefer to call them
directly against an `*rpc.PrismServer` rather than through the
type-erased descriptors.

> **Import-firewall guarantee.** The `github.com/frankbardon/prism/mcp`
> core pulls in **no** MCP SDK. This is enforced by
> `internal/gates/mcp_firewall_test.go`, which fails the build if the
> package's transitive imports ever include one. Depending on the catalog
> never couples your binary to a particular MCP protocol library or version.

### In-process: run a server with `mcpserve`

`github.com/frankbardon/prism/mcpserve` is the runner the other two paths
leave out by design. The catalog and the `gosdk` adapter only *mount* tools;
neither constructs or runs a server. `mcpserve` does both: hand it a
configured `*rpc.PrismServer` and it builds a go-sdk server, registers the
full Prism surface through `gosdk.Register`, and serves it — so the dataset
registry, the afero filesystem seam, and the executor hooks you set on the
facade are all exposed verbatim to the agent. That is more than `prism mcp`
can offer, since the binary only surfaces what its flags reach.

`Serve(ctx, facade, opts, in, out)` runs over any `io.Reader` / `io.Writer`
pair and blocks until `ctx` is cancelled or the transport errors. For an
in-process client, wire it with two pipes:

```go
import (
	"context"
	"io"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/mcpserve"
	"github.com/frankbardon/prism/rpc"
)

// mountInProcess starts a Prism MCP server on a pair of pipes and hands back
// the ends your MCP client talks to: write JSON-RPC frames to reqs, read
// responses from resps. The returned channel carries the server's exit error.
func mountInProcess(ctx context.Context) (reqs io.WriteCloser, resps io.Reader, done <-chan error) {
	facade := &rpc.PrismServer{
		Fs: afero.NewOsFs(),
		// DatasetRegistry and ExecOpts are optional — the zero value works,
		// and a nil facade serves the zero-value server.
	}

	reqR, reqW := io.Pipe()   // client → server
	respR, respW := io.Pipe() // server → client

	exit := make(chan error, 1)
	go func() {
		defer respW.Close()
		exit <- mcpserve.Serve(ctx, facade, mcpserve.Options{
			Version: "1.2.3",
			// ExamplesRoot left empty → serve the embedded example corpus.
			// Set it (plus ExamplesFS) to walk an on-disk directory instead.
		}, reqR, respW)
	}()

	return reqW, respR, exit
}
```

`Serve` never closes `out` — the caller owns its lifetime, which is why the
goroutine above closes `respW` itself.

`ServeStdio(facade, opts)` is the same thing over the process's stdin and
stdout, taking no `ctx`; it blocks until stdin closes or the client
disconnects. It is a one-liner, and exactly what `prism mcp` runs:

```go
return mcpserve.ServeStdio(&rpc.PrismServer{Fs: afero.NewOsFs()}, mcpserve.Options{Version: "1.2.3"})
```

`Options` carries three fields: `Version` (the identity advertised during
`initialize`; defaults to `1.0.0` when empty), `ExamplesRoot`, and
`ExamplesFS`. The server name is always `prism`.

**Which one do I want?** Reach for `mcpserve` when you want a configured
Prism mounted in your own process and you do not care what the transport is
— it picks one for you. Reach for `gosdk.Register` (below) when you already
run a go-sdk server, when Prism's tools have to sit alongside your own, or
when you need a transport `mcpserve` does not hand you; `Register` grafts
onto the server you built, so the transport stays yours.

### go-sdk: graft everything with one `Register` call

If you already run (or want) a `modelcontextprotocol/go-sdk` server,
`github.com/frankbardon/prism/mcp/gosdk` mounts all four tools **and** the
embedded example specs (as read-only `prism://examples/<stem>` resources)
in a single call. This is the shape `mcpserve` wraps, spelled out — build a
bare server, `Register`, then serve — and `prism mcp` reaches it through
`mcpserve.ServeStdio`:

```go
import (
	gosdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/afero"

	"github.com/frankbardon/prism/mcp"
	prismgosdk "github.com/frankbardon/prism/mcp/gosdk"
	"github.com/frankbardon/prism/rpc"
)

func serve(ctx context.Context) error {
	facade := &rpc.PrismServer{Fs: afero.NewOsFs()}
	cfg := mcp.Config{ServerName: "prism", Version: "0.1.0"}

	srv := gosdk.NewServer(&gosdk.Implementation{Name: cfg.ServerName, Version: cfg.Version}, nil)
	if err := prismgosdk.Register(srv, facade, cfg); err != nil {
		return err
	}
	return srv.Run(ctx, &gosdk.StdioTransport{})
}
```

`Register(server, facade, cfg)` never constructs or returns a server — it
grafts onto the one you pass, so Prism's tools sit alongside your own.

## Embedded example corpus

The curated example specs are embedded in a standalone, stdlib-pure package:
`github.com/frankbardon/prism/examples`. Import it to surface examples as
resources on a non-go-sdk server, or anywhere you need spec fixtures without
pulling in the pipeline or an MCP SDK:

- `examples.List() []string` — sorted stems of every valid spec (e.g.
  `bar_basic`, `scales/log`).
- `examples.Get(name string) ([]byte, bool)` — raw spec JSON by stem.
- `examples.Search(query string, limit int) []examples.Result` — substring
  search over stem + title.

The `mcp/gosdk` adapter uses exactly these accessors to publish each spec as
a `prism://examples/<stem>` resource, so you can mirror that wiring on any
transport.

## Geographic marks

If the agent will plot `geoshape` / `geopoint` charts, give the server a
map tier directory: both `prism mcp` and `prism serve` accept
`--geodata-dir <path>` (or the `PRISM_GEODATA` environment variable),
pointing at a folder of `<tier>.geo.json` files. Without it, a geo plot
fails with `PRISM_GEODATA_DIR_UNSET`:

```json
{
  "mcpServers": {
    "prism": {
      "command": "prism",
      "args": ["mcp"],
      "env": {"PRISM_GEODATA": "/path/to/geodata"}
    }
  }
}
```

See [Geographic Marks](../concepts/geo.md) for the tier files and
download link.
