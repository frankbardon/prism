# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly.

**Do not open a public issue.** Instead, email security concerns to the maintainer or use [GitHub's private vulnerability reporting](https://github.com/frankbardon/prism/security/advisories/new).

Please include:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

You should receive a response within 72 hours. We'll work with you to understand the issue and coordinate a fix before any public disclosure.

## Scope

Security issues in the following areas are in scope:

- **Spec parsing / validation** — `spec/` decoders, `validate/` shape + semantic checks, schema bundle (`schema/`)
- **Resolver** — `.pulse` resolution through `afero.Fs`, archive-shard refs, path handling
- **Plan execution** — `plan/` DAG executor, worker pool, cache, partial-failure policy
- **Compiler** — Pulse request construction in `compile/`, expression compilation
- **Encoders** — `encode/` scene IR, scale/axis/legend builders, palette generation
- **Renderers** — `render/svg`, `render/pdf`, `render/canvas` — SVG/PDF/JSON output
- **Service** — Twirp HTTP server (`rpc/`), MCP stdio server (`mcp/`)
- **CLI** — Flag parsing, input handling, `prism init` template extraction
- **Filesystem boundary** — Path handling through `afero.Fs`, symlink handling

## Known Considerations

- **File paths**: All filesystem access funnels through the `afero.Fs` abstraction. Direct `os.Open`/`os.ReadFile` is forbidden in library code.
- **Join cardinality**: Hash joins are bounded by `PRISM_JOIN_MAX_ROWS` to prevent unbounded memory growth on adversarial inputs.
- **GCS resolution**: GCS resolution is deferred behind `PRISM_RESOLVE_GCS_UNAVAILABLE` until a hardened path lands.
- **Embedded scripts**: SVG output is data-only — no `<script>` elements, no event handlers, no external resource refs. PDF output is vector-only with embedded fonts.
- **Image marks**: Only `data:` URLs are accepted for `image` marks — external URL fetching is not implemented.
