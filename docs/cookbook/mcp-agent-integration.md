# Cookbook: MCP agent integration

Expose Prism to an LLM agent so it can plot, validate, and describe
specs as tool calls.

## Start the MCP server

```
prism mcp
```

Reads JSON-RPC frames on stdin, writes responses on stdout. Standard
MCP stdio transport.

## Tools exposed

| Tool | Args | Returns |
|---|---|---|
| `prism_plot` | `{spec, format?}` | `{bytes (base64), mime, caption}` |
| `prism_validate` | `{spec}` | `{ok, errors}` |
| `prism_describe` | `{spec}` | `{summary}` |
| `prism_examples_search` | `{query}` | `{examples: [{name, summary, spec}]}` |

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
