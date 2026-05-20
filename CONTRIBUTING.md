# Contributing to Prism

Thanks for your interest in contributing to Prism! This guide covers the basics.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/<you>/prism.git`
3. Create a branch: `git checkout -b my-feature`
4. Make your changes
5. Run checks: `make test && make test-race && make lint`
6. Commit and push
7. Open a pull request

## Development Setup

- Go 1.26+
- (Optional) `mdbook` for docs preview — `cargo install mdbook`
- (Optional) `protoc`, `protoc-gen-go`, `protoc-gen-twirp` for regenerating Twirp stubs

```bash
make build       # Build CLI binary to bin/prism
make test        # Run tests
make test-race   # Run tests with the race detector
make lint        # Run staticcheck (includes vet)
make cover       # Run tests with coverage
make docs-serve  # Preview the mdBook site locally
```

## Code Conventions

- **Library-first.** All compilation, planning, encoding, and rendering logic lives in library packages. The CLI in `cmd/prism/` is a thin adapter.
- **No business logic in `cmd/prism/`.** Parse flags, call library, format output.
- **snake_case spec vocabulary.** Multi-word Prism spec fields are snake_case (e.g., `stroke_width`, `corner_radius`). Single-word Vega-Lite vocabulary (`mark`, `encoding`, `transform`, `layer`, `facet`) stays as-is.
- **Pulse expression syntax** in `filter` predicates and `calculate` transforms. No `datum.` prefix. No JS function calls.
- **Error codes** use `PRISM_*` namespace. Every code carries fixup metadata accessible via `prism errors lookup CODE`.
- **All file I/O via `afero.Fs`.** Never `os.Open` directly in library code.
- **No `fmt.Sprintf` for JSON.** Use `encoding/json` envelopes.

See [CLAUDE.md](CLAUDE.md) for the full set of conventions and contracts.

## The Update Demand

Any change to code, configuration, spec vocabulary, or public surface MUST update the corresponding doc / skill file(s) and `CLAUDE.md` in the same PR. The Update Demand table in `CLAUDE.md` lists every trigger and its enforcing gate. Do not defer doc updates to a follow-up PR.

## Pull Request Guidelines

- Keep PRs focused — one feature or fix per PR
- Include tests for new functionality (TDD: tests first)
- Update docs and `CLAUDE.md` per the Update Demand
- Run `make test && make lint` before submitting
- Fill out the PR template

## Reporting Bugs

Use the [bug report template](https://github.com/frankbardon/prism/issues/new?template=bug_report.yml). Include the minimal Prism spec, Go version, and OS.

## Suggesting Features

Use the [feature request template](https://github.com/frankbardon/prism/issues/new?template=feature_request.yml). Describe the problem you're solving, not just the solution you want.
