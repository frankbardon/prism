//go:build !js

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/spf13/afero"

	"github.com/frankbardon/prism/rpc"
)

// Default server identity used when a Config leaves them unset. These are
// constants, not mutable globals — callers override via Config/Options.
const (
	DefaultServerName = "prism"
	DefaultVersion    = "0.1.0"
)

// Config carries the server-construction parameters for the tool catalog.
// It threads the MCP server identity (ServerName/Version) and the
// examples_search data-source override so neither lives in a process global.
// An empty ExamplesRoot serves the embedded corpus; a non-empty value opts
// into an on-disk afero walk of that directory (ExamplesFS defaults to the OS
// filesystem when unset).
type Config struct {
	ServerName   string
	Version      string
	ExamplesRoot string
	ExamplesFS   afero.Fs
}

// ToolDescriptor is a transport- and SDK-agnostic description of one Prism
// MCP tool. Invoke is type-erased: it unmarshals the raw JSON arguments into
// the tool's typed input, calls the corresponding typed handler, and returns
// the typed output as any (with the handler's coded error verbatim). Any
// consumer can mount these descriptors without depending on an MCP SDK or
// Go generics.
type ToolDescriptor struct {
	Name         string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
	Invoke       func(ctx context.Context, f *rpc.PrismServer, raw json.RawMessage) (any, error)
}

// Reflected schemas for each tool's typed I/O. Computed once at package init
// from the struct tags via jsonschema-go — never hand-written. Fields tagged
// json:"-" (e.g. ExamplesSearchInput.Root/FS) are omitted by the reflector, so
// the examples_search input schema exposes only "query".
var (
	plotInputSchema  = reflectSchema[PlotInput]()
	plotOutputSchema = reflectSchema[PlotOutput]()

	validateInputSchema  = reflectSchema[ValidateInput]()
	validateOutputSchema = reflectSchema[ValidateOutput]()

	describeInputSchema  = reflectSchema[DescribeInput]()
	describeOutputSchema = reflectSchema[DescribeOutput]()

	examplesSearchInputSchema  = reflectSchema[ExamplesSearchInput]()
	examplesSearchOutputSchema = reflectSchema[ExamplesSearchOutput]()
)

// reflectSchema infers a JSON Schema for T from its struct tags and marshals
// it to a stable json.RawMessage. It panics on failure: the catalog's I/O
// types are flat and SDK-reflectable, so a failure here is a programming
// error caught at package init rather than at request time.
func reflectSchema[T any]() json.RawMessage {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("mcp: reflect schema for %T: %v", *new(T), err))
	}
	raw, err := json.Marshal(s)
	if err != nil {
		panic(fmt.Sprintf("mcp: marshal schema for %T: %v", *new(T), err))
	}
	return raw
}

// Tools returns the full Prism MCP tool catalog as type-erased descriptors.
// The descriptors carry reflected input/output schemas and Invoke closures
// that bridge to the E3-S1 typed handlers. The examples_search override
// (cfg.ExamplesRoot/ExamplesFS) is baked into that tool's closure so the
// data source is a parameter, not a global. Names and descriptions match the
// registered tool surface verbatim.
func Tools(cfg Config) []ToolDescriptor {
	exRoot := cfg.ExamplesRoot
	exFS := cfg.ExamplesFS

	return []ToolDescriptor{
		{
			Name:         "prism_plot",
			Description:  "Compile a Prism spec and render to image bytes. SVG (default) and PDF are supported; PNG returns PRISM_RENDER_FORMAT_UNAVAILABLE (V2).",
			InputSchema:  plotInputSchema,
			OutputSchema: plotOutputSchema,
			Invoke: func(ctx context.Context, f *rpc.PrismServer, raw json.RawMessage) (any, error) {
				var in PlotInput
				if err := decodeArgs(raw, &in); err != nil {
					return nil, err
				}
				return PlotTool(ctx, f, in)
			},
		},
		{
			Name:         "prism_validate",
			Description:  "Validate a Prism spec against the embedded JSON Schema + semantic rules.",
			InputSchema:  validateInputSchema,
			OutputSchema: validateOutputSchema,
			Invoke: func(ctx context.Context, f *rpc.PrismServer, raw json.RawMessage) (any, error) {
				var in ValidateInput
				if err := decodeArgs(raw, &in); err != nil {
					return nil, err
				}
				return ValidateTool(ctx, f, in)
			},
		},
		{
			Name:         "prism_describe",
			Description:  "Return a natural-language summary of what a Prism spec renders.",
			InputSchema:  describeInputSchema,
			OutputSchema: describeOutputSchema,
			Invoke: func(ctx context.Context, f *rpc.PrismServer, raw json.RawMessage) (any, error) {
				var in DescribeInput
				if err := decodeArgs(raw, &in); err != nil {
					return nil, err
				}
				return DescribeTool(ctx, f, in)
			},
		},
		{
			Name:         "prism_examples_search",
			Description:  "Search the curated example spec library by substring match on name + title. Returns up to 5 results.",
			InputSchema:  examplesSearchInputSchema,
			OutputSchema: examplesSearchOutputSchema,
			Invoke: func(ctx context.Context, f *rpc.PrismServer, raw json.RawMessage) (any, error) {
				var in ExamplesSearchInput
				if err := decodeArgs(raw, &in); err != nil {
					return nil, err
				}
				// The data-source override is server config, never wire input.
				in.Root = exRoot
				in.FS = exFS
				return ExamplesSearchTool(ctx, f, in)
			},
		},
	}
}

// decodeArgs unmarshals raw tool-call arguments into dst, treating an empty
// payload as the zero value (so a missing-argument check in the typed handler
// fires rather than a JSON syntax error).
func decodeArgs(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}
