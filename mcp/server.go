//go:build !js

// Package mcp is the SDK-agnostic core of Prism's Model Context Protocol
// surface. It exposes the four Prism tools as typed handlers and as a
// type-erased descriptor catalog (Tools(cfg)), plus the pure helpers behind
// them — and imports NO MCP SDK. Mounting the catalog onto a concrete server
// is the job of an adapter (see the mcp/gosdk package, which grafts these
// descriptors onto a modelcontextprotocol/go-sdk server).
//
// The four tools are:
//
//   - prism_plot(spec, format?)         → bytes + mime + caption
//   - prism_validate(spec)              → ok + structured errors
//   - prism_describe(spec)              → natural-language summary
//   - prism_examples_search(query)      → list of fixture specs
//
// Each has a typed handler (PlotTool, ValidateTool, DescribeTool,
// ExamplesSearchTool) callable directly against an *rpc.PrismServer, and a
// matching descriptor in Tools(cfg) for transport-neutral mounting.
package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/afero"

	"github.com/frankbardon/prism/examples"
	"github.com/frankbardon/prism/rpc"
	prismspec "github.com/frankbardon/prism/spec"
)

// PlotInput is the typed input for the prism_plot tool.
type PlotInput struct {
	Spec   string `json:"spec"`
	Format string `json:"format,omitempty"`
}

// PlotOutput is the structured payload returned by prism_plot.
type PlotOutput struct {
	Bytes    string   `json:"bytes"` // base64-encoded
	Mime     string   `json:"mime"`
	Caption  string   `json:"caption"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateInput is the typed input for the prism_validate tool.
type ValidateInput struct {
	Spec string `json:"spec"`
}

// ValidateError is one validation diagnostic in a ValidateOutput. Field order
// mirrors the historical map-marshalled shape (code, fixups, message).
type ValidateError struct {
	Code    string   `json:"code"`
	Fixups  []string `json:"fixups"`
	Message string   `json:"message"`
}

// ValidateOutput is the structured payload returned by prism_validate. Field
// order mirrors the historical map-marshalled shape (errors, ok).
type ValidateOutput struct {
	Errors []ValidateError `json:"errors"`
	Ok     bool            `json:"ok"`
}

// DescribeInput is the typed input for the prism_describe tool.
type DescribeInput struct {
	Spec string `json:"spec"`
}

// DescribeOutput is the structured payload returned by prism_describe.
type DescribeOutput struct {
	Summary string `json:"summary"`
}

// ExampleResult is one entry returned by prism_examples_search.
type ExampleResult struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Spec    string `json:"spec"`
}

// ExamplesSearchInput is the typed input for the prism_examples_search tool.
// Query is the wire-facing field; Root + FS carry the server's examples-source
// configuration (empty Root → embedded corpus, non-empty → on-disk walk) and
// are not part of the tool's JSON schema.
type ExamplesSearchInput struct {
	Query string   `json:"query"`
	Root  string   `json:"-"`
	FS    afero.Fs `json:"-"`
}

// ExamplesSearchOutput is the structured payload returned by
// prism_examples_search.
type ExamplesSearchOutput struct {
	Examples []ExampleResult `json:"examples"`
}

// missingArg builds the argument-error returned when a required tool argument
// is absent. The SDK wiring surfaces it as a tool-result error so the agent
// can self-correct.
func missingArg(name string) error {
	return errors.New("missing required argument: " + name)
}

// PlotTool compiles a Prism spec and renders it to image bytes via the rpc
// facade. It returns the facade's coded errors verbatim.
func PlotTool(ctx context.Context, f *rpc.PrismServer, in PlotInput) (PlotOutput, error) {
	if in.Spec == "" {
		return PlotOutput{}, missingArg("spec")
	}
	format := in.Format
	if format == "" {
		format = "svg"
	}

	// The Twirp Plot handler enforces the format switch + runs the full
	// pipeline; we reuse it to keep one source of truth for "what 'svg' means".
	resp, err := f.Plot(ctx, &rpc.PlotRequest{Spec: in.Spec, Format: format})
	if err != nil {
		return PlotOutput{}, err
	}
	return PlotOutput{
		Bytes:    base64.StdEncoding.EncodeToString(resp.Bytes),
		Mime:     resp.Mime,
		Caption:  summariseSpec(in.Spec), // caption from the parsed spec
		Warnings: append([]string(nil), resp.Warnings...),
	}, nil
}

// ValidateTool validates a Prism spec via the rpc facade, returning the
// facade's coded errors verbatim.
func ValidateTool(ctx context.Context, f *rpc.PrismServer, in ValidateInput) (ValidateOutput, error) {
	if in.Spec == "" {
		return ValidateOutput{}, missingArg("spec")
	}
	resp, err := f.Validate(ctx, &rpc.ValidateRequest{Spec: in.Spec})
	if err != nil {
		return ValidateOutput{}, err
	}
	errs := make([]ValidateError, 0, len(resp.Errors))
	for _, e := range resp.Errors {
		errs = append(errs, ValidateError{
			Code:    e.Code,
			Fixups:  e.Fixups,
			Message: e.Message,
		})
	}
	return ValidateOutput{Errors: errs, Ok: resp.Ok}, nil
}

// DescribeTool returns a natural-language summary of a Prism spec. The facade
// argument is accepted for signature uniformity and is unused.
func DescribeTool(_ context.Context, _ *rpc.PrismServer, in DescribeInput) (DescribeOutput, error) {
	if in.Spec == "" {
		return DescribeOutput{}, missingArg("spec")
	}
	return DescribeOutput{Summary: summariseSpec(in.Spec)}, nil
}

// ExamplesSearchTool searches the example spec library. An empty in.Root serves
// the embedded corpus; a non-empty Root opts into an on-disk afero walk of that
// directory. The facade argument is accepted for signature uniformity.
func ExamplesSearchTool(_ context.Context, _ *rpc.PrismServer, in ExamplesSearchInput) (ExamplesSearchOutput, error) {
	if in.Query == "" {
		return ExamplesSearchOutput{}, missingArg("query")
	}
	var hits []ExampleResult
	if in.Root == "" {
		hits = searchEmbedded(in.Query, 5)
	} else {
		hits = searchExamples(in.FS, in.Root, in.Query, 5)
	}
	return ExamplesSearchOutput{Examples: hits}, nil
}

// searchEmbedded serves prism_examples_search from the embedded examples
// corpus. It mirrors examples.Search but layers the richer summariseSpec
// fallback for specs that carry no title: examples.Search falls back to the
// bare stem to stay stdlib-pure, whereas the MCP tool historically produced a
// spec-aware summary, so we preserve that user-visible output here.
func searchEmbedded(query string, limit int) []ExampleResult {
	results := examples.Search(query, limit)
	out := make([]ExampleResult, 0, len(results))
	for _, r := range results {
		summary := r.Summary
		if extractTitle([]byte(r.Spec)) == "" {
			if richer := summariseSpec(r.Spec); richer != "" {
				summary = richer
			}
		}
		out = append(out, ExampleResult{
			Name:    r.Name,
			Summary: summary,
			Spec:    r.Spec,
		})
	}
	return out
}

// searchExamples walks the examples root and returns up to limit
// results whose filename stem (or title) contains query case-
// insensitively. Each result carries the raw spec JSON for direct
// invocation.
func searchExamples(fsys afero.Fs, root, query string, limit int) []ExampleResult {
	if fsys == nil {
		fsys = afero.NewOsFs()
	}
	q := strings.ToLower(query)
	var out []ExampleResult
	_ = afero.Walk(fsys, root, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".json") {
			return nil
		}
		// Skip the "invalid/" subdirectory — those are designed to fail.
		if strings.Contains(path, string(filepath.Separator)+"invalid"+string(filepath.Separator)) {
			return nil
		}
		base := strings.TrimSuffix(filepath.Base(path), ".json")
		body, err := afero.ReadFile(fsys, path)
		if err != nil {
			return nil
		}
		title := extractTitle(body)
		hay := strings.ToLower(base + " " + title)
		if !strings.Contains(hay, q) {
			return nil
		}
		summary := title
		if summary == "" {
			summary = summariseSpec(string(body))
		}
		out = append(out, ExampleResult{
			Name:    base,
			Summary: summary,
			Spec:    string(body),
		})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// extractTitle picks the spec.title field without forcing a full
// decode (the spec may not pass strict decoding even for examples).
func extractTitle(body []byte) string {
	var doc struct {
		Title string `json:"title"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return ""
	}
	return doc.Title
}

// summariseSpec returns a one-line English description of the
// rendered chart. Deterministic and side-effect-free; safe for the
// describe tool. Returns "" when the spec fails decoding.
func summariseSpec(specJSON string) string {
	s, err := prismspec.DecodeBytes([]byte(specJSON))
	if err != nil {
		return ""
	}
	mark := strings.ToLower(s.Mark.TypeName())
	if mark == "" {
		mark = "chart"
	}
	var parts []string
	parts = append(parts, mark+" chart")
	if s.Encoding != nil {
		var enc []string
		if s.Encoding.X != nil && s.Encoding.X.Field != "" {
			enc = append(enc, "x="+s.Encoding.X.Field)
		}
		if s.Encoding.Y != nil && s.Encoding.Y.Field != "" {
			enc = append(enc, "y="+s.Encoding.Y.Field)
		}
		if s.Encoding.Color != nil && s.Encoding.Color.Field != "" {
			enc = append(enc, "color="+s.Encoding.Color.Field)
		}
		if len(enc) > 0 {
			parts = append(parts, "with "+strings.Join(enc, ", "))
		}
	}
	if len(s.Transform) > 0 {
		parts = append(parts, fmt.Sprintf("(%d transform%s)", len(s.Transform), pluralS(len(s.Transform))))
	}
	if title := titleString(s.Title); title != "" {
		parts = append([]string{title + ":"}, parts...)
	}
	return strings.Join(parts, " ")
}

// titleString extracts the text portion of a spec title regardless of
// whether it was provided as a bare string or rich object.
func titleString(t *prismspec.TextOrTextObj) string {
	if t == nil {
		return ""
	}
	if t.Text != nil {
		return *t.Text
	}
	if t.Obj != nil {
		return t.Obj.Text
	}
	return ""
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
