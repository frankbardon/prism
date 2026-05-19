// Package errors defines Prism's typed application errors and the code
// catalog with fixup metadata.
//
// AppError serializes to the Pulse-style JSON envelope (design/09-errors.md):
//
//	{
//	  "code":    "PRISM_SPEC_001",
//	  "message": "Field x not found in dataset y.",
//	  "fixups":  [...],
//	  "context": {...},
//	  "see_also": [...]
//	}
//
// The plain Error() method renders a single-line text form suitable for
// stderr; the CLI prints the multi-line "ERROR / Fixups:" block by
// formatting AppError fields directly.
//
// Pulse's *errors.CodedError values pass through verbatim — interceptors
// route them based on prefix.
package errors

import (
	"encoding/json"
	"fmt"
	"sort"
)

// AppError is the canonical Prism error type.
type AppError struct {
	// Code is a PRISM_* identifier (e.g. "PRISM_SPEC_001").
	Code string

	// Message is the human-readable, already-formatted message.
	Message string

	// Fixups are the ordered, already-formatted fixup suggestions.
	Fixups []string

	// SeeAlso lists related codes or documentation references.
	SeeAlso []string

	// Context carries the variables that were substituted into the
	// message template (e.g. {"Field": "xfield", "Dataset": "cohort"}).
	Context map[string]any

	// Inner is a wrapped underlying error, if any.
	Inner error
}

// New constructs an AppError with the given code, message, and context.
// Fixups and SeeAlso are looked up from the code catalog when present.
func New(code, message string, ctx map[string]any) *AppError {
	e := &AppError{Code: code, Message: message, Context: ctx}
	if meta, ok := Codes[code]; ok {
		e.Fixups = formatFixups(meta.Fixups, ctx)
		e.SeeAlso = append([]string(nil), meta.SeeAlso...)
	}
	return e
}

// Wrap is New plus an inner error.
func Wrap(code, message string, ctx map[string]any, inner error) *AppError {
	e := New(code, message, ctx)
	e.Inner = inner
	return e
}

// Error renders a single-line description suitable for stderr.
func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Inner != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Inner)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the inner error so errors.Is / errors.As work.
func (e *AppError) Unwrap() error { return e.Inner }

// envelope is the JSON shape from design/09-errors.md ("errors" entry).
type envelope struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Context map[string]any `json:"context,omitempty"`
	Fixups  []string       `json:"fixups,omitempty"`
	SeeAlso []string       `json:"see_also,omitempty"`
}

// MarshalJSON implements json.Marshaler emitting the envelope shape.
func (e *AppError) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte("null"), nil
	}
	return json.Marshal(envelope{
		Code:    e.Code,
		Message: e.Message,
		Context: e.Context,
		Fixups:  e.Fixups,
		SeeAlso: e.SeeAlso,
	})
}

// ContextKeys returns context keys in deterministic order; useful for
// stable text rendering and tests.
func (e *AppError) ContextKeys() []string {
	out := make([]string, 0, len(e.Context))
	for k := range e.Context {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
