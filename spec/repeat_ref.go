package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// RepeatRef is the in-memory representation of the
// {"repeat": "row"|"column"} substitution allowed in any channel
// that accepts a field name. Per D055 the substitution is a pure
// JSON field-name swap applied at build time by the repeat composite
// walker (plan/build/composite.go). The walker resolves a RepeatRef
// to a bare field-name string by looking up Axis in the parent's
// repeat bindings; unknown axes raise PRISM_SPEC_012.
//
// Example raw JSON:
//
//	{"field": {"repeat": "row"}}
//
// After substitution the channel becomes:
//
//	{"field": "score"}
//
// At the spec layer both forms decode to the same struct; the
// distinction lives on whether Field (the bare string) or FieldRef
// (the {repeat: axis} placeholder) is populated.
type RepeatRef struct {
	// Axis is "row" or "column"; any other value is accepted at
	// decode time but rejected at build-time substitution.
	Axis string `json:"repeat"`
}

// fieldOrRepeat parses a JSON value that may be either a bare string
// (the field name) or a {"repeat": <axis>} object (the substitution
// placeholder). Returns (fieldName, nil) for the string form and
// ("", *RepeatRef) for the object form. Empty input returns
// ("", nil, nil) — callers that require a non-empty field handle the
// missing-binding case downstream.
func fieldOrRepeat(raw json.RawMessage) (string, *RepeatRef, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", nil, nil
	}
	// Strip surrounding whitespace so we can peek the first
	// non-whitespace byte.
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "", nil, nil
	}
	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", nil, fmt.Errorf("spec: channel field: %w", err)
		}
		return s, nil, nil
	case '{':
		var ref RepeatRef
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&ref); err != nil {
			return "", nil, fmt.Errorf("spec: channel field repeat ref: %w", err)
		}
		if ref.Axis == "" {
			return "", nil, fmt.Errorf("spec: channel field repeat ref missing axis")
		}
		return "", &ref, nil
	}
	return "", nil, fmt.Errorf("spec: channel field must be a string or {repeat: <axis>} object, got %s", string(raw))
}
